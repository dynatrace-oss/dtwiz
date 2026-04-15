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

func TestNodeInstrumentationPlan_PrintPlanSteps_Regular(t *testing.T) {
	plan := &NodeInstrumentationPlan{
		Project:        ScannedProject{Path: "/tmp/node-svc"},
		Entrypoint:     "server.js",
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
		"node --require @opentelemetry/auto-instrumentations-node/register server.js",
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
		Entrypoint:     "next:start",
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
		"node .otel/next-register.js start",
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
		Entrypoint:     "nuxt:start",
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
		"node .otel/nuxt-register.js start",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestNodeInstrumentationPlan_PrintPlanSteps_ShowsPackageManager(t *testing.T) {
	for _, pm := range []string{"npm", "yarn", "pnpm"} {
		t.Run(pm, func(t *testing.T) {
			plan := &NodeInstrumentationPlan{
				Project:        ScannedProject{Path: "/tmp/svc"},
				Entrypoint:     "index.js",
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
	plan := &NodeInstrumentationPlan{
		Project:    ScannedProject{Path: "/tmp/node-svc"},
		Entrypoint: "server.js",
		EnvVars:    map[string]string{"OTEL_SERVICE_NAME": "node-svc"},
	}

	output := captureStdout(t, func() {
		plan.Execute()
	})

	checks := []string{
		"cd /tmp/node-svc",
		"npm install @opentelemetry/auto-instrumentations-node @opentelemetry/sdk-node @opentelemetry/exporter-trace-otlp-http @opentelemetry/exporter-metrics-otlp-http @opentelemetry/exporter-logs-otlp-http",
		"export OTEL_SERVICE_NAME=\"node-svc\"",
		"node --require @opentelemetry/auto-instrumentations-node/register server.js",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

// --- Task 1.1a: isNextJSProject tests ---

func TestIsNextJSProject_ConfigFile(t *testing.T) {
	for _, configFile := range []string{"next.config.js", "next.config.ts", "next.config.mjs"} {
		t.Run(configFile, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, configFile), []byte(""), 0644); err != nil {
				t.Fatal(err)
			}
			if !isNextJSProject(dir) {
				t.Errorf("expected isNextJSProject=true for %s", configFile)
			}
		})
	}
}

func TestIsNextJSProject_PackageDep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if !isNextJSProject(dir) {
		t.Error("expected isNextJSProject=true for next in dependencies")
	}
}

func TestIsNextJSProject_DevDep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"next":"14.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if !isNextJSProject(dir) {
		t.Error("expected isNextJSProject=true for next in devDependencies")
	}
}

func TestIsNextJSProject_NotNextJS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"express":"4.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if isNextJSProject(dir) {
		t.Error("expected isNextJSProject=false for non-Next.js project")
	}
}

// --- Task 1.1b: detectNodeFramework tests ---

func TestDetectNodeFramework_NuxtConfigFile(t *testing.T) {
	for _, configFile := range []string{"nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs"} {
		t.Run(configFile, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, configFile), []byte(""), 0644); err != nil {
				t.Fatal(err)
			}
			if got := detectNodeFramework(dir); got != "nuxt" {
				t.Errorf("detectNodeFramework() = %q, want %q for %s", got, "nuxt", configFile)
			}
		})
	}
}

func TestDetectNodeFramework_NuxtDep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"nuxt":"3.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodeFramework(dir); got != "nuxt" {
		t.Errorf("detectNodeFramework() = %q, want %q", got, "nuxt")
	}
}

func TestDetectNodeFramework_NextTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0","nuxt":"3.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodeFramework(dir); got != "next" {
		t.Errorf("detectNodeFramework() = %q, want %q (Next.js takes precedence)", got, "next")
	}
}

func TestDetectNodeFramework_Regular(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"express":"4.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodeFramework(dir); got != "" {
		t.Errorf("detectNodeFramework() = %q, want empty string for regular project", got)
	}
}

// --- Task 1.2: detectNodePackageManager tests ---

func TestDetectNodePackageManager_NPM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodePackageManager(dir); got != "npm" {
		t.Errorf("detectNodePackageManager() = %q, want %q", got, "npm")
	}
}

func TestDetectNodePackageManager_Yarn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodePackageManager(dir); got != "yarn" {
		t.Errorf("detectNodePackageManager() = %q, want %q", got, "yarn")
	}
}

func TestDetectNodePackageManager_PNPM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodePackageManager(dir); got != "pnpm" {
		t.Errorf("detectNodePackageManager() = %q, want %q", got, "pnpm")
	}
}

func TestDetectNodePackageManager_Default(t *testing.T) {
	dir := t.TempDir()
	if got := detectNodePackageManager(dir); got != "npm" {
		t.Errorf("detectNodePackageManager() = %q, want %q (default)", got, "npm")
	}
}

// --- Task 1.3: Monorepo detection tests ---

func TestDetectNodeProjects_Monorepo(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	// Root package.json with workspaces.
	rootPkg := `{"name":"monorepo","workspaces":["packages/*"]}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatal(err)
	}

	// Create workspace packages.
	for _, pkg := range []string{"api", "web"} {
		pkgDir := filepath.Join(dir, "packages", pkg)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name":"`+pkg+`"}`), 0644); err != nil {
			t.Fatal(err)
		}
	}

	setTestWorkingDir(t, dir)
	projects := detectNodeProjects()

	// Should include the root and both workspace packages.
	paths := make(map[string]bool)
	for _, p := range projects {
		paths[p.Path] = true
	}

	// Check root is present.
	if !paths[dir] && !paths[realDir] {
		t.Errorf("expected monorepo root in projects, got %v", projects)
	}

	// Check workspace packages are present.
	for _, pkg := range []string{"api", "web"} {
		pkgPath := filepath.Join(dir, "packages", pkg)
		realPkgPath := filepath.Join(realDir, "packages", pkg)
		if !paths[pkgPath] && !paths[realPkgPath] {
			t.Errorf("expected workspace package %s in projects, got %v", pkg, projects)
		}
	}
}

func TestResolveWorkspaces_ArrayFormat(t *testing.T) {
	dir := t.TempDir()

	rootPkg := `{"workspaces":["packages/*"]}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(dir, "packages", "lib")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := resolveWorkspaces(dir)
	if len(dirs) != 1 {
		t.Fatalf("expected 1 workspace dir, got %v", dirs)
	}
	if dirs[0] != pkgDir {
		t.Errorf("expected %q, got %q", pkgDir, dirs[0])
	}
}

func TestResolveWorkspaces_ObjectFormat(t *testing.T) {
	dir := t.TempDir()

	rootPkg := `{"workspaces":{"packages":["packages/*"]}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(dir, "packages", "lib")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := resolveWorkspaces(dir)
	if len(dirs) != 1 {
		t.Fatalf("expected 1 workspace dir, got %v", dirs)
	}
	if dirs[0] != pkgDir {
		t.Errorf("expected %q, got %q", pkgDir, dirs[0])
	}
}

func TestResolveWorkspaces_NoWorkspaces(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := resolveWorkspaces(dir)
	if len(dirs) != 0 {
		t.Errorf("expected no workspaces, got %v", dirs)
	}
}

func TestResolveWorkspaces_SkipsDirWithoutPackageJSON(t *testing.T) {
	dir := t.TempDir()

	rootPkg := `{"workspaces":["packages/*"]}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a workspace dir without package.json.
	emptyDir := filepath.Join(dir, "packages", "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	dirs := resolveWorkspaces(dir)
	if len(dirs) != 0 {
		t.Errorf("expected no workspaces (no package.json), got %v", dirs)
	}
}

// --- Task 1.4/1.5: detectNodeEntrypoints for Next.js / Nuxt ---

func TestDetectNodeEntrypoints_NextJS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 1 || eps[0] != "next:start" {
		t.Errorf("expected [next:start], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_Nuxt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"nuxt":"3.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 1 || eps[0] != "nuxt:start" {
		t.Errorf("expected [nuxt:start], got %v", eps)
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
	if plan.Entrypoint != "next:start" {
		t.Errorf("Entrypoint = %q, want %q", plan.Entrypoint, "next:start")
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
	if plan.Entrypoint != "nuxt:start" {
		t.Errorf("Entrypoint = %q, want %q", plan.Entrypoint, "nuxt:start")
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
