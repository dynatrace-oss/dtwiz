//go:build windows

package oneagent

import (
	"os"
	"path/filepath"
	"testing"
)

// withInstallDir is a no-op on Windows: oneAgentInstallDir is defined in the
// Unix build only, so there is nothing to override.
func withInstallDir(t *testing.T, _ string) {
	t.Helper()
}

// withServicePresent overrides oneAgentServicePresentFn for the duration of
// the test so tests can verify detection behaviour without a real Windows
// service.
func withServicePresent(t *testing.T, present bool) {
	t.Helper()
	orig := oneAgentServicePresentFn
	oneAgentServicePresentFn = func() bool { return present }
	t.Cleanup(func() { oneAgentServicePresentFn = orig })
}

func TestOneAgentInstalled_ServicePresent(t *testing.T) {
	withServicePresent(t, true)
	if !oneAgentInstalled() {
		t.Error("expected true when service is present")
	}
}

func TestOneAgentInstalled_ServiceAbsent_OneagentctlOnPATH(t *testing.T) {
	withServicePresent(t, false)

	bin := t.TempDir()
	stub := filepath.Join(bin, "oneagentctl.exe")
	if err := os.WriteFile(stub, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	if !oneAgentInstalled() {
		t.Error("expected true when oneagentctl.exe is on PATH")
	}
}

func TestOneAgentInstalled_ServiceAbsent_NoPATH(t *testing.T) {
	withServicePresent(t, false)
	t.Setenv("PATH", "")

	if oneAgentInstalled() {
		t.Error("expected false when service absent and oneagentctl not on PATH")
	}
}

func TestOneAgentInstalled_DirectoryAloneIsNotSufficient(t *testing.T) {
	withServicePresent(t, false)
	t.Setenv("PATH", "")

	// Simulate the leftover install directory that persists after MSI uninstall.
	programFiles := t.TempDir()
	t.Setenv("ProgramFiles", programFiles)
	if err := os.MkdirAll(filepath.Join(programFiles, "Dynatrace", "OneAgent"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Even if %ProgramFiles%\Dynatrace\OneAgent exists, detection must return false
	// without a service or binary — the directory check was removed precisely for this.
	if oneAgentInstalled() {
		t.Error("expected false: leftover install directory must not trigger detected-as-installed")
	}
}
