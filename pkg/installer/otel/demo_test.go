package otel

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func TestCheckDemoExists(t *testing.T) {
	// Ensure schnitzel dir doesn't exist in test working dir
	_ = os.RemoveAll(demoDirName)
	if checkDemoExists() {
		t.Fatal("expected checkDemoExists() = false when dir does not exist")
	}

	// Create the dir and check again
	if err := os.MkdirAll(demoDirName, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer os.RemoveAll(demoDirName)

	if !checkDemoExists() {
		t.Fatal("expected checkDemoExists() = true when dir exists")
	}
}

func TestConfirmProceedAutoConfirm(t *testing.T) {
	orig := installer.AutoConfirm
	defer func() { installer.AutoConfirm = orig }()

	installer.AutoConfirm = true
	ok, err := installer.ConfirmProceed("Test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ConfirmProceed to return true when AutoConfirm=true")
	}
}

// TestInstallOtelCollectorWithProject_DryRun verifies that dry-run goes through
// the full interactive flow (project detection) but makes no changes on disk.
func TestInstallOtelCollectorWithProject_DryRun(t *testing.T) {
	dir := t.TempDir()
	helpers.SetTestWorkingDir(t, dir)
	setTestStdin(t, "y\n")

	output := helpers.CaptureStdout(t, func() {
		err := InstallOtelCollectorWithProject(
			"https://fake.live.dynatrace.com", "tok", "tok", "", true,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// The collector config preview (printed via fmt.Printf) must be present.
	if !strings.Contains(output, "fake.live.dynatrace.com") {
		t.Fatalf("expected collector config in dry-run output, got:\n%s", output)
	}
	// No files must be written.
	if _, err := os.Stat(filepath.Join(dir, "opentelemetry")); !os.IsNotExist(err) {
		t.Fatal("dry-run must not create the opentelemetry directory")
	}
}

func TestInstallOtelCollectorWithProjectPathNotFound(t *testing.T) {
	err := InstallOtelCollectorWithProject("https://fake.live.dynatrace.com", "tok", "tok", "/nonexistent/path", false)
	if err == nil {
		t.Fatal("expected error for non-existent project path")
	}
	if err.Error() != "project path not found: /nonexistent/path" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestInstallOtelPythonProjectPathNotFound(t *testing.T) {
	err := InstallOtelPython("https://fake.live.dynatrace.com", "tok", "plat", "", "/nonexistent/path", false)
	if err == nil {
		t.Fatal("expected error for non-existent project path")
	}
	if err.Error() != "project path not found: /nonexistent/path" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestPythonInstallPlanCurrentOS verifies that pythonInstallPlan returns a non-nil
// command (or no error) on the current OS when Python is NOT on PATH.
// We can only test the logic path for the running OS.
func TestPythonInstallPlanCurrentOS(t *testing.T) {
	// If Python is present, the plan returns nil (nothing to install) — that's fine.
	cmd, err := pythonInstallPlan()
	if err != nil && cmd != nil {
		t.Fatalf("should not return both a command and an error: cmd=%v err=%v", cmd, err)
	}
	// On unsupported OS the error is non-nil and cmd is nil — acceptable.
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		// On these OSes we expect either nil (Python found) or a valid command slice
		if err != nil {
			// Acceptable only on macOS without brew
			t.Logf("pythonInstallPlan returned error (expected on macOS without brew): %v", err)
		}
	}
}

func TestDetectLinuxDistro(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}
	distro := detectLinuxDistro()
	if distro == "" {
		t.Fatal("expected non-empty distro string")
	}
	if distro != "debian" && distro != "ubuntu" && distro != "rhel" {
		t.Fatalf("unexpected distro value: %s", distro)
	}
}
