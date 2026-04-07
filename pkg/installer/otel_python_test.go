package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── generateOtelPythonEnvVars ─────────────────────────────────────────────────

func TestGenerateOtelPythonEnvVars_ContainsBase(t *testing.T) {
	envVars := generateOtelPythonEnvVars("https://abc123.live.dynatrace.com", "dt0c01.TOKEN", "my-svc")

	wantEndpoint := "https://abc123.live.dynatrace.com/api/v2/otlp"
	if got := envVars["OTEL_EXPORTER_OTLP_ENDPOINT"]; got != wantEndpoint {
		t.Errorf("ENDPOINT = %q, want %q", got, wantEndpoint)
	}
	if got := envVars["OTEL_SERVICE_NAME"]; got != "my-svc" {
		t.Errorf("SERVICE_NAME = %q, want %q", got, "my-svc")
	}
}

func TestGenerateOtelPythonEnvVars_PythonLoggingEnabled(t *testing.T) {
	envVars := generateOtelPythonEnvVars("https://abc123.live.dynatrace.com", "tok", "svc")
	if got := envVars["OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED"]; got != "true" {
		t.Errorf("OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED = %q, want %q", got, "true")
	}
}

// ── serviceNameFromEntrypoint ─────────────────────────────────────────────────

func TestServiceNameFromEntrypoint(t *testing.T) {
	tests := []struct {
		projectPath string
		entrypoint  string
		want        string
	}{
		// Entrypoint in project root → use project name only.
		{"/home/user/orderschnitzel", "app.py", "orderschnitzel"},
		{"/home/user/orderschnitzel", "main.py", "orderschnitzel"},
		// Entrypoint one level deep → projectName-dirName.
		{"/home/user/orderschnitzel", "s-frontend/app.py", "orderschnitzel-s-frontend"},
		{"/home/user/orderschnitzel", "s-order/main.py", "orderschnitzel-s-order"},
		// Entrypoint two levels deep → projectName-immediateParent.
		{"/home/user/myapp", "services/api/main.py", "myapp-api"},
	}
	for _, tt := range tests {
		got := serviceNameFromEntrypoint(tt.projectPath, tt.entrypoint)
		if got != tt.want {
			t.Errorf("serviceNameFromEntrypoint(%q, %q) = %q, want %q", tt.projectPath, tt.entrypoint, got, tt.want)
		}
	}
}

// ── parseEntrypointFromPyproject ──────────────────────────────────────────────

func TestParseEntrypointFromPyproject_ProjectScripts(t *testing.T) {
	content := `
[build-system]
requires = ["setuptools"]

[project]
name = "myapp"

[project.scripts]
myapp = "myapp.main:run"
`
	got := parseEntrypointFromPyproject(content)
	if got != "myapp/main.py" {
		t.Errorf("got %q, want %q", got, "myapp/main.py")
	}
}

func TestParseEntrypointFromPyproject_PoetryScripts(t *testing.T) {
	content := `
[tool.poetry]
name = "svc"

[tool.poetry.scripts]
svc = "svc.server:main"
`
	got := parseEntrypointFromPyproject(content)
	if got != "svc/server.py" {
		t.Errorf("got %q, want %q", got, "svc/server.py")
	}
}

func TestParseEntrypointFromPyproject_SingleModuleScript(t *testing.T) {
	content := `
[project.scripts]
app = "app:main"
`
	got := parseEntrypointFromPyproject(content)
	if got != "app.py" {
		t.Errorf("got %q, want %q", got, "app.py")
	}
}

func TestParseEntrypointFromPyproject_NoScripts(t *testing.T) {
	content := `
[project]
name = "notool"
version = "0.1.0"
`
	got := parseEntrypointFromPyproject(content)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestParseEntrypointFromPyproject_EmptyContent(t *testing.T) {
	got := parseEntrypointFromPyproject("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ── detectPythonEntrypoints ───────────────────────────────────────────────────

func TestDetectPythonEntrypoints_CommonFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectPythonEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "app.py" {
		t.Errorf("expected [app.py], got %v", eps)
	}
}

func TestDetectPythonEntrypoints_PriorityOrder(t *testing.T) {
	// main.py should win over app.py when both exist.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectPythonEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "main.py" {
		t.Errorf("expected main.py first, got %v", eps)
	}
}

func TestDetectPythonEntrypoints_PyprojectWins(t *testing.T) {
	dir := t.TempDir()
	pyproject := "[project.scripts]\napp = \"myapp.server:run\"\n"
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatal(err)
	}
	// Also create app.py — pyproject.toml should still take priority.
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectPythonEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "myapp/server.py" {
		t.Errorf("expected pyproject-derived entrypoint, got %v", eps)
	}
}

func TestDetectPythonEntrypoints_SubDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "api")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "app.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectPythonEntrypoints(dir)
	want := filepath.Join("api", "app.py")
	if len(eps) == 0 || eps[0] != want {
		t.Errorf("expected [%s], got %v", want, eps)
	}
}

func TestDetectPythonEntrypoints_SkipsHiddenAndPycache(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{".hidden", "__pycache__", "node_modules"} {
		subDir := filepath.Join(dir, sub)
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "app.py"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	eps := detectPythonEntrypoints(dir)
	for _, ep := range eps {
		if strings.Contains(ep, ".hidden") || strings.Contains(ep, "__pycache__") || strings.Contains(ep, "node_modules") {
			t.Errorf("entrypoint from excluded dir found: %s", ep)
		}
	}
}

func TestDetectPythonEntrypoints_None(t *testing.T) {
	dir := t.TempDir()
	eps := detectPythonEntrypoints(dir)
	if len(eps) != 0 {
		t.Errorf("expected no entrypoints, got %v", eps)
	}
}

// ── detectPythonProjects ──────────────────────────────────────────────────────

func TestDetectPythonProjects_Found(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask\n"), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	projects := detectPythonProjects()
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Python project at %s, got %v", dir, projects)
	}
}

func TestDetectPythonProjects_AllMarkers(t *testing.T) {
	markers := []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile", "poetry.lock", "manage.py"}
	for _, marker := range markers {
		t.Run(marker, func(t *testing.T) {
			dir := t.TempDir()
			realDir, _ := filepath.EvalSymlinks(dir)
			if err := os.WriteFile(filepath.Join(dir, marker), []byte(""), 0644); err != nil {
				t.Fatal(err)
			}

			setTestWorkingDir(t, dir)
			projects := detectPythonProjects()
			found := false
			for _, p := range projects {
				if p.Path == dir || p.Path == realDir {
					found = true
				}
			}
			if !found {
				t.Errorf("marker %q: expected project at %s, got %v", marker, dir, projects)
			}
		})
	}
}

// ── resolveVenvBinary ─────────────────────────────────────────────────────────

func TestResolveVenvBinary_FindsInVenv(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, ".venv", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "python")
	if err := os.WriteFile(binPath, []byte(""), 0700); err != nil {
		t.Fatal(err)
	}

	got := resolveVenvBinary(dir, "python")
	if got != binPath {
		t.Errorf("resolveVenvBinary = %q, want %q", got, binPath)
	}
}

func TestResolveVenvBinary_FallsBackToName(t *testing.T) {
	dir := t.TempDir()
	got := resolveVenvBinary(dir, "python3")
	if got != "python3" {
		t.Errorf("resolveVenvBinary = %q, want %q", got, "python3")
	}
}

func TestResolveVenvBinary_ChecksAllVenvNames(t *testing.T) {
	for _, venvName := range []string{".venv", "venv", "env", ".env"} {
		t.Run(venvName, func(t *testing.T) {
			dir := t.TempDir()
			binDir := filepath.Join(dir, venvName, "bin")
			if err := os.MkdirAll(binDir, 0755); err != nil {
				t.Fatal(err)
			}
			binPath := filepath.Join(binDir, "pip")
			if err := os.WriteFile(binPath, []byte(""), 0700); err != nil {
				t.Fatal(err)
			}

			got := resolveVenvBinary(dir, "pip")
			if got != binPath {
				t.Errorf("venv=%q: resolveVenvBinary = %q, want %q", venvName, got, binPath)
			}
		})
	}
}

// ── detectProjectPip ─────────────────────────────────────────────────────────

func TestDetectProjectPip_Found(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, ".venv", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	pipPath := filepath.Join(binDir, "pip")
	if err := os.WriteFile(pipPath, []byte(""), 0700); err != nil {
		t.Fatal(err)
	}

	pip := detectProjectPip(dir)
	if pip == nil {
		t.Fatal("expected pip to be found, got nil")
	}
	if pip.name != pipPath {
		t.Errorf("pip.name = %q, want %q", pip.name, pipPath)
	}
}

func TestDetectProjectPip_NotFound(t *testing.T) {
	dir := t.TempDir()
	pip := detectProjectPip(dir)
	if pip != nil {
		t.Errorf("expected nil when no venv exists, got %+v", pip)
	}
}

func TestDetectProjectPip_ChecksAllVenvNames(t *testing.T) {
	for _, venvName := range []string{".venv", "venv", "env", ".env"} {
		t.Run(venvName, func(t *testing.T) {
			dir := t.TempDir()
			binDir := filepath.Join(dir, venvName, "bin")
			if err := os.MkdirAll(binDir, 0755); err != nil {
				t.Fatal(err)
			}
			pipPath := filepath.Join(binDir, "pip")
			if err := os.WriteFile(pipPath, []byte(""), 0700); err != nil {
				t.Fatal(err)
			}

			pip := detectProjectPip(dir)
			if pip == nil {
				t.Fatalf("venv=%q: expected pip to be found, got nil", venvName)
			}
			if pip.name != pipPath {
				t.Errorf("venv=%q: pip.name = %q, want %q", venvName, pip.name, pipPath)
			}
		})
	}
}

func TestDetectPython_NotFound(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := detectPython()
	if err == nil {
		t.Fatal("expected error when python is not on PATH")
	}
}

func TestBuildPythonInstrumentationPlan(t *testing.T) {
	apiURL := "https://tenant.live.dynatrace.com"
	token := "token"
	envURL := "https://tenant.apps.dynatrace.com"
	platformToken := "platform-token"

	t.Run("returns plan when entrypoint exists", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('ok')\n"), 0644); err != nil {
			t.Fatal(err)
		}

		plan := buildPythonInstrumentationPlan(ScannedProject{Path: dir}, apiURL, token, envURL, platformToken)
		if plan == nil {
			t.Fatal("expected non-nil plan")
		}
		if len(plan.Entrypoints) != 1 || plan.Entrypoints[0] != "main.py" {
			t.Fatalf("unexpected entrypoints: %v", plan.Entrypoints)
		}
		if plan.EnvURL != envURL || plan.PlatformToken != platformToken {
			t.Fatalf("unexpected env metadata: %+v", plan)
		}
	})

	t.Run("returns nil when entrypoint is missing", func(t *testing.T) {
		plan := buildPythonInstrumentationPlan(ScannedProject{Path: t.TempDir()}, apiURL, token, envURL, platformToken)
		if plan != nil {
			t.Fatalf("expected nil plan, got %#v", plan)
		}
	})
}

func TestDetectPythonPlan_NoPythonOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	plan := DetectPythonPlan("https://tenant.live.dynatrace.com", "token")
	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}
}

func TestDetectPythonPlan_FindsProject(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		if _, err := exec.LookPath("python"); err != nil {
			t.Skip("python not installed on PATH")
		}
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('ok')\n"), 0644); err != nil {
		t.Fatal(err)
	}

	setTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")

	plan := DetectPythonPlan("https://tenant.live.dynatrace.com", "token")
	if plan == nil {
		t.Fatal("expected Python plan")
	}
	if len(plan.Entrypoints) != 1 || plan.Entrypoints[0] != "main.py" {
		t.Fatalf("unexpected entrypoints: %v", plan.Entrypoints)
	}
}

func TestPrintManualInstructions(t *testing.T) {
	output := captureStdout(t, func() {
		printManualInstructions(map[string]string{"OTEL_SERVICE_NAME": "py-svc"})
	})

	checks := []string{"pip install opentelemetry-distro opentelemetry-exporter-otlp", "opentelemetry-bootstrap -a install", "export OTEL_SERVICE_NAME=\"py-svc\"", "opentelemetry-instrument python your_app.py"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestPythonInstrumentationPlan_Runtime(t *testing.T) {
	plan := &PythonInstrumentationPlan{}
	if got := plan.Runtime(); got != "Python" {
		t.Fatalf("Runtime() = %q, want %q", got, "Python")
	}
}

func TestPythonInstrumentationPlan_PrintPlanSteps(t *testing.T) {
	plan := &PythonInstrumentationPlan{
		Project:     ScannedProject{Path: "/tmp/orderschnitzel", RunningProcessIDs: []int{111, 222}},
		Entrypoints: []string{"main.py", filepath.Join("api", "app.py")},
		NeedsVenv:   true,
	}

	output := captureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{"Project: /tmp/orderschnitzel", "Stop running processes (PIDs: 111, 222)", "Create virtualenv (.venv)", "pip install opentelemetry-distro opentelemetry-exporter-otlp", "opentelemetry-instrument python main.py", "service: orderschnitzel-api"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func TestPythonInstrumentationPlan_ExecuteFailsWithoutPythonForVenvCreation(t *testing.T) {
	t.Setenv("PATH", "")
	plan := &PythonInstrumentationPlan{
		Project:   ScannedProject{Path: t.TempDir()},
		NeedsVenv: true,
		EnvVars:   map[string]string{"OTEL_SERVICE_NAME": "py-svc"},
	}

	output := captureStdout(t, func() {
		plan.Execute()
	})

	checks := []string{"Creating virtualenv... failed.", "Python 3 not found"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}
func TestValidatePythonPrerequisites_PythonNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := validatePythonPrerequisites()
	if err == nil || !strings.Contains(err.Error(), "Python 3") {
		t.Fatalf("expected Python 3 error, got %v", err)
	}
}

func TestValidatePythonPrerequisites_PipNotFound(t *testing.T) {
	pythonDir := requireFakePython3(t)
	t.Setenv("PATH", pythonDir)
	t.Setenv("DTWIZ_TEST_FAIL_PIP", "1")

	err := validatePythonPrerequisites()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "pip") {
		t.Fatalf("expected pip error, got %v", err)
	}
}

func TestValidatePythonPrerequisites_VenvNotFound(t *testing.T) {
	pythonDir := requireFakePython3(t)
	t.Setenv("PATH", pythonDir)
	t.Setenv("DTWIZ_TEST_FAIL_VENV", "1")

	err := validatePythonPrerequisites()
	if err == nil {
		t.Fatal("expected venv error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "venv") {
		t.Fatalf("expected venv error, got %v", err)
	}
	if !strings.Contains(err.Error(), "apt install python3-venv") {
		t.Fatalf("expected install suggestion, got %v", err)
	}
}

func TestValidatePythonPrerequisites_AllPresent(t *testing.T) {
	pythonDir := requireFakePython3(t)
	t.Setenv("PATH", pythonDir)

	if err := validatePythonPrerequisites(); err != nil {
		t.Fatalf("validatePythonPrerequisites() error = %v", err)
	}
}

func TestInstallPackages_ErrorIncludesCommand(t *testing.T) {
	pip := &pipCommand{
		name: filepath.Join(t.TempDir(), "missing-python"),
		args: []string{"-m", "pip"},
	}

	err := installPackages(pip, []string{"opentelemetry-distro"})
	if err == nil || !strings.Contains(err.Error(), "command:") {
		t.Fatalf("expected command in error, got %v", err)
	}
}

func TestRunOtelBootstrap_ErrorIncludesCommand(t *testing.T) {
	err := runOtelBootstrap(filepath.Join(t.TempDir(), "missing-python"))
	if err == nil || !strings.Contains(err.Error(), "command:") {
		t.Fatalf("expected command in error, got %v", err)
	}
}

func TestGenerateOtelPythonEnvVars(t *testing.T) {
	vars := generateOtelPythonEnvVars("https://abc123.live.dynatrace.com", "dt0c01.test", "my-svc")
	if vars["OTEL_SERVICE_NAME"] != "my-svc" {
		t.Fatalf("OTEL_SERVICE_NAME = %q, want %q", vars["OTEL_SERVICE_NAME"], "my-svc")
	}
	if !strings.Contains(vars["OTEL_EXPORTER_OTLP_ENDPOINT"], "/api/v2/otlp") {
		t.Fatalf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want to contain /api/v2/otlp", vars["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
	if vars["OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE"] != "delta" {
		t.Fatalf("temporality = %q, want delta", vars["OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE"])
	}
}

func TestGenerateEnvExportScript(t *testing.T) {
	script := GenerateEnvExportScript(map[string]string{"FOO": "bar"})
	if !strings.Contains(script, "export FOO=") {
		t.Fatalf("script = %q, want to contain export FOO=", script)
	}
}

func TestEnvVarsToSlice(t *testing.T) {
	slice := envVarsToSlice(map[string]string{"A": "1", "B": "2"})
	if len(slice) != 2 {
		t.Fatalf("len = %d, want 2", len(slice))
	}
	joined := strings.Join(slice, ",")
	if !strings.Contains(joined, "A=1") || !strings.Contains(joined, "B=2") {
		t.Fatalf("slice = %v, want A=1 and B=2", slice)
	}
}

// --- Shared test helpers (used across otel_python_*_test.go files) ---

func requireFakePython3(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell helper for python prerequisite tests is only used on Unix-like platforms")
	}
	dir := t.TempDir()
	createStubFile(t, filepath.Join(dir, "python3"), `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "Python 3.12.0"
  exit 0
fi
if [ "$1" = "-m" ] && [ "$2" = "pip" ] && [ "$3" = "--version" ]; then
  if [ "${DTWIZ_TEST_FAIL_PIP:-0}" = "1" ]; then
    echo "pip unavailable" >&2
    exit 1
  fi
  echo "pip 24.0 from /tmp/site-packages/pip"
  exit 0
fi
if [ "$1" = "-m" ] && [ "$2" = "venv" ] && [ "$3" = "--help" ]; then
  if [ "${DTWIZ_TEST_FAIL_VENV:-0}" = "1" ]; then
    echo "venv unavailable" >&2
    exit 1
  fi
  echo "usage: venv"
  exit 0
fi
echo "unexpected args: $@" >&2
exit 1
`, 0o755)
	return dir
}

func createStubVenvPython(t *testing.T, projectDir, venvName, pythonName string, executable bool) string {
	t.Helper()
	binDir := filepath.Join(projectDir, venvName, "bin")
	if runtime.GOOS == "windows" {
		binDir = filepath.Join(projectDir, venvName, "Scripts")
		if !strings.HasSuffix(pythonName, ".exe") {
			pythonName += ".exe"
		}
	}
	mode := os.FileMode(0o644)
	content := "stub"
	if executable {
		mode = 0o755
		if runtime.GOOS == "windows" {
			content = "MZ"
		} else {
			content = "#!/bin/sh\necho Python 3.12.0\n"
		}
	}
	path := filepath.Join(binDir, pythonName)
	createStubFile(t, path, content, mode)
	return path
}

func createStubFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func withStdinText(t *testing.T, input string, fn func()) {
	t.Helper()
	stdinFile, err := os.CreateTemp(t.TempDir(), "stdin-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if _, err := stdinFile.WriteString(input); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if _, err := stdinFile.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error = %v", err)
	}

	originalStdin := os.Stdin
	os.Stdin = stdinFile
	defer func() {
		os.Stdin = originalStdin
		_ = stdinFile.Close()
	}()

	fn()
}
