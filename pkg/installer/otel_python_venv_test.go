package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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

func TestDetectProjectPip_ReturnsPythonMPip(t *testing.T) {
	projectDir := t.TempDir()
	pythonPath := createStubVenvPython(t, projectDir, ".venv", "python", true)

	pip := detectProjectPip(projectDir)
	if pip == nil {
		t.Fatal("expected pip command, got nil")
	}
	if pip.name != pythonPath {
		t.Fatalf("pip.name = %q, want %q", pip.name, pythonPath)
	}
	if len(pip.args) < 2 || pip.args[0] != "-m" || pip.args[1] != "pip" {
		t.Fatalf("pip.args = %v, want prefix [-m pip]", pip.args)
	}
}

func TestDetectProjectPip_NoPipScriptFallback(t *testing.T) {
	projectDir := t.TempDir()
	pythonPath := createStubVenvPython(t, projectDir, ".venv", "python3", true)
	createStubFile(t, filepath.Join(projectDir, ".venv", "bin", "pip3"), "#!/bin/sh\nexit 0\n", 0o755)

	pip := detectProjectPip(projectDir)
	if pip == nil {
		t.Fatal("expected pip command, got nil")
	}
	if pip.name != pythonPath {
		t.Fatalf("pip.name = %q, want %q", pip.name, pythonPath)
	}
	if strings.HasSuffix(pip.name, "pip3") {
		t.Fatalf("detectProjectPip returned pip script instead of python binary: %q", pip.name)
	}
	if len(pip.args) < 2 || pip.args[0] != "-m" || pip.args[1] != "pip" {
		t.Fatalf("pip.args = %v, want prefix [-m pip]", pip.args)
	}
}

func TestDetectProjectPip_NoVenv(t *testing.T) {
	pip := detectProjectPip(t.TempDir())
	if pip != nil {
		t.Fatalf("expected nil for project without venv, got %+v", pip)
	}
}

func TestIsVenvHealthy_NoVenv(t *testing.T) {
	if isVenvHealthy(t.TempDir()) {
		t.Fatal("expected no venv to be unhealthy")
	}
}

func TestIsVenvHealthy_BrokenPython(t *testing.T) {
	projectDir := t.TempDir()
	createStubVenvPython(t, projectDir, ".venv", "python", false)

	if isVenvHealthy(projectDir) {
		t.Fatal("expected broken venv python to be unhealthy")
	}
}

func TestIsVenvHealthy_WorkingPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		// A stub .exe containing only "MZ" is not a valid PE executable and cannot
		// be launched by the OS, so isVenvHealthy (which runs python --version)
		// always returns false for stub binaries on Windows.
		t.Skip("stub .exe cannot be executed on Windows")
	}
	projectDir := t.TempDir()
	createStubVenvPython(t, projectDir, ".venv", "python", true)

	if !isVenvHealthy(projectDir) {
		t.Fatal("expected working venv python to be healthy")
	}
}

func TestDetectProjectVenvDir_Found(t *testing.T) {
	projectDir := t.TempDir()
	venvDir := filepath.Join(projectDir, ".venv")
	if err := os.MkdirAll(venvDir, 0o755); err != nil {
		t.Fatal(err)
	}
	got := detectProjectVenvDir(projectDir)
	if got != venvDir {
		t.Fatalf("detectProjectVenvDir() = %q, want %q", got, venvDir)
	}
}

func TestDetectProjectVenvDir_NotFound(t *testing.T) {
	got := detectProjectVenvDir(t.TempDir())
	if got != "" {
		t.Fatalf("detectProjectVenvDir() = %q, want empty", got)
	}
}

func TestResolveVenvBinary_Found(t *testing.T) {
	projectDir := t.TempDir()
	binPath := filepath.Join(projectDir, ".venv", "bin", "mybin")
	createStubFile(t, binPath, "#!/bin/sh\n", 0o755)

	got := resolveVenvBinary(projectDir, "mybin")
	if got != binPath {
		t.Fatalf("resolveVenvBinary() = %q, want %q", got, binPath)
	}
}

func TestResolveVenvBinary_Fallback(t *testing.T) {
	got := resolveVenvBinary(t.TempDir(), "mybin")
	if got != "mybin" {
		t.Fatalf("resolveVenvBinary() = %q, want %q", got, "mybin")
	}
}

func TestRemoveStaleVirtualenv_UserDeclines(t *testing.T) {
	venvDir := filepath.Join(t.TempDir(), ".venv")
	if err := os.MkdirAll(venvDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	output := captureStdout(t, func() {
		withStdinText(t, "n\n", func() {
			removed, err := removeStaleVirtualenv(venvDir)
			if err != nil {
				t.Fatalf("removeStaleVirtualenv() error = %v", err)
			}
			if removed {
				t.Fatal("expected stale venv deletion to be cancelled")
			}
		})
	})

	if _, err := os.Stat(venvDir); err != nil {
		t.Fatalf("expected venv directory to remain, got %v", err)
	}
	if !strings.Contains(output, "working virtualenv is required") || !strings.Contains(output, "OTLP ingest") {
		t.Fatalf("expected confirmation prompt to explain why recreation is needed, got %q", output)
	}
}

func TestRemoveStaleVirtualenv_UserConfirms(t *testing.T) {
	venvDir := filepath.Join(t.TempDir(), ".venv")
	if err := os.MkdirAll(venvDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	withStdinText(t, "y\n", func() {
		removed, err := removeStaleVirtualenv(venvDir)
		if err != nil {
			t.Fatalf("removeStaleVirtualenv() error = %v", err)
		}
		if !removed {
			t.Fatal("expected stale venv deletion to be confirmed")
		}
	})

	if _, err := os.Stat(venvDir); !os.IsNotExist(err) {
		t.Fatalf("expected venv directory to be removed, got %v", err)
	}
}

func TestDetectPython_PrefersPython3(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based python stubs only work on Unix")
	}
	dir := t.TempDir()
	// Create python that reports Python 2 and python3 that reports Python 3
	createStubFile(t, filepath.Join(dir, "python"), "#!/bin/sh\necho Python 2.7.18\n", 0o755)
	createStubFile(t, filepath.Join(dir, "python3"), "#!/bin/sh\necho Python 3.12.0\n", 0o755)
	t.Setenv("PATH", dir)

	got, err := detectPython()
	if err != nil {
		t.Fatalf("detectPython() error = %v", err)
	}
	if !strings.HasSuffix(got, "python3") {
		t.Fatalf("detectPython() = %q, want python3", got)
	}
}

func TestDetectPython_NoPython3Available(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs only work on Unix")
	}
	dir := t.TempDir()
	// only a Python 2 interpreter available
	createStubFile(t, filepath.Join(dir, "python"), "#!/bin/sh\necho Python 2.7.18\n", 0o755)
	t.Setenv("PATH", dir)

	_, err := detectPython()
	if err == nil || !strings.Contains(err.Error(), "Python 3") {
		t.Fatalf("detectPython() error = %v, want Python 3 not found error", err)
	}
}

func TestDetectPython_FallbackToPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs only work on Unix")
	}
	dir := t.TempDir()
	// No python3, only python which happens to be Python 3
	createStubFile(t, filepath.Join(dir, "python"), "#!/bin/sh\necho Python 3.10.0\n", 0o755)
	t.Setenv("PATH", dir)

	got, err := detectPython()
	if err != nil {
		t.Fatalf("detectPython() error = %v", err)
	}
	if !strings.HasSuffix(got, "python") {
		t.Fatalf("detectPython() = %q, want path ending in python", got)
	}
}

func TestDetectProjectVenvDir_AlternativeVenvNames(t *testing.T) {
	for _, venvName := range []string{"venv", "env", ".env"} {
		t.Run(venvName, func(t *testing.T) {
			dir := t.TempDir()
			venvDir := filepath.Join(dir, venvName)
			if err := os.MkdirAll(venvDir, 0o755); err != nil {
				t.Fatal(err)
			}
			got := detectProjectVenvDir(dir)
			if got != venvDir {
				t.Fatalf("detectProjectVenvDir() = %q, want %q", got, venvDir)
			}
		})
	}
}

func TestDetectProjectVenvDir_PrefersFirst(t *testing.T) {
	dir := t.TempDir()
	// Both .venv and venv exist — .venv is checked first and should win.
	for _, name := range []string{".venv", "venv"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := detectProjectVenvDir(dir)
	expected := filepath.Join(dir, ".venv")
	if got != expected {
		t.Fatalf("detectProjectVenvDir() = %q, want %q (should prefer .venv)", got, expected)
	}
}

func TestResolveVenvBinary_AlternativeVenvNames(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only bin/ layout test")
	}
	for _, venvName := range []string{"venv", "env", ".env"} {
		t.Run(venvName, func(t *testing.T) {
			dir := t.TempDir()
			binPath := filepath.Join(dir, venvName, "bin", "mybin")
			createStubFile(t, binPath, "#!/bin/sh\n", 0o755)
			got := resolveVenvBinary(dir, "mybin")
			if got != binPath {
				t.Fatalf("resolveVenvBinary() = %q, want %q", got, binPath)
			}
		})
	}
}
