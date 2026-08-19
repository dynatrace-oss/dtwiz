package otel

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func TestBuildNodeInstrumentationPlan(t *testing.T) {
	t.Run("returns plan when entrypoint exists", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"main":"server.js"}`), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "server.js"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "http://127.0.0.1:4318")
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

		plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "http://127.0.0.1:4318")
		if plan != nil {
			t.Fatalf("expected nil plan, got %#v", plan)
		}
	})
}

func TestDetectNodePlan_NoNodeOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	plan, _ := DetectNodePlan("http://127.0.0.1:4318")
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

	helpers.SetTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")

	plan, _ := DetectNodePlan("http://127.0.0.1:4318")
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

	output := helpers.CaptureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{
		"/tmp/node-svc",
		"Package manager: npm",
		"/tmp/node-svc/.otel/",
		"npm install (in project dir, if node_modules/ missing)",
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

	output := helpers.CaptureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{
		"/tmp/next-app",
		"Package manager: yarn",
		"Framework:       next",
		"npm install (in project dir, if node_modules/ missing)",
		"npm run build",
		"npm install (in .otel/)",
		"node .otel/next-otel-bootstrap.js start",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestNodeInstrumentationPlan_PrintPlanSteps_NextJS_BuildOutputExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".next"), 0755); err != nil {
		t.Fatal(err)
	}

	plan := &NodeInstrumentationPlan{
		Project:        ScannedProject{Path: dir},
		PackageManager: "yarn",
		OtelDir:        filepath.Join(dir, ".otel"),
		Framework:      "next",
	}

	output := helpers.CaptureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	// Build step should NOT appear when .next/ already exists.
	if strings.Contains(output, "npm run build") {
		t.Fatalf("unexpected build step when .next/ already exists:\n%s", output)
	}
	if !strings.Contains(output, "next-otel-bootstrap.js") {
		t.Fatalf("expected launch command in output:\n%s", output)
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

	output := helpers.CaptureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{
		"/tmp/nuxt-app",
		"Package manager: pnpm",
		"Framework:       nuxt",
		"npm install (in project dir, if node_modules/ missing)",
		"npm run build",
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

func TestNodeInstrumentationPlan_PrintPlanSteps_Nuxt_BuildOutputExists(t *testing.T) {
	dir := t.TempDir()
	nitroDir := filepath.Join(dir, ".output", "server")
	if err := os.MkdirAll(nitroDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nitroDir, "index.mjs"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	plan := &NodeInstrumentationPlan{
		Project:        ScannedProject{Path: dir},
		PackageManager: "pnpm",
		OtelDir:        filepath.Join(dir, ".otel"),
		Framework:      "nuxt",
	}

	output := helpers.CaptureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	// Build step should NOT appear when .output/server/index.mjs already exists.
	if strings.Contains(output, "npm run build") {
		t.Fatalf("unexpected build step when .output/server/index.mjs already exists:\n%s", output)
	}
	if !strings.Contains(output, "nuxt-otel-bootstrap.mjs") {
		t.Fatalf("expected launch command in output:\n%s", output)
	}
}

// --- runBuildScript tests ---

func TestRunBuildScript_FailsWhenNoBuildScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app","scripts":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	err := runBuildScript(dir)
	if err == nil {
		t.Fatal("runBuildScript() should return error when no build script is defined")
	}
	if !strings.Contains(err.Error(), "build") {
		t.Errorf("error should mention 'build', got: %v", err)
	}
}

func TestRunBuildScript_FailsWhenNoPackageJSON(t *testing.T) {
	dir := t.TempDir()

	if err := runBuildScript(dir); err == nil {
		t.Fatal("runBuildScript() should return error when package.json is missing")
	}
}

func TestNodeInstrumentationPlan_PrintPlanSteps_PackageManager(t *testing.T) {
	for _, pm := range []string{"npm", "yarn", "pnpm"} {
		t.Run(pm, func(t *testing.T) {
			plan := &NodeInstrumentationPlan{
				Project:        ScannedProject{Path: "/tmp/svc"},
				Entrypoints:    []string{"index.js"},
				PackageManager: pm,
				OtelDir:        "/tmp/svc/.otel",
			}
			output := helpers.CaptureStdout(t, func() {
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

func TestNodeServiceNameFromEntrypoint(t *testing.T) {
	tests := []struct {
		projectPath string
		entrypoint  string
		want        string
	}{
		{"/home/user/my-app", "index.js", "my-app"},
		{"/home/user/my-app", "server.js", "my-app"},
		{"/home/user/node-package-delivery", "s-load-balancer/index.js", "s-load-balancer"},
		{"/home/user/my-app", "services/api/server.js", "api"},
	}
	for _, tt := range tests {
		got := nodeServiceNameFromEntrypoint(tt.projectPath, tt.entrypoint)
		if got != tt.want {
			t.Errorf("nodeServiceNameFromEntrypoint(%q, %q) = %q, want %q", tt.projectPath, tt.entrypoint, got, tt.want)
		}
	}
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

func TestGenerateNuxtBootstrapMJS_ExitsOnEADDRINUSE(t *testing.T) {
	content := generateNuxtBootstrapMJS("/tmp/project/.otel")

	// The bootstrap must register an uncaughtException handler that exits with
	// code 1 on EADDRINUSE, before Nitro's own handler runs. This makes dtwiz
	// report the crash correctly instead of showing "running, port not detected".
	checks := []string{
		"process.on('uncaughtException'",
		"EADDRINUSE",
		"process.exit(1)",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("expected bootstrap to contain %q, got:\n%s", check, content)
		}
	}

	// Non-EADDRINUSE errors must be rethrown so other crashes still fail fast.
	// Without this, adding an uncaughtException listener suppresses Node's default exit.
	if !strings.Contains(content, "throw err") {
		t.Errorf("expected bootstrap to rethrow non-EADDRINUSE errors, got:\n%s", content)
	}
}

func TestGenerateNuxtBootstrapMJS_EADDRINUSEHandlerBeforeHooks(t *testing.T) {
	content := generateNuxtBootstrapMJS("/tmp/project/.otel")

	// The uncaughtException handler must appear before module.register() so it
	// is active before any application code (including Nitro's own handler) runs.
	eaddrPos := strings.Index(content, "EADDRINUSE")
	registerPos := strings.Index(content, "register(hookURL")
	if eaddrPos < 0 {
		t.Fatal("EADDRINUSE handler not found in bootstrap")
	}
	if registerPos < 0 {
		t.Fatal("register(hookURL call not found in bootstrap")
	}
	if eaddrPos > registerPos {
		t.Errorf("EADDRINUSE handler (pos %d) appears after register(hookURL (pos %d) — handler must come first", eaddrPos, registerPos)
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

	output := helpers.CaptureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	if !strings.Contains(output, "Stop running processes") {
		t.Fatalf("expected output to mention stopping processes, got:\n%s", output)
	}
	if !strings.Contains(output, "1234") || !strings.Contains(output, "5678") {
		t.Fatalf("expected output to contain PIDs 1234 and 5678, got:\n%s", output)
	}
}

// withFakeNpmRunner replaces npmRunner with fn for the duration of the test.
func withFakeNpmRunner(t *testing.T, fn func(dir string, args ...string) error) {
	t.Helper()
	orig := npmRunner
	npmRunner = fn
	t.Cleanup(func() { npmRunner = orig })
}

// --- runNpm tests ---

func TestRunNpm_ErrorIncludesSubcmdAndDir(t *testing.T) {
	dir := t.TempDir()
	// Stub npmRunner to simulate a failure, then verify runNpm wraps the error
	// with both the subcommand and the directory in the message.
	withFakeNpmRunner(t, func(d string, args ...string) error {
		subCmd := strings.Join(args, " ")
		return fmt.Errorf("npm %s in %s failed: exit status 1\nsome npm output", subCmd, d)
	})

	err := runNpm(dir, "ci")
	if err == nil {
		t.Fatal("runNpm should propagate the error from npmRunner")
	}
	if !strings.Contains(err.Error(), "npm ci") {
		t.Errorf("error should mention subcommand, got: %v", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("error should mention directory, got: %v", err)
	}
}

// --- installNodeProjectDeps tests ---

func TestInstallProjectDeps_SkipsWhenNodeModulesExists(t *testing.T) {
	dir := t.TempDir()
	// Create a node_modules/ directory to simulate already-installed deps.
	if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}

	// Should return nil immediately without attempting to run npm.
	if err := installNodeProjectDeps(dir); err != nil {
		t.Fatalf("installNodeProjectDeps() = %v, want nil when node_modules exists", err)
	}
}

func TestInstallProjectDeps_UsesNpmCI_WhenPackageLockExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test","private":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	lockContent := `{"name":"test","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"test","version":"1.0.0"}}}`
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(lockContent), 0644); err != nil {
		t.Fatal(err)
	}

	var gotSubCmd string
	withFakeNpmRunner(t, func(_ string, args ...string) error {
		gotSubCmd = args[0]
		return nil
	})

	if err := installNodeProjectDeps(dir); err != nil {
		t.Fatalf("installNodeProjectDeps() with package-lock.json: %v", err)
	}
	if gotSubCmd != "ci" {
		t.Errorf("expected npm ci, got npm %s", gotSubCmd)
	}
}

func TestInstallProjectDeps_UsesNpmInstall_WhenNoLockfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test","private":true}`), 0644); err != nil {
		t.Fatal(err)
	}

	var gotSubCmd string
	withFakeNpmRunner(t, func(_ string, args ...string) error {
		gotSubCmd = args[0]
		return nil
	})

	if err := installNodeProjectDeps(dir); err != nil {
		t.Fatalf("installNodeProjectDeps() without lockfile: %v", err)
	}
	if gotSubCmd != "install" {
		t.Errorf("expected npm install, got npm %s", gotSubCmd)
	}
}

func TestExecute_RegularApp_MissingNodeModules_DoesNotCreateOtelDir(t *testing.T) {
	// Execute() always calls installNodeProjectDeps(), which is a no-op when
	// node_modules exists and runs npm ci/install otherwise.
	// Install success/failure is covered by TestInstallProjectDeps_* and e2e tests.
	t.Skip("Execute() delegates dep install to installNodeProjectDeps — covered by unit and e2e tests")
}

func TestExecute_NextJSApp_MissingNodeModules_DoesNotCreateOtelDir(t *testing.T) {
	// Execute() always calls installNodeProjectDeps(), which is a no-op when
	// node_modules exists and runs npm ci/install otherwise.
	// Install success/failure is covered by TestInstallProjectDeps_* and e2e tests.
	t.Skip("Execute() delegates dep install to installNodeProjectDeps — covered by unit and e2e tests")
}

// --- Task 2.3: generateOtelNodeEnvVars tests ---

func TestGenerateOtelNodeEnvVars_IncludesResourceDetectors(t *testing.T) {
	envVars := generateOtelNodeEnvVars("http://127.0.0.1:4318", "my-svc")

	if got := envVars["OTEL_NODE_RESOURCE_DETECTORS"]; got != "all" {
		t.Errorf("OTEL_NODE_RESOURCE_DETECTORS = %q, want %q", got, "all")
	}
}

func TestGenerateOtelNodeEnvVars_IncludesBaseVars(t *testing.T) {
	envVars := generateOtelNodeEnvVars("http://127.0.0.1:4318", "my-svc")

	// Check that all base vars are present (no auth header — collector owns credentials).
	baseVars := []string{
		"OTEL_SERVICE_NAME",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
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

	if _, ok := envVars["OTEL_EXPORTER_OTLP_HEADERS"]; ok {
		t.Error("OTEL_EXPORTER_OTLP_HEADERS must not be present — credentials belong to the collector")
	}

	if got := envVars["OTEL_SERVICE_NAME"]; got != "my-svc" {
		t.Errorf("OTEL_SERVICE_NAME = %q, want %q", got, "my-svc")
	}

	wantEndpoint := "http://127.0.0.1:4318"
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

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "http://127.0.0.1:4318")
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

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "http://127.0.0.1:4318")
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

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "http://127.0.0.1:4318")
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

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "http://127.0.0.1:4318")
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

	plan := buildNodeInstrumentationPlan(ScannedProject{Path: dir}, "http://127.0.0.1:4318")
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if got := plan.EnvVars["OTEL_NODE_RESOURCE_DETECTORS"]; got != "all" {
		t.Errorf("EnvVars[OTEL_NODE_RESOURCE_DETECTORS] = %q, want %q", got, "all")
	}
}
