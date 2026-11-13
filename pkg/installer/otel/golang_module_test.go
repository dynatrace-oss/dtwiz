package otel

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeGoMod(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestModuleUsesOtelSDK(t *testing.T) {
	tests := []struct {
		name string
		mod  string
		want bool
	}{
		{"direct sdk", "module x\n\nrequire (\n\tgo.opentelemetry.io/otel/sdk v1.30.0\n)\n", true},
		{"direct exporter", "module x\n\nrequire go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.0.0\n", true},
		{"indirect sdk", "module x\n\nrequire (\n\tgo.opentelemetry.io/otel/sdk v1.30.0 // indirect\n)\n", false},
		{"api only", "module x\n\nrequire go.opentelemetry.io/otel v1.30.0\n", false},
		{"none", "module x\n\ngo 1.22\n", false},
		{"require without space", "module x\n\nrequire(\n\tgo.opentelemetry.io/otel/sdk v1.30.0\n)\n", true},
		{"comment after close paren", "module x\n\nrequire (\n\tgo.opentelemetry.io/otel/sdk v1.30.0\n) // deps\n", true},
		{"indirect in single-line require", "module x\n\nrequire go.opentelemetry.io/otel/sdk v1.30.0 // indirect\n", false},
		{"sdk metric", "module x\n\nrequire (\n\tgo.opentelemetry.io/otel/sdk/metric v1.30.0\n)\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := moduleUsesOtelSDK(writeGoMod(t, tt.mod)); got != tt.want {
				t.Fatalf("moduleUsesOtelSDK = %v, want %v", got, tt.want)
			}
		})
	}
	if moduleUsesOtelSDK(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("missing go.mod must not be reported as using the SDK")
	}
}

func TestFindGoWork(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(mod, 0755); err != nil {
		t.Fatal(err)
	}

	if got := findGoWork(mod); got != "" {
		// A go.work above the temp dir would make this test environment-dependent.
		t.Skipf("unexpected go.work above temp dir: %s", got)
	}

	work := filepath.Join(root, "go.work")
	if err := os.WriteFile(work, []byte("go 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := findGoWork(mod); got != work {
		t.Fatalf("parent go.work: got %q, want %q", got, work)
	}
	if got := findGoWork(root); got != work {
		t.Fatalf("root go.work: got %q, want %q", got, work)
	}
}

func TestGoWorkspaceModules(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	writeTree(t, root, map[string]string{
		"a/go.mod": "module a\n", "b/go.mod": "module b\n", "c d/go.mod": "module c\n",
		"nogomod/x.go": "package x\n",
	})
	tests := []struct {
		name string
		work string
		want []string
	}{
		{"none", "", nil},
		{"block with comments", "go 1.25\n\nuse (\n\t. // root\n\t./a\n\t// ./b\n\t\"./c d\"\n)\n", []string{"a", "c d"}},
		{"single line", "go 1.25\nuse ./b\n", []string{"b"}},
		{"outside project and dot-dot", "use (\n\t../other\n\t..\n\t./a\n)\n", []string{"a"}},
		{"duplicates and missing go.mod", "use (\n\t./a\n\ta\n\t./nogomod\n\t./missing\n)\n", []string{"a"}},
		{"use without space and trailing comment", "use(\n\t./a\n) // modules\n", []string{"a"}},
		{"several use lines", "use ./a\nuse ./b\n", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Remove(filepath.Join(root, "go.work"))
			if tt.work != "" {
				writeTree(t, root, map[string]string{"go.work": tt.work})
			}
			var want []string
			for _, w := range tt.want {
				want = append(want, filepath.Join(root, w))
			}
			if got := goWorkspaceModules(root); !reflect.DeepEqual(got, want) {
				t.Fatalf("modules = %v, want %v", got, want)
			}
		})
	}
}
