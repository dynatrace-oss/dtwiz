package otel

import (
	"os"
	"path/filepath"
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
