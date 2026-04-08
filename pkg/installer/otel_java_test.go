package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectJavaProjects_Maven(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	projects := detectJavaProjects()
	if len(projects) == 0 {
		t.Fatal("expected at least one Java project, got none")
	}
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
			if len(p.Markers) == 0 || p.Markers[0] != "pom.xml" {
				t.Errorf("expected marker pom.xml, got %v", p.Markers)
			}
		}
	}
	if !found {
		t.Errorf("dir %s not found in projects %v", dir, projects)
	}
}

func TestDetectJavaProjects_Gradle(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	projects := detectJavaProjects()
	if len(projects) == 0 {
		t.Fatal("expected at least one Java project, got none")
	}
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
			hasGradle := false
			for _, m := range p.Markers {
				if m == "build.gradle" {
					hasGradle = true
				}
			}
			if !hasGradle {
				t.Errorf("expected marker build.gradle, got %v", p.Markers)
			}
		}
	}
	if !found {
		t.Errorf("dir %s not found in projects %v", dir, projects)
	}
}

func TestDetectJavaProjects_None(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	setTestWorkingDir(t, dir)
	projects := detectJavaProjects()
	for _, p := range projects {
		// The temp dir itself should not appear (no markers).
		if p.Path == dir || p.Path == realDir {
			t.Errorf("unexpected project at %s with no Java markers", dir)
		}
	}
}

func TestDetectJava_NotFound(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := detectJava()
	if err == nil {
		t.Fatal("expected error when java is not on PATH")
	}
}

func TestDetectJavaPlan_NoJavaOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	plan := DetectJavaPlan("https://tenant.live.dynatrace.com", "token")
	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}
}

func TestDetectJavaPlan_FindsProject(t *testing.T) {
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java not installed on PATH")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")

	plan := DetectJavaPlan("https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected Java plan")
	}
	if plan.Project.Path == "" {
		t.Fatal("expected selected project path to be set")
	}
}

func TestJavaInstrumentationPlan_Runtime(t *testing.T) {
	plan := &JavaInstrumentationPlan{}
	if got := plan.Runtime(); got != "Java" {
		t.Fatalf("Runtime() = %q, want %q", got, "Java")
	}
}

func TestJavaInstrumentationPlan_PrintPlanSteps(t *testing.T) {
	plan := &JavaInstrumentationPlan{Project: ScannedProject{Path: "/tmp/service"}}

	output := captureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{"/tmp/service", otelJavaAgentURL, "java -javaagent:opentelemetry-javaagent.jar -jar your_app.jar"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestJavaInstrumentationPlan_Execute(t *testing.T) {
	plan := &JavaInstrumentationPlan{
		EnvVars: map[string]string{"OTEL_SERVICE_NAME": "orders-api"},
	}

	output := captureStdout(t, func() {
		plan.Execute()
	})

	checks := []string{"Download the OpenTelemetry Java agent:", otelJavaAgentURL, "export OTEL_SERVICE_NAME=\"orders-api\"", "java -javaagent:opentelemetry-javaagent.jar -jar your_app.jar"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}
