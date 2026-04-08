package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectNodeProjects_Found(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	projects := detectNodeProjects()
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected project at %s, got %v", dir, projects)
	}
}

func TestDetectNodeProjects_ExcludesNodeModules(t *testing.T) {
	dir := t.TempDir()
	// Create node_modules subdirectory with a package.json inside.
	nmDir := filepath.Join(dir, "node_modules", "somelib")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Create the real project package.json.
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	projects := detectNodeProjects()
	for _, p := range projects {
		if filepath.Base(filepath.Dir(p.Path)) == "node_modules" {
			t.Errorf("node_modules project should be excluded, found: %s", p.Path)
		}
	}
}

func TestDetectNodeEntrypoints_Main(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"main":"server.js"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "server.js" {
		t.Errorf("expected [server.js], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_ScriptsStart(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"start":"node app.js"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "app.js" {
		t.Errorf("expected [app.js], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_Fallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "index.js" {
		t.Errorf("expected [index.js], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_TypeScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Only a TypeScript variant exists.
	if err := os.WriteFile(filepath.Join(dir, "app.ts"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "app.ts" {
		t.Errorf("expected [app.ts], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_None(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 0 {
		t.Errorf("expected empty entrypoints, got %v", eps)
	}
}

func TestBuildNodeInstrumentationPlan(t *testing.T) {
	apiURL := "https://tenant.live.dynatrace.com"
	token := "token"

	t.Run("returns plan when entrypoint exists", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"main":"server.js"}`), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "server.js"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, apiURL, token)
		if plan == nil {
			t.Fatal("expected non-nil plan")
		}
		if plan.Entrypoint != "server.js" {
			t.Fatalf("Entrypoint = %q, want %q", plan.Entrypoint, "server.js")
		}
	})

	t.Run("returns nil when no entrypoint exists", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"svc"}`), 0644); err != nil {
			t.Fatal(err)
		}

		plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, apiURL, token)
		if plan != nil {
			t.Fatalf("expected nil plan, got %#v", plan)
		}
	})
}

func TestDetectNodePlan_NoNodeOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	plan := DetectNodePlan("https://tenant.live.dynatrace.com", "token")
	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}
}

func TestDetectNodePlan_FindsProject(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed on PATH")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"main":"server.js"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.js"), []byte("console.log('ok')\n"), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")

	plan := DetectNodePlan("https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected Node.js plan")
	}
	if plan.Entrypoint != "server.js" {
		t.Fatalf("Entrypoint = %q, want %q", plan.Entrypoint, "server.js")
	}
}

func TestNodeInstrumentationPlan_Runtime(t *testing.T) {
	plan := &NodeInstrumentationPlan{}
	if got := plan.Runtime(); got != "Node.js" {
		t.Fatalf("Runtime() = %q, want %q", got, "Node.js")
	}
}

func TestNodeInstrumentationPlan_PrintPlanSteps(t *testing.T) {
	plan := &NodeInstrumentationPlan{Project: ScannedProject{Path: "/tmp/node-svc"}, Entrypoint: "server.js"}

	output := captureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{"/tmp/node-svc", "npm install @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @opentelemetry/exporter-trace-otlp-http", "node --require @opentelemetry/auto-instrumentations-node/register server.js"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestNodeInstrumentationPlan_Execute(t *testing.T) {
	plan := &NodeInstrumentationPlan{
		Project:    ScannedProject{Path: "/tmp/node-svc"},
		Entrypoint: "server.js",
		EnvVars:    map[string]string{"OTEL_SERVICE_NAME": "node-svc"},
	}

	output := captureStdout(t, func() {
		plan.Execute()
	})

	checks := []string{"cd /tmp/node-svc", "npm install @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @opentelemetry/exporter-trace-otlp-http", "export OTEL_SERVICE_NAME=\"node-svc\"", "node --require @opentelemetry/auto-instrumentations-node/register server.js"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}
