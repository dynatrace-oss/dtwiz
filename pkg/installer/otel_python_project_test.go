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

func TestMatchProcessesToProjects_PythonCWDMatch(t *testing.T) {
	projects := []ScannedProject{
		{Path: "/home/user/myapp"},
	}
	procs := []DetectedProcess{
		{PID: 100, Command: "python /home/user/myapp/app.py", WorkingDirectory: "/home/user/myapp"},
		{PID: 200, Command: "python /other/app.py", WorkingDirectory: "/other"},
	}

	matchProcessesToProjects(projects, procs)

	if len(projects[0].RunningProcessIDs) != 1 || projects[0].RunningProcessIDs[0] != 100 {
		t.Fatalf("RunningProcessIDs = %v, want [100]", projects[0].RunningProcessIDs)
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

func TestDetectPythonProjects_FindsCWD(t *testing.T) {
	dir := t.TempDir()
	createStubFile(t, filepath.Join(dir, "requirements.txt"), "flask\n", 0o644)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// EvalSymlinks resolves macOS /var → /private/var; match against the canonical path.
	// On Windows, os.Getwd() may return a short path (RUNNER~1) while t.TempDir() returns
	// the long path, so compare via os.SameFile rather than string equality.
	expected, _ := filepath.EvalSymlinks(dir)
	expectedInfo, err := os.Lstat(expected)
	if err != nil {
		t.Fatalf("Lstat expected path %q: %v", expected, err)
	}
	projects := detectPythonProjects()
	found := false
	for _, p := range projects {
		info, err := os.Lstat(p.Path)
		if err == nil && os.SameFile(expectedInfo, info) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("detectPythonProjects() did not find project at %q, got %v", expected, projects)
	}
}

func TestDetectPythonProjects_FindsSubDir(t *testing.T) {
	root := t.TempDir()
	subDir := filepath.Join(root, "myservice")
	createStubFile(t, filepath.Join(subDir, "pyproject.toml"), "[project]\nname=\"svc\"\n", 0o644)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	expectedSub, _ := filepath.EvalSymlinks(subDir)
	expectedSubInfo, err := os.Lstat(expectedSub)
	if err != nil {
		t.Fatalf("Lstat expected subdir path %q: %v", expectedSub, err)
	}
	projects := detectPythonProjects()
	found := false
	for _, p := range projects {
		info, err := os.Lstat(p.Path)
		if err == nil && os.SameFile(expectedSubInfo, info) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("detectPythonProjects() did not find subdir project at %q, got %v", expectedSub, projects)
	}
}

func TestMatchProcessesToProjects_CommandPathMatch(t *testing.T) {
	projects := []ScannedProject{
		{Path: "/home/user/myapp"},
	}
	procs := []DetectedProcess{
		// process has a different CWD but its command references the project path
		{PID: 300, Command: "python /home/user/myapp/server.py", WorkingDirectory: "/tmp"},
	}
	matchProcessesToProjects(projects, procs)
	if len(projects[0].RunningProcessIDs) != 1 || projects[0].RunningProcessIDs[0] != 300 {
		t.Fatalf("RunningProcessIDs = %v, want [300]", projects[0].RunningProcessIDs)
	}
}

func TestMatchProcessesToProjects_UnrelatedProcess(t *testing.T) {
	projects := []ScannedProject{
		{Path: "/home/user/myapp"},
	}
	procs := []DetectedProcess{
		{PID: 500, Command: "python /other/project/app.py", WorkingDirectory: "/other/project"},
	}
	matchProcessesToProjects(projects, procs)
	if len(projects[0].RunningProcessIDs) != 0 {
		t.Fatalf("RunningProcessIDs = %v, want empty", projects[0].RunningProcessIDs)
	}
}
