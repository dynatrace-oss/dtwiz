package otel

import (
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGoInitSource_IsValidGo(t *testing.T) {
	if _, err := parser.ParseFile(token.NewFileSet(), "init.go", goInitSource, parser.AllErrors); err != nil {
		t.Fatalf("embedded init source does not parse: %v", err)
	}
}

// The injected init must not replace the application's logger: output keeps
// going to its original destination and the OTLP copy is added.
func TestGoInitSource_KeepsApplicationLogging(t *testing.T) {
	if strings.Contains(goInitSource, "SetDefault(dtwizotelslog") {
		t.Error("slog.SetDefault must not be given the OTel-only logger")
	}
	for _, want := range []string{"dtwizTeeHandler", "dtwizlog.SetOutput(logOut)", "WithBatchTimeout", "WithExportInterval"} {
		if !strings.Contains(goInitSource, want) {
			t.Errorf("init source lost %q", want)
		}
	}
}

func TestPrepareGoFallback_OverlayAndIsolation(t *testing.T) {
	module := setupGuardModule(t, "module a\n", "sum a\n")
	services := []GoService{
		{Name: "api", Dir: filepath.Join(module, "cmd", "api"), RelDir: "cmd/api"},
		{Name: "worker", Dir: filepath.Join(module, "cmd", "worker"), RelDir: "cmd/worker"},
	}

	f, err := prepareGoFallback(module, services)
	if err != nil {
		t.Fatal(err)
	}

	var overlay struct{ Replace map[string]string }
	if err := json.Unmarshal([]byte(readFile(t, f.overlay)), &overlay); err != nil {
		t.Fatal(err)
	}
	if len(overlay.Replace) != 2 {
		t.Fatalf("expected one overlay entry per main package, got %v", overlay.Replace)
	}
	for _, s := range services {
		target, ok := overlay.Replace[filepath.Join(s.Dir, goInitVirtualName)]
		if !ok {
			t.Fatalf("missing overlay entry for %s", s.Dir)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("overlay target missing: %v", err)
		}
	}

	if readFile(t, filepath.Join(module, "go.mod")) != "module a\n" || readFile(t, filepath.Join(module, "go.sum")) != "sum a\n" {
		t.Fatal("project go.mod/go.sum must be untouched")
	}
	if readFile(t, f.modFile) != "module a\n" {
		t.Fatal("isolated go.mod should start as a copy")
	}
}

func TestGoOverlayJSON_EscapesWindowsPaths(t *testing.T) {
	svc := GoService{Dir: `C:\work\shop\cmd\api`}
	data, err := goOverlayJSON([]GoService{svc}, `C:\work\shop\.otel\go\otel_init.go.src`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `C:\\work\\shop\\.otel\\go\\otel_init.go.src`) {
		t.Fatalf("backslashes must be escaped in the overlay JSON:\n%s", data)
	}
	var parsed map[string]map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("overlay is not valid JSON: %v", err)
	}
}

func TestBuildClientOnly_CommandsAndGOWORK(t *testing.T) {
	module := setupGuardModule(t, "module a\n", "")
	svc := GoService{Name: "api", Dir: module, RelDir: "."}
	f, err := prepareGoFallback(module, []GoService{svc})
	if err != nil {
		t.Fatal(err)
	}

	type call struct {
		args []string
		env  []string
	}
	var calls []call
	stubBuildRunner(t, func(dir string, env []string, name string, args []string) (string, error) {
		calls = append(calls, call{args, env})
		return "", nil
	})

	bin := filepath.Join(goOtelDir(module), "bin", "api")
	if err := f.buildClientOnly(context.Background(), svc, bin); err != nil {
		t.Fatal(err)
	}
	if err := f.buildClientOnly(context.Background(), svc, bin); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 { // one `go get` shared by both builds
		t.Fatalf("expected go get once plus two builds, got %d calls", len(calls))
	}
	if calls[0].args[0] != "get" || !slices.Contains(calls[0].args, "-modfile="+f.modFile) {
		t.Fatalf("go get args = %v", calls[0].args)
	}
	build := calls[1]
	for _, want := range []string{"build", "-modfile=" + f.modFile, "-overlay=" + f.overlay, "-o", bin} {
		if !slices.Contains(build.args, want) {
			t.Fatalf("build args %v missing %q", build.args, want)
		}
	}
	for _, c := range calls {
		if !slices.Contains(c.env, "GOWORK=off") {
			t.Fatal("all builds must set GOWORK=off")
		}
	}
	if readFile(t, filepath.Join(module, "go.mod")) != "module a\n" {
		t.Fatal("project go.mod must not change")
	}
}

func TestBuildClientOnly_Failure(t *testing.T) {
	module := setupGuardModule(t, "module a\n", "")
	svc := GoService{Name: "api", Dir: module, RelDir: "."}
	f, err := prepareGoFallback(module, []GoService{svc})
	if err != nil {
		t.Fatal(err)
	}
	stubBuildRunner(t, func(dir string, env []string, name string, args []string) (string, error) {
		return "boom", errors.New("exit status 1")
	})
	if err := f.buildClientOnly(context.Background(), svc, filepath.Join(module, "bin", "api")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected failure carrying build output, got %v", err)
	}
}
