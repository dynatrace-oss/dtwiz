package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServiceNameFromEntrypoint_RootFile(t *testing.T) {
	got := serviceNameFromEntrypoint("/home/user/orderschnitzel", "app.py")
	if got != "orderschnitzel" {
		t.Fatalf("serviceNameFromEntrypoint() = %q, want %q", got, "orderschnitzel")
	}
}

func TestServiceNameFromEntrypoint_SubDir(t *testing.T) {
	got := serviceNameFromEntrypoint("/home/user/orderschnitzel", "s-frontend/app.py")
	if got != "orderschnitzel-s-frontend" {
		t.Fatalf("serviceNameFromEntrypoint() = %q, want %q", got, "orderschnitzel-s-frontend")
	}
}

func TestServiceNameFromEntrypoint_DeepSubDir(t *testing.T) {
	got := serviceNameFromEntrypoint("/home/user/myapp", "services/api/main.py")
	if got != "myapp-api" {
		t.Fatalf("serviceNameFromEntrypoint() = %q, want %q", got, "myapp-api")
	}
}

func TestDetectPythonEntrypoints_CommonFile(t *testing.T) {
	projectDir := t.TempDir()
	createStubFile(t, filepath.Join(projectDir, "app.py"), "# app\n", 0o644)

	eps := detectPythonEntrypoints(projectDir)
	if len(eps) != 1 || eps[0] != "app.py" {
		t.Fatalf("detectPythonEntrypoints() = %v, want [app.py]", eps)
	}
}

func TestDetectPythonEntrypoints_SubdirEntrypoints(t *testing.T) {
	projectDir := t.TempDir()
	createStubFile(t, filepath.Join(projectDir, "svc-a", "main.py"), "# main\n", 0o644)
	createStubFile(t, filepath.Join(projectDir, "svc-b", "app.py"), "# app\n", 0o644)

	eps := detectPythonEntrypoints(projectDir)
	if len(eps) != 2 {
		t.Fatalf("detectPythonEntrypoints() = %v, want 2 entries", eps)
	}
}

func TestDetectPythonEntrypoints_Pyproject(t *testing.T) {
	projectDir := t.TempDir()
	content := `[project]
name = "myapp"

[project.scripts]
myapp = "myapp.main:run"
`
	createStubFile(t, filepath.Join(projectDir, "pyproject.toml"), content, 0o644)

	eps := detectPythonEntrypoints(projectDir)
	if len(eps) != 1 || eps[0] != "myapp/main.py" {
		t.Fatalf("detectPythonEntrypoints() = %v, want [myapp/main.py]", eps)
	}
}

func TestDetectPythonEntrypoints_None(t *testing.T) {
	eps := detectPythonEntrypoints(t.TempDir())
	if len(eps) != 0 {
		t.Fatalf("detectPythonEntrypoints() = %v, want empty", eps)
	}
}

func TestParseEntrypointFromPyproject_ProjectScripts(t *testing.T) {
	content := `[project.scripts]
cli = "myapp.cli:main"
`
	got := parseEntrypointFromPyproject(content)
	if got != "myapp/cli.py" {
		t.Fatalf("parseEntrypointFromPyproject() = %q, want %q", got, "myapp/cli.py")
	}
}

func TestParseEntrypointFromPyproject_PoetryScripts(t *testing.T) {
	content := `[tool.poetry.scripts]
serve = "myapp.server:run"
`
	got := parseEntrypointFromPyproject(content)
	if got != "myapp/server.py" {
		t.Fatalf("parseEntrypointFromPyproject() = %q, want %q", got, "myapp/server.py")
	}
}

func TestParseEntrypointFromPyproject_NoScripts(t *testing.T) {
	content := `[project]
name = "myapp"
`
	got := parseEntrypointFromPyproject(content)
	if got != "" {
		t.Fatalf("parseEntrypointFromPyproject() = %q, want empty", got)
	}
}

func TestMatchProcessesToProjects(t *testing.T) {
	projects := []PythonProject{
		{Path: "/home/user/myapp"},
	}
	procs := []PythonProcess{
		{PID: 100, Command: "python /home/user/myapp/app.py", CWD: "/home/user/myapp"},
		{PID: 200, Command: "python /other/app.py", CWD: "/other"},
	}

	matchProcessesToProjects(projects, procs)

	if len(projects[0].RunningPIDs) != 1 || projects[0].RunningPIDs[0] != 100 {
		t.Fatalf("RunningPIDs = %v, want [100]", projects[0].RunningPIDs)
	}
}

func TestProjectDepsDescription(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		expected string
	}{
		{"requirements.txt", "requirements.txt", "pip install -r requirements.txt"},
		{"pyproject.toml", "pyproject.toml", "pip install . (pyproject.toml)"},
		{"setup.py", "setup.py", "pip install . (setup.py)"},
		{"none", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.file != "" {
				createStubFile(t, filepath.Join(dir, tt.file), "# deps\n", 0o644)
			}
			got := projectDepsDescription(dir)
			if got != tt.expected {
				t.Fatalf("projectDepsDescription() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestInstallProjectDeps_NoDepFile(t *testing.T) {
	pip := &pipCommand{name: "python", args: []string{"-m", "pip"}}
	desc, err := installProjectDeps(pip, t.TempDir())
	if err != nil {
		t.Fatalf("installProjectDeps() error = %v", err)
	}
	if desc != "" {
		t.Fatalf("installProjectDeps() = %q, want empty", desc)
	}
}

func TestInstallProjectDeps_ErrorIncludesCommand(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "requirements.txt"), []byte("flask\n"), 0o644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}
	pip := &pipCommand{
		name: filepath.Join(t.TempDir(), "missing-python"),
		args: []string{"-m", "pip"},
	}

	_, err := installProjectDeps(pip, projectDir)
	if err == nil || !containsAll(err.Error(), "command:", "requirements.txt") {
		t.Fatalf("expected error with command and label, got %v", err)
	}
}

func TestNormalizePipName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Flask", "flask"},
		{"my_package", "my-package"},
		{"my.package", "my-package"},
		{"My_Package.Name", "my-package-name"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizePipName(tt.input)
			if got != tt.expected {
				t.Fatalf("normalizePipName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
