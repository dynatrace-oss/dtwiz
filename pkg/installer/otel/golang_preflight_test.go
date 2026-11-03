package otel

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoPreflight_FakeRunner(t *testing.T) {
	orig := goBuildRunner
	t.Cleanup(func() { goBuildRunner = orig })

	var gotDir, gotPkg string
	goBuildRunner = func(_ context.Context, dir string, _ []string, _ string, args ...string) (string, error) {
		gotDir, gotPkg = dir, args[len(args)-1]
		return "", nil
	}
	if err := goPreflight(context.Background(), GoService{RelDir: "cmd/api"}, "/m"); err != nil || gotDir != "/m" || gotPkg != "./cmd/api" {
		t.Fatalf("err=%v dir=%q pkg=%q", err, gotDir, gotPkg)
	}
	if err := goPreflight(context.Background(), GoService{RelDir: "."}, "/m"); err != nil || gotPkg != "." {
		t.Fatalf("err=%v pkg=%q", err, gotPkg)
	}

	goBuildRunner = func(context.Context, string, []string, string, ...string) (string, error) {
		return "line1\nline2\n", errors.New("exit status 1")
	}
	if err := goPreflight(context.Background(), GoService{RelDir: "."}, "/m"); err == nil || !strings.Contains(err.Error(), "line1; line2") {
		t.Fatalf("err = %v", err)
	}
}

// A module that imports a sibling only through go.work resolves in workspace
// mode but not on its own, which is what the pre-flight must catch. It needs a
// Go toolchain but no network.
func TestGoPreflight_RealGo_SiblingOnlyThroughWorkspace(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	root := filepath.Join(t.TempDir(), "ws")
	writeTree(t, root, map[string]string{
		"go.work":   "go 1.22\n\nuse (\n\t./a\n\t./b\n\t./c\n)\n",
		"a/go.mod":  "module example.com/ws/a\n\ngo 1.22\n\nrequire example.com/ws/b v0.0.0\n",
		"a/main.go": "package main\n\nimport \"example.com/ws/b\"\n\nfunc main() { b.Hello() }\n",
		"b/go.mod":  "module example.com/ws/b\n\ngo 1.22\n",
		"b/b.go":    "package b\n\nfunc Hello() {}\n",
		"c/go.mod":  "module example.com/ws/c\n\ngo 1.22\n",
		"c/main.go": mainSrc,
	})
	ctx := context.Background()
	if err := goPreflight(ctx, GoService{RelDir: "."}, filepath.Join(root, "c")); err != nil {
		t.Fatalf("independent module must pass: %v", err)
	}
	if err := goPreflight(ctx, GoService{RelDir: "."}, filepath.Join(root, "a")); err == nil {
		t.Fatal("module depending on a workspace sibling must fail the pre-flight")
	}
}
