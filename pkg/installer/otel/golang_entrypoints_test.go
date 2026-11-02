package otel

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func serviceNames(services []GoService) []string {
	names := make([]string, 0, len(services))
	for _, s := range services {
		names = append(names, s.Name)
	}
	return names
}

const mainSrc = "package main\n\nfunc main() {}\n"

func TestDiscoverGoServices(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"root main", map[string]string{"go.mod": "module x\n", "main.go": mainSrc}, []string{"shop"}},
		{"cmd dirs", map[string]string{
			"go.mod": "module x\n", "cmd/api/main.go": mainSrc, "cmd/worker/main.go": mainSrc,
			"internal/lib/lib.go": "package lib\n",
		}, []string{"api", "worker"}},
		{"depth limit", map[string]string{
			"go.mod": "module x\n", "a/b/c/d/e/main.go": mainSrc, "a/b/c/d/e/f/main.go": mainSrc,
		}, []string{"e"}},
		{"nested module", map[string]string{
			"go.mod": "module x\n", "cmd/api/main.go": mainSrc, "tools/go.mod": "module y\n", "tools/gen/main.go": mainSrc,
		}, []string{"api"}},
		{"library only", map[string]string{"go.mod": "module x\n", "lib.go": "package lib\n"}, []string{}},
		{"testdata and tests ignored", map[string]string{
			"go.mod": "module x\n", "testdata/main.go": mainSrc, "main_test.go": mainSrc, "vendor/v/main.go": mainSrc,
		}, []string{}},
		{"ignore build tag", map[string]string{
			"go.mod": "module x\n", "gen/gen.go": "//go:build ignore\n\npackage main\n",
		}, []string{}},
		{"name collision", map[string]string{
			"go.mod": "module x\n", "services/a/cmd/server/main.go": mainSrc, "services/b/cmd/server/main.go": mainSrc,
		}, []string{"services-a-cmd-server", "services-b-cmd-server"}},
		{"root and cmd same name", map[string]string{
			"go.mod": "module x\n", "main.go": mainSrc, "cmd/shop/main.go": mainSrc,
		}, []string{"shop", "cmd-shop"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "shop")
			writeTree(t, root, tt.files)
			got := serviceNames(discoverGoServices(root))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("services = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoverGoServices_RelDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shop")
	writeTree(t, root, map[string]string{"go.mod": "module x\n", "cmd/api/main.go": mainSrc})
	got := discoverGoServices(root)
	if len(got) != 1 || got[0].RelDir != "cmd/api" || got[0].Dir != filepath.Join(root, "cmd", "api") {
		t.Fatalf("unexpected services: %#v", got)
	}
}
