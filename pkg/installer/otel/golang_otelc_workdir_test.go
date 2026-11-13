package otel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOtelcWorkDirs(t *testing.T) {
	var w otelcWorkDirs
	a1, err := w.For("/m/a")
	if err != nil {
		t.Fatal(err)
	}
	a2, _ := w.For("/m/a")
	b, _ := w.For("/m/b")
	if a1 != a2 || a1 == b {
		t.Fatalf("same module must share a directory, different modules must not: %q %q %q", a1, a2, b)
	}

	w.Reset("/m/a")
	if _, err := os.Stat(a1); !os.IsNotExist(err) {
		t.Fatal("Reset must remove the directory")
	}
	if a3, _ := w.For("/m/a"); a3 == a1 {
		t.Fatal("after Reset the module gets a new directory")
	}

	w.Close()
	if _, err := os.Stat(b); !os.IsNotExist(err) {
		t.Fatal("Close must remove every directory")
	}
	w.Close() // idempotent
}

func TestBuildWithOtelc_UsesAndKeepsGivenWorkDir(t *testing.T) {
	modulePath := setupGuardModule(t, "module a\n", "sum\n")
	workDir := t.TempDir()
	var gotArgs []string
	stubBuildRunner(t, func(_ string, _ []string, _ string, args []string) (string, error) {
		gotArgs = args
		return "", nil
	})
	err := buildWithOtelc(context.Background(), &otelcTool{Path: "otelc"}, modulePath, workDir,
		GoService{Name: "api", RelDir: "."}, filepath.Join(modulePath, "bin", "api"))
	if err != nil {
		t.Fatal(err)
	}
	if len(gotArgs) < 2 || gotArgs[0] != "--work-dir" || gotArgs[1] != workDir {
		t.Fatalf("args = %v", gotArgs)
	}
	if _, err := os.Stat(workDir); err != nil {
		t.Fatal("a work directory owned by the caller must not be removed")
	}
}

func TestExecute_SharesOtelcWorkDirPerModuleAndCleansUp(t *testing.T) {
	newHarness(t)
	stubGoToolchain(t, true)
	root := newGoProject(t, map[string]string{"cmd/api/main.go": mainSrc, "cmd/worker/main.go": mainSrc, "cmd/jobs/main.go": mainSrc})
	plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)
	if plan == nil || len(plan.Services) != 3 {
		t.Fatalf("plan = %#v", plan)
	}

	dirs := map[string]string{}
	var order []string
	goBuildOtelcFn = func(_ context.Context, _ *otelcTool, _ string, workDir string, svc GoService, _ string) error {
		if _, err := os.Stat(workDir); err != nil {
			t.Errorf("work directory must exist during the build of %s: %v", svc.Name, err)
		}
		dirs[svc.Name] = workDir
		order = append(order, svc.Name)
		if svc.Name == "jobs" {
			return errors.New("otelc build failed")
		}
		return nil
	}
	plan.Execute()

	// Services build in name order: api and jobs share the directory; jobs fails,
	// so worker starts in a clean one.
	if dirs["api"] == "" || dirs["api"] != dirs["jobs"] || dirs["worker"] == dirs["jobs"] {
		t.Fatalf("services of one module share a work directory until a build fails: %v", dirs)
	}
	for name, dir := range dirs {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("work directory of %s must be removed after the run", name)
		}
	}
}

func TestExecute_FailedBuildResetsOtelcWorkDir(t *testing.T) {
	newHarness(t)
	stubGoToolchain(t, true)
	root := newGoProject(t, map[string]string{"cmd/api/main.go": mainSrc, "cmd/worker/main.go": mainSrc})
	plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)

	dirs := map[string]string{}
	goBuildOtelcFn = func(_ context.Context, _ *otelcTool, _ string, workDir string, svc GoService, _ string) error {
		dirs[svc.Name] = workDir
		if svc.Name == "api" {
			return errors.New("otelc build failed")
		}
		return nil
	}
	plan.Execute()

	if dirs["api"] == "" || dirs["worker"] == "" || dirs["api"] == dirs["worker"] {
		t.Fatalf("after a failed build the next service gets a clean directory: %v", dirs)
	}
}
