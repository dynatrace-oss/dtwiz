package oneagent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// createStubScript writes a minimal agent/uninstall.sh to installDir that exits
// with the given code. Returns the script path.
func createStubScript(t *testing.T, installDir string, exitCode int) string {
	t.Helper()
	agentDir := filepath.Join(installDir, "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}
	script := filepath.Join(agentDir, "uninstall.sh")
	if err := os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)), 0o755); err != nil {
		t.Fatalf("write stub script: %v", err)
	}
	return script
}

func TestUninstallOneAgentV2_NotInstalled_ReturnsError(t *testing.T) {
	skipNonLinux(t)
	withInstallDir(t, filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("PATH", "")

	flush := captureStdout(t)
	err := UninstallOneAgentV2(UninstallOptions{})
	out := flush()

	if err == nil {
		t.Fatal("expected error when OneAgent is not installed")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %q, want 'not installed'", err.Error())
	}
	if out != "" {
		t.Errorf("expected no output when not installed, got: %q", out)
	}
}

func TestUninstallOneAgentV2_DryRun_PrintsPlanAndReturnsNil(t *testing.T) {
	skipNonLinux(t)
	dir := t.TempDir()
	withInstallDir(t, dir)
	withNeedsSudo(t, false)
	createStubScript(t, dir, 0)

	flush := captureStdout(t)
	err := UninstallOneAgentV2(UninstallOptions{DryRun: true})
	out := flush()

	if err != nil {
		t.Fatalf("expected nil error for dry-run, got: %v", err)
	}
	if !strings.Contains(out, "no changes made") {
		t.Errorf("expected 'no changes made' in output, got:\n%s", out)
	}
	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
		t.Error("install dir must not be removed during dry-run")
	}
}

func TestUninstallOneAgentV2_Decline_ReturnsCancelled(t *testing.T) {
	skipNonLinux(t)
	dir := t.TempDir()
	withInstallDir(t, dir)
	withNeedsSudo(t, false)
	withStdin(t, "n\n")

	flush := captureStdout(t)
	err := UninstallOneAgentV2(UninstallOptions{DryRun: false})
	out := flush()

	if !errors.Is(err, installer.ErrInstallCancelled) {
		t.Fatalf("expected ErrInstallCancelled, got: %v", err)
	}
	if !strings.Contains(out, "uninstall cancelled") {
		t.Errorf("expected 'uninstall cancelled' in output, got:\n%s", out)
	}
	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
		t.Error("install dir must not be removed when user declines")
	}
}

func TestUninstallOneAgentV2_Accept_RemovesInstallDir(t *testing.T) {
	skipNonLinux(t)
	dir := t.TempDir()
	withInstallDir(t, dir)
	withNeedsSudo(t, false)
	createStubScript(t, dir, 0)

	orig := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = orig })

	flush := captureStdout(t)
	err := UninstallOneAgentV2(UninstallOptions{DryRun: false})
	flush()

	if err != nil {
		t.Fatalf("expected nil error after accept, got: %v", err)
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Error("expected install dir to be removed after successful uninstall")
	}
}

func TestUninstallOneAgentV2_AutoConfirm_SkipsPrompt(t *testing.T) {
	skipNonLinux(t)
	dir := t.TempDir()
	withInstallDir(t, dir)
	withNeedsSudo(t, false)
	createStubScript(t, dir, 0)

	orig := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = orig })

	// No stdin wired up — any attempt to read stdin would hang/EOF-cancel.
	flush := captureStdout(t)
	err := UninstallOneAgentV2(UninstallOptions{DryRun: false})
	flush()

	if err != nil {
		t.Fatalf("expected nil error with AutoConfirm, got: %v", err)
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Error("expected install dir to be removed when AutoConfirm bypasses prompt")
	}
}

func TestUninstallOneAgentV2_ScriptFailure_PropagatesError(t *testing.T) {
	skipNonLinux(t)
	dir := t.TempDir()
	withInstallDir(t, dir)
	withNeedsSudo(t, false)
	createStubScript(t, dir, 1) // exits non-zero

	orig := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = orig })

	flush := captureStdout(t)
	err := UninstallOneAgentV2(UninstallOptions{DryRun: false})
	out := flush()

	if err == nil {
		t.Fatal("expected error when uninstall script exits non-zero")
	}
	if strings.Contains(out, "OneAgent uninstalled successfully") {
		t.Error("success message must not be printed when uninstall fails")
	}
}
