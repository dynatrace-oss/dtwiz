package installer

import (
	"os"
	"path/filepath"
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

	withCWD(t, dir)
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

			withCWD(t, dir)
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
