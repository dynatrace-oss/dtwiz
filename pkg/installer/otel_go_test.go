package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectGoProjects_Found(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	goMod := "module github.com/example/myapp\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	projects := detectGoProjects()
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
			if p.ModuleName != "github.com/example/myapp" {
				t.Errorf("expected module name github.com/example/myapp, got %q", p.ModuleName)
			}
		}
	}
	if !found {
		t.Errorf("expected Go project at %s, got %v", dir, projects)
	}
}

func TestDetectGoProjects_None(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	setTestWorkingDir(t, dir)
	projects := detectGoProjects()
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			t.Errorf("unexpected Go project at %s with no go.mod", dir)
		}
	}
}

func TestExtractGoModuleName(t *testing.T) {
	dir := t.TempDir()
	content := "module github.com/acme/service\n\ngo 1.22\n"
	goModPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := extractGoModuleName(goModPath)
	if got != "github.com/acme/service" {
		t.Errorf("expected github.com/acme/service, got %q", got)
	}
}

func TestExtractGoModuleName_MissingOrWithoutModule(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		got := extractGoModuleName(filepath.Join(t.TempDir(), "go.mod"))
		if got != "" {
			t.Fatalf("expected empty module name, got %q", got)
		}
	})

	t.Run("no module line", func(t *testing.T) {
		dir := t.TempDir()
		goModPath := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(goModPath, []byte("go 1.22\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := extractGoModuleName(goModPath)
		if got != "" {
			t.Fatalf("expected empty module name, got %q", got)
		}
	})
}

func TestDetectGoPlan_NoGoOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	plan := DetectGoPlan("https://tenant.live.dynatrace.com", "token")
	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}
}

func TestDetectGoPlan_FindsProject(t *testing.T) {
	dir := t.TempDir()
	goMod := "module github.com/example/myapp\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")

	plan := DetectGoPlan("https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected Go plan")
	}
	if plan.Project.ModuleName != "github.com/example/myapp" {
		t.Fatalf("ModuleName = %q, want %q", plan.Project.ModuleName, "github.com/example/myapp")
	}
}

func TestGoInstrumentationPlan_Runtime(t *testing.T) {
	plan := &GoInstrumentationPlan{}
	if got := plan.Runtime(); got != "Go" {
		t.Fatalf("Runtime() = %q, want %q", got, "Go")
	}
}

func TestGoInstrumentationPlan_PrintPlanSteps(t *testing.T) {
	plan := &GoInstrumentationPlan{Project: GoProject{ScannedProject: ScannedProject{Path: "/tmp/go-svc"}, ModuleName: "github.com/example/go-svc"}}

	output := captureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{"/tmp/go-svc", "github.com/example/go-svc", "go get go.opentelemetry.io/otel", "go get go.opentelemetry.io/otel/sdk"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestGoInstrumentationPlan_Execute(t *testing.T) {
	plan := &GoInstrumentationPlan{
		Project: GoProject{ScannedProject: ScannedProject{Path: "/tmp/go-svc"}},
		EnvVars: map[string]string{"OTEL_SERVICE_NAME": "go-svc"},
	}

	output := captureStdout(t, func() {
		plan.Execute()
	})

	checks := []string{"cd /tmp/go-svc", "go get go.opentelemetry.io/otel", "export OTEL_SERVICE_NAME=\"go-svc\"", "Initialize the OTel SDK in your main() function."}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}
