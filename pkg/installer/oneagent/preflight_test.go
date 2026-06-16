package oneagent

import (
	"errors"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// withOneAgentDetected overrides oneAgentInstalledFn for the duration of the test.
func withOneAgentDetected(t *testing.T, installed bool) {
	t.Helper()
	orig := oneAgentInstalledFn
	oneAgentInstalledFn = func() bool { return installed }
	t.Cleanup(func() { oneAgentInstalledFn = orig })
}

// withSudoMissing overrides sudoPathFn to simulate sudo not found.
func withSudoMissing(t *testing.T) {
	t.Helper()
	orig := sudoPathFn
	sudoPathFn = func() (string, error) { return "", errors.New("not in PATH") }
	t.Cleanup(func() { sudoPathFn = orig })
}

func TestRunPreflightChecks_NoInstall_NoSudo(t *testing.T) {
	withOneAgentDetected(t, false)
	withNeedsSudo(t, false)

	result, err := runPreflightChecks(Environment{OS: "linux", Arch: "x86"}, InstallOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsUpdate {
		t.Error("expected IsUpdate=false when no agent installed")
	}
}

func TestRunPreflightChecks_NoInstall_SudoAvailable(t *testing.T) {
	withOneAgentDetected(t, false)
	withNeedsSudo(t, true)
	withSudoPath(t, "/usr/bin/sudo")

	result, err := runPreflightChecks(Environment{OS: "linux", Arch: "x86"}, InstallOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsUpdate {
		t.Error("expected IsUpdate=false")
	}
}

func TestRunPreflightChecks_NoInstall_SudoMissing(t *testing.T) {
	withOneAgentDetected(t, false)
	withNeedsSudo(t, true)
	withSudoMissing(t)

	_, err := runPreflightChecks(Environment{OS: "linux", Arch: "x86"}, InstallOptions{})
	if err == nil {
		t.Fatal("expected error when sudo missing")
	}
	if !strings.Contains(err.Error(), "sudo not found") || !strings.Contains(err.Error(), "root") {
		t.Errorf("error = %q, want message containing 'sudo not found' and 'root'", err)
	}
}

func TestRunPreflightChecks_ExistingInstall_Confirmed(t *testing.T) {
	withOneAgentDetected(t, true)
	withNeedsSudo(t, false)
	withStdin(t, "y\n")

	result, err := runPreflightChecks(Environment{OS: "linux", Arch: "x86"}, InstallOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsUpdate {
		t.Error("expected IsUpdate=true")
	}
}

func TestRunPreflightChecks_ExistingInstall_Declined(t *testing.T) {
	withOneAgentDetected(t, true)
	withNeedsSudo(t, false)
	withStdin(t, "n\n")

	_, err := runPreflightChecks(Environment{OS: "linux", Arch: "x86"}, InstallOptions{})
	if !errors.Is(err, installer.ErrInstallCancelled) {
		t.Fatalf("expected ErrInstallCancelled, got: %v", err)
	}
}

func TestRunPreflightChecks_ExistingInstall_DryRun(t *testing.T) {
	withOneAgentDetected(t, true)
	withNeedsSudo(t, false)

	result, err := runPreflightChecks(Environment{OS: "linux", Arch: "x86"}, InstallOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsUpdate {
		t.Error("expected IsUpdate=true even in dry-run")
	}
}

func TestRunPreflightChecks_ExistingInstall_Quiet(t *testing.T) {
	withOneAgentDetected(t, true)
	withNeedsSudo(t, false)

	result, err := runPreflightChecks(Environment{OS: "linux", Arch: "x86"}, InstallOptions{Quiet: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsUpdate {
		t.Error("expected IsUpdate=true in quiet mode")
	}
}

func TestRunPreflightChecks_Windows_SkipsSudo(t *testing.T) {
	withOneAgentDetected(t, false)

	sudoCalled := false
	orig := sudoPathFn
	sudoPathFn = func() (string, error) {
		sudoCalled = true
		return "", nil
	}
	t.Cleanup(func() { sudoPathFn = orig })

	_, err := runPreflightChecks(Environment{OS: "windows", Arch: "x86"}, InstallOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sudoCalled {
		t.Error("sudoPathFn should not be called on non-Linux env.OS")
	}
}
