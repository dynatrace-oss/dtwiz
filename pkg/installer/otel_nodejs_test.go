package installer

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
		if len(plan.Entrypoints) == 0 || plan.Entrypoints[0] != "server.js" {
			t.Fatalf("Entrypoints = %v, want [server.js]", plan.Entrypoints)
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

	plan, _ := DetectNodePlan("https://tenant.live.dynatrace.com", "token")
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

	plan, _ := DetectNodePlan("https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected Node.js plan")
	}
	if len(plan.Entrypoints) == 0 || plan.Entrypoints[0] != "server.js" {
		t.Fatalf("Entrypoints = %v, want [server.js]", plan.Entrypoints)
	}
}

func TestNodeInstrumentationPlan_Runtime(t *testing.T) {
	plan := &NodeInstrumentationPlan{}
	if got := plan.Runtime(); got != "Node.js" {
		t.Fatalf("Runtime() = %q, want %q", got, "Node.js")
	}
}

func TestNodeInstrumentationPlan_PrintPlanSteps_Regular(t *testing.T) {
	plan := &NodeInstrumentationPlan{
		Project:        ScannedProject{Path: "/tmp/node-svc"},
		Entrypoints:    []string{"server.js"},
		PackageManager: "npm",
		OtelDir:        "/tmp/node-svc/.otel",
	}

	output := captureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{
		"/tmp/node-svc",
		"Package manager: npm",
		"/tmp/node-svc/.otel/",
		"npm install (in .otel/)",
		"node --require @opentelemetry/auto-instrumentations-node/register server.js  (service: node-svc)",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
	// Framework line should NOT appear for regular projects.
	if strings.Contains(output, "Framework:") {
		t.Fatalf("unexpected Framework line in regular project output:\n%s", output)
	}
}

func TestNodeInstrumentationPlan_PrintPlanSteps_NextJS(t *testing.T) {
	plan := &NodeInstrumentationPlan{
		Project:        ScannedProject{Path: "/tmp/next-app"},
		Entrypoints:    []string{"next:start"},
		PackageManager: "yarn",
		OtelDir:        "/tmp/next-app/.otel",
		Framework:      "next",
	}

	output := captureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{
		"/tmp/next-app",
		"Package manager: yarn",
		"Framework:       next",
		"npm install (in .otel/)",
		"node .otel/next-otel-bootstrap.js start",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestNodeInstrumentationPlan_PrintPlanSteps_Nuxt(t *testing.T) {
	plan := &NodeInstrumentationPlan{
		Project:        ScannedProject{Path: "/tmp/nuxt-app"},
		Entrypoints:    []string{"nuxt:start"},
		PackageManager: "pnpm",
		OtelDir:        "/tmp/nuxt-app/.otel",
		Framework:      "nuxt",
	}

	output := captureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{
		"/tmp/nuxt-app",
		"Package manager: pnpm",
		"Framework:       nuxt",
		"--import",
		"nuxt-otel-bootstrap.mjs",
		".output/server/index.mjs",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
	// Should NOT reference the old wrapper script.
	if strings.Contains(output, "nuxt-otel-bootstrap.js") {
		t.Fatalf("unexpected nuxt-otel-bootstrap.js reference in output:\n%s", output)
	}
}

func TestNodeInstrumentationPlan_PrintPlanSteps_ShowsPackageManager(t *testing.T) {
	for _, pm := range []string{"npm", "yarn", "pnpm"} {
		t.Run(pm, func(t *testing.T) {
			plan := &NodeInstrumentationPlan{
				Project:        ScannedProject{Path: "/tmp/svc"},
				Entrypoints:    []string{"index.js"},
				PackageManager: pm,
				OtelDir:        "/tmp/svc/.otel",
			}
			output := captureStdout(t, func() {
				plan.PrintPlanSteps()
			})
			if !strings.Contains(output, "Package manager: "+pm) {
				t.Fatalf("expected output to contain package manager %q, got:\n%s", pm, output)
			}
		})
	}
}

func TestNodeInstrumentationPlan_Execute(t *testing.T) {
	// Execute() now performs real operations (creates .otel/, runs npm install, etc).
	// This test verifies the old print-based stub was replaced; a full integration test
	// requires npm on PATH and is covered by end-to-end tests.
	t.Skip("Execute() is now a real implementation — tested via end-to-end tests")
}

// --- Task 3.3: createOtelDir tests ---

func TestCreateOtelDir_CreatesPackageJSON(t *testing.T) {
	dir := t.TempDir()
	otelDir := filepath.Join(dir, ".otel")
	plan := &NodeInstrumentationPlan{
		OtelDir: otelDir,
	}

	if err := createOtelDir(plan); err != nil {
		t.Fatalf("createOtelDir() error: %v", err)
	}

	pkgPath := filepath.Join(otelDir, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		t.Fatal("expected .otel/package.json to exist")
	}
}

func TestCreateOtelDir_PackageJSONContainsOtelDeps(t *testing.T) {
	dir := t.TempDir()
	otelDir := filepath.Join(dir, ".otel")
	plan := &NodeInstrumentationPlan{
		OtelDir: otelDir,
	}

	if err := createOtelDir(plan); err != nil {
		t.Fatalf("createOtelDir() error: %v", err)
	}

	pkgPath := filepath.Join(otelDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		t.Fatalf("read .otel/package.json: %v", err)
	}

	content := string(data)
	for _, pkg := range otelNodePackages {
		if !strings.Contains(content, pkg) {
			t.Errorf("expected .otel/package.json to contain %q, got:\n%s", pkg, content)
		}
	}

	// Verify it's valid JSON with a dependencies field.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON in .otel/package.json: %v", err)
	}
	deps, ok := parsed["dependencies"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'dependencies' field in .otel/package.json")
	}
	if len(deps) != len(otelNodePackages) {
		t.Errorf("expected %d dependencies, got %d", len(otelNodePackages), len(deps))
	}
}

// --- Task 3.4: generateWrapperJS tests ---

func TestGenerateWrapperJS_Next_NoEnvVarsEmbedded(t *testing.T) {
	content := generateWrapperJS("next")

	// Env vars should NOT be embedded — they are passed via cmd.Env at launch time.
	if strings.Contains(content, "process.env[") {
		t.Errorf("expected no process.env assignments in wrapper, got:\n%s", content)
	}
}

func TestGenerateWrapperJS_Next_DelegatesToNextCLI(t *testing.T) {
	content := generateWrapperJS("next")

	if !strings.Contains(content, "require('@opentelemetry/auto-instrumentations-node/register')") {
		t.Error("expected wrapper to require auto-instrumentations-node/register")
	}
	if !strings.Contains(content, "require('next/dist/bin/next')") {
		t.Error("expected wrapper to delegate to next/dist/bin/next")
	}
}

func TestGenerateWrapperJS_Nuxt_NoWrapper(t *testing.T) {
	// Nuxt doesn't use generateWrapperJS — it uses generateNuxtBootstrapMJS instead.
	// generateWrapperJS("nuxt", ...) should not contain any nuxt-specific delegation code.
	content := generateWrapperJS("nuxt")
	if strings.Contains(content, "nuxt") {
		t.Errorf("expected no nuxt references in wrapper, got:\n%s", content)
	}
}

func TestGenerateNuxtBootstrapMJS_ContainsModuleRegister(t *testing.T) {
	content := generateNuxtBootstrapMJS("/tmp/project/.otel")

	checks := []string{
		"import { register } from 'node:module'",
		"import { createRequire } from 'node:module'",
		"register(hookURL, import.meta.url)",
		"hook.mjs",
		"createRequire(import.meta.url)",
		"register.js",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("expected bootstrap to contain %q, got:\n%s", check, content)
		}
	}
}

func TestGenerateNuxtBootstrapMJS_UsesOtelDir(t *testing.T) {
	content := generateNuxtBootstrapMJS("/app/.otel")

	// Verify relative paths are used (works on Windows, macOS, and Linux)
	if !strings.Contains(content, "./node_modules/@opentelemetry/instrumentation/hook.mjs") {
		t.Errorf("expected bootstrap to reference hook.mjs with relative path, got:\n%s", content)
	}
	if !strings.Contains(content, "./node_modules/@opentelemetry/auto-instrumentations-node/build/src/register.js") {
		t.Errorf("expected bootstrap to reference register.js with relative path, got:\n%s", content)
	}
}

// --- Task 4.7: PrintPlanSteps shows running PIDs ---

func TestNodeInstrumentationPlan_PrintPlanSteps_ShowsRunningPIDs(t *testing.T) {
	plan := &NodeInstrumentationPlan{
		Project: ScannedProject{
			Path:              "/tmp/node-svc",
			RunningProcessIDs: []int{1234, 5678},
		},
		Entrypoints:    []string{"server.js"},
		PackageManager: "npm",
		OtelDir:        "/tmp/node-svc/.otel",
	}

	output := captureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	if !strings.Contains(output, "Stop running processes") {
		t.Fatalf("expected output to mention stopping processes, got:\n%s", output)
	}
	if !strings.Contains(output, "1234") || !strings.Contains(output, "5678") {
		t.Fatalf("expected output to contain PIDs 1234 and 5678, got:\n%s", output)
	}
}

// --- Prerequisite check: node_modules/ ---

func TestExecute_RegularApp_MissingNodeModules_DoesNotCreateOtelDir(t *testing.T) {
	dir := t.TempDir()
	// Project has a package.json and an entrypoint but NO node_modules/.
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"myapp"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(`console.log("hi")`), 0644); err != nil {
		t.Fatal(err)
	}

	otelDir := filepath.Join(dir, ".otel")
	plan := &NodeInstrumentationPlan{
		Project:        ScannedProject{Path: dir},
		Entrypoints:    []string{"index.js"},
		PackageManager: "npm",
		OtelDir:        otelDir,
		Framework:      "",
	}

	plan.Execute()

	if _, err := os.Stat(otelDir); !os.IsNotExist(err) {
		t.Errorf(".otel/ was created despite missing node_modules/ — expected early exit")
	}
}

func TestExecute_NextJSApp_MissingNodeModules_DoesNotCreateOtelDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "next.config.js"), []byte(`module.exports = {}`), 0644); err != nil {
		t.Fatal(err)
	}

	otelDir := filepath.Join(dir, ".otel")
	plan := &NodeInstrumentationPlan{
		Project:        ScannedProject{Path: dir},
		Entrypoints:    []string{"next:start"},
		PackageManager: "npm",
		OtelDir:        otelDir,
		Framework:      "next",
	}

	plan.Execute()

	if _, err := os.Stat(otelDir); !os.IsNotExist(err) {
		t.Errorf(".otel/ was created despite missing node_modules/ — expected early exit")
	}
}

// --- Task 2.3: generateOtelNodeEnvVars tests ---

func TestGenerateOtelNodeEnvVars_IncludesResourceDetectors(t *testing.T) {
	envVars := generateOtelNodeEnvVars("https://tenant.live.dynatrace.com", "dt0c01.TOKEN", "my-svc")

	if got := envVars["OTEL_NODE_RESOURCE_DETECTORS"]; got != "all" {
		t.Errorf("OTEL_NODE_RESOURCE_DETECTORS = %q, want %q", got, "all")
	}
}

func TestGenerateOtelNodeEnvVars_IncludesBaseVars(t *testing.T) {
	envVars := generateOtelNodeEnvVars("https://tenant.live.dynatrace.com", "dt0c01.TOKEN", "my-svc")

	// Check that all base vars are present.
	baseVars := []string{
		"OTEL_SERVICE_NAME",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_HEADERS",
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE",
		"OTEL_TRACES_EXPORTER",
		"OTEL_METRICS_EXPORTER",
		"OTEL_LOGS_EXPORTER",
	}
	for _, key := range baseVars {
		if _, ok := envVars[key]; !ok {
			t.Errorf("missing base env var %q", key)
		}
	}

	if got := envVars["OTEL_SERVICE_NAME"]; got != "my-svc" {
		t.Errorf("OTEL_SERVICE_NAME = %q, want %q", got, "my-svc")
	}

	wantEndpoint := "https://tenant.live.dynatrace.com/api/v2/otlp"
	if got := envVars["OTEL_EXPORTER_OTLP_ENDPOINT"]; got != wantEndpoint {
		t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want %q", got, wantEndpoint)
	}
}

// --- buildNodeInstrumentationPlan new field tests ---

func TestBuildNodeInstrumentationPlan_DetectsNextJS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected non-nil plan for Next.js project")
	}
	if plan.Framework != "next" {
		t.Errorf("Framework = %q, want %q", plan.Framework, "next")
	}
	if len(plan.Entrypoints) == 0 || plan.Entrypoints[0] != "next:start" {
		t.Errorf("Entrypoints = %v, want [next:start]", plan.Entrypoints)
	}
}

func TestBuildNodeInstrumentationPlan_DetectsNuxt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"nuxt":"3.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected non-nil plan for Nuxt project")
	}
	if plan.Framework != "nuxt" {
		t.Errorf("Framework = %q, want %q", plan.Framework, "nuxt")
	}
	if len(plan.Entrypoints) == 0 || plan.Entrypoints[0] != "nuxt:start" {
		t.Errorf("Entrypoints = %v, want [nuxt:start]", plan.Entrypoints)
	}
}

func TestBuildNodeInstrumentationPlan_DetectsPackageManager(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"main":"server.js"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.PackageManager != "yarn" {
		t.Errorf("PackageManager = %q, want %q", plan.PackageManager, "yarn")
	}
}

func TestBuildNodeInstrumentationPlan_SetsOtelDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"main":"server.js"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	expected := filepath.Join(dir, ".otel")
	if plan.OtelDir != expected {
		t.Errorf("OtelDir = %q, want %q", plan.OtelDir, expected)
	}
}

func TestBuildNodeInstrumentationPlan_UsesNodeEnvVars(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"main":"server.js"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if got := plan.EnvVars["OTEL_NODE_RESOURCE_DETECTORS"]; got != "all" {
		t.Errorf("EnvVars[OTEL_NODE_RESOURCE_DETECTORS] = %q, want %q", got, "all")
	}
}
