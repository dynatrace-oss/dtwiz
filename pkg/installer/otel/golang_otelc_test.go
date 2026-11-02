package otel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

type runnerCall struct {
	name string
	args []string
	env  []string
}

func stubGoRunner(t *testing.T, fn func(call runnerCall) (string, error)) *[]runnerCall {
	t.Helper()
	var calls []runnerCall
	orig := goRunner
	goRunner = func(name string, args []string, env []string) (string, error) {
		c := runnerCall{name, args, env}
		calls = append(calls, c)
		return fn(c)
	}
	t.Cleanup(func() { goRunner = orig })
	return &calls
}

func stubLookPath(t *testing.T, found map[string]string) {
	t.Helper()
	orig := installer.ExecLookPath
	installer.ExecLookPath = func(file string) (string, error) {
		if p, ok := found[file]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { installer.ExecLookPath = orig })
}

func setHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

const versionOutput = "/x/otelc: go1.26.2\n\tpath\tgo.opentelemetry.io/otelc/tool/cmd/otelc\n\tmod\tgo.opentelemetry.io/otelc/tool\tv1.2.0\th1:abc\n"

func TestEnsureOtelc_InstallsLatestIsolated(t *testing.T) {
	home := setHome(t)
	stubLookPath(t, nil)
	calls := stubGoRunner(t, func(c runnerCall) (string, error) {
		if c.args[0] == "version" {
			return versionOutput, nil
		}
		return "", nil
	})

	tool, err := ensureOtelc()
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(home, ".opentelemetry", "go", "bin")
	if tool.Path != filepath.Join(binDir, otelcBinaryName()) || !tool.Isolated || tool.Version != "v1.2.0" {
		t.Fatalf("unexpected tool: %#v", tool)
	}

	install := (*calls)[0]
	if install.name != "go" || !slices.Equal(install.args, []string{"install", "go.opentelemetry.io/otelc/tool/cmd/otelc@latest"}) {
		t.Fatalf("install command = %s %v", install.name, install.args)
	}
	if !slices.Contains(install.env, "GOBIN="+binDir) || !slices.Contains(install.env, "GOTOOLCHAIN=local") {
		t.Fatalf("install env must set GOBIN and GOTOOLCHAIN=local, got GOBIN/GOTOOLCHAIN entries: %v", filterEnv(install.env))
	}
}

func filterEnv(env []string) []string {
	var out []string
	for _, e := range env {
		if strings.HasPrefix(e, "GOBIN=") || strings.HasPrefix(e, "GOTOOLCHAIN=") {
			out = append(out, e)
		}
	}
	return out
}

func TestEnsureOtelc_PathWins(t *testing.T) {
	setHome(t)
	stubLookPath(t, map[string]string{"otelc": "/usr/local/bin/otelc"})
	calls := stubGoRunner(t, func(c runnerCall) (string, error) { return versionOutput, nil })

	tool, err := ensureOtelc()
	if err != nil {
		t.Fatal(err)
	}
	if tool.Path != "/usr/local/bin/otelc" || tool.Isolated {
		t.Fatalf("PATH otelc must be reused and not owned by dtwiz: %#v", tool)
	}
	for _, c := range *calls {
		if c.args[0] == "install" {
			t.Fatal("must not install when otelc is on PATH")
		}
	}
}

func TestEnsureOtelc_ReusesIsolatedInstall(t *testing.T) {
	home := setHome(t)
	stubLookPath(t, nil)
	binDir := filepath.Join(home, ".opentelemetry", "go", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, otelcBinaryName()), []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	calls := stubGoRunner(t, func(c runnerCall) (string, error) { return versionOutput, nil })

	tool, err := ensureOtelc()
	if err != nil || !tool.Isolated {
		t.Fatalf("tool=%#v err=%v", tool, err)
	}
	for _, c := range *calls {
		if c.args[0] == "install" {
			t.Fatal("must not reinstall an existing isolated otelc")
		}
	}
}

func TestEnsureOtelc_TooOldGo(t *testing.T) {
	setHome(t)
	stubLookPath(t, nil)
	stubGoRunner(t, func(c runnerCall) (string, error) {
		return "", errors.New("exit status 1: go: go.opentelemetry.io/otelc/tool/cmd/otelc@latest: go.opentelemetry.io/otelc/tool@v1.2.0 requires go >= 1.26.0 (running go 1.24.1; GOTOOLCHAIN=local)")
	})

	_, err := ensureOtelc()
	var tooOld *GoVersionTooOldError
	if !errors.As(err, &tooOld) {
		t.Fatalf("expected GoVersionTooOldError, got %v", err)
	}
	if tooOld.Required != "1.26.0" || !strings.Contains(err.Error(), "upgrade Go to 1.26.0") {
		t.Fatalf("unexpected error: %v (required=%q)", err, tooOld.Required)
	}
}

func TestEnsureOtelc_UnrelatedInstallFailure(t *testing.T) {
	setHome(t)
	stubLookPath(t, nil)
	stubGoRunner(t, func(c runnerCall) (string, error) {
		return "", errors.New("dial tcp: lookup proxy.golang.org: no such host")
	})

	_, err := ensureOtelc()
	var tooOld *GoVersionTooOldError
	if err == nil || errors.As(err, &tooOld) || strings.Contains(err.Error(), "upgrade Go") {
		t.Fatalf("unrelated failure must not produce the upgrade message: %v", err)
	}
	if !strings.Contains(err.Error(), runtime.GOOS+"/"+runtime.GOARCH) {
		t.Fatalf("error should name the platform: %v", err)
	}
}

func TestParseOtelcVersion(t *testing.T) {
	if got := parseOtelcVersion(versionOutput); got != "v1.2.0" {
		t.Fatalf("got %q", got)
	}
	if got := parseOtelcVersion("garbage"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func stubBuildRunner(t *testing.T, fn func(dir string, env []string, name string, args []string) (string, error)) {
	t.Helper()
	orig := goBuildRunner
	goBuildRunner = func(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
		return fn(dir, env, name, args)
	}
	t.Cleanup(func() { goBuildRunner = orig })
}

func TestBuildWithOtelc_CommandLineAndEnv(t *testing.T) {
	modulePath := setupGuardModule(t, "module a\n", "sum\n")
	svc := GoService{Name: "api", RelDir: "cmd/api", Dir: filepath.Join(modulePath, "cmd", "api")}
	binPath := filepath.Join(goOtelDir(modulePath), "bin", "api")

	var gotDir, gotName string
	var gotArgs, gotEnv []string
	stubBuildRunner(t, func(dir string, env []string, name string, args []string) (string, error) {
		gotDir, gotName, gotArgs, gotEnv = dir, name, args, env
		mutateModule(t, modulePath) // otelc rewrites module files
		return "", nil
	})

	tool := &otelcTool{Path: "/bin/otelc"}
	if err := buildWithOtelc(context.Background(), tool, modulePath, svc, binPath); err != nil {
		t.Fatal(err)
	}
	if gotDir != modulePath || gotName != "/bin/otelc" {
		t.Fatalf("dir=%q name=%q", gotDir, gotName)
	}
	if len(gotArgs) != 7 || gotArgs[0] != "--work-dir" || !slices.Equal(gotArgs[2:], []string{"go", "build", "-o", binPath, "./cmd/api"}) {
		t.Fatalf("args = %v", gotArgs)
	}
	if !slices.Contains(gotEnv, "GOWORK=off") {
		t.Fatal("builds must set GOWORK=off")
	}
	if got := readFile(t, filepath.Join(modulePath, "go.mod")); got != "module a\n" {
		t.Fatalf("go.mod not restored: %q", got)
	}
}

func TestBuildWithOtelc_RootPackage(t *testing.T) {
	modulePath := setupGuardModule(t, "module a\n", "")
	var gotArgs []string
	stubBuildRunner(t, func(dir string, env []string, name string, args []string) (string, error) {
		gotArgs = args
		return "", nil
	})
	svc := GoService{Name: "shop", RelDir: "."}
	if err := buildWithOtelc(context.Background(), &otelcTool{Path: "otelc"}, modulePath, svc, filepath.Join(modulePath, "bin", "shop")); err != nil {
		t.Fatal(err)
	}
	if gotArgs[len(gotArgs)-1] != "." {
		t.Fatalf("root package should be built as '.', got %v", gotArgs)
	}
}

func TestBuildWithOtelc_FailureClassifiesNetworkAndRestores(t *testing.T) {
	modulePath := setupGuardModule(t, "module a\n", "sum\n")
	stubBuildRunner(t, func(dir string, env []string, name string, args []string) (string, error) {
		mutateModule(t, modulePath)
		return "go: downloading x\ndial tcp: lookup proxy.golang.org: no such host", errors.New("exit status 1")
	})
	err := buildWithOtelc(context.Background(), &otelcTool{Path: "otelc"}, modulePath,
		GoService{Name: "api", RelDir: "."}, filepath.Join(modulePath, "bin", "api"))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{runtime.GOOS + "/" + runtime.GOARCH, "network error", "no such host"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q should contain %q", err, want)
		}
	}
	if got := readFile(t, filepath.Join(modulePath, "go.mod")); got != "module a\n" {
		t.Fatal("go.mod must be restored after a failed build")
	}
}

func TestBuildWithOtelc_Cancelled(t *testing.T) {
	modulePath := setupGuardModule(t, "module a\n", "sum\n")
	ctx, cancel := context.WithCancel(context.Background())
	stubBuildRunner(t, func(dir string, env []string, name string, args []string) (string, error) {
		mutateModule(t, modulePath)
		cancel()
		return "", errors.New("signal: killed")
	})
	err := buildWithOtelc(ctx, &otelcTool{Path: "otelc"}, modulePath,
		GoService{Name: "api", RelDir: "."}, filepath.Join(modulePath, "bin", "api"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation error, got %v", err)
	}
	if got := readFile(t, filepath.Join(modulePath, "go.mod")); got != "module a\n" {
		t.Fatal("go.mod must be restored after cancellation")
	}
}
