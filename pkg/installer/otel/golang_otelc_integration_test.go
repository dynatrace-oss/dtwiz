//go:build otelc_integration

package otel

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestOtelcIntegration_BuildTinyModule installs the current otelc@latest into an
// isolated HOME, builds a tiny module with it and checks that the binary exists
// and go.mod/go.sum are byte-identical afterwards. It needs network access and
// runs in the scheduled "otelc smoke" workflow:
//
//	go test -tags otelc_integration -run OtelcIntegration ./pkg/installer/otel/
func TestOtelcIntegration_BuildTinyModule(t *testing.T) {
	// Keep the module and build caches of the real environment so the test does
	// not download everything twice, but install otelc into a throwaway HOME.
	modCache := goEnv(t, "GOMODCACHE")
	buildCache := goEnv(t, "GOCACHE")
	setHome(t)
	t.Setenv("GOMODCACHE", modCache)
	t.Setenv("GOCACHE", buildCache)

	tool, err := ensureOtelc()
	if err != nil {
		t.Fatalf("installing otelc@latest failed: %v", err)
	}
	t.Logf("otelc %s at %s", tool.Version, tool.Path)

	module := t.TempDir()
	writeTree(t, module, map[string]string{
		"go.mod":  "module example.com/tiny\n\ngo 1.22\n",
		"main.go": "package main\n\nimport \"net/http\"\n\nfunc main() {\n\t_ = http.ListenAndServe(\"127.0.0.1:0\", nil)\n}\n",
	})
	wantMod := readFile(t, filepath.Join(module, "go.mod"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	bin := filepath.Join(goOtelDir(module), "bin", goBinaryName("tiny"))
	svc := GoService{Name: "tiny", Dir: module, RelDir: "."}
	if err := buildWithOtelc(ctx, tool, module, svc, bin); err != nil {
		t.Fatalf("otelc build failed: %v", err)
	}

	if info, err := os.Stat(bin); err != nil || info.IsDir() {
		t.Fatalf("instrumented binary missing: %v", err)
	}
	if got := readFile(t, filepath.Join(module, "go.mod")); got != wantMod {
		t.Fatalf("go.mod changed:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(module, "go.sum")); !os.IsNotExist(err) {
		t.Fatal("go.sum must not exist afterwards when it did not exist before")
	}
	if _, err := os.Stat(filepath.Join(goOtelDir(module), goModguardDirName)); !os.IsNotExist(err) {
		t.Fatal("modguard backup should be cleaned up after a successful build")
	}
}

func goEnv(t *testing.T, key string) string {
	t.Helper()
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		t.Fatalf("go env %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}
