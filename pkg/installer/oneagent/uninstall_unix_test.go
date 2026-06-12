//go:build !windows

package oneagent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// --- UninstallOneAgentV2 ---

func TestUninstallOneAgentV2_NotInstalled(t *testing.T) {
	withInstallDir(t, filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("PATH", "")

	err := UninstallOneAgentV2(UninstallOptions{})
	if err == nil {
		t.Fatal("expected error when OneAgent is not installed")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %q, want 'not installed' message", err)
	}
}

func TestUninstallOneAgentV2_DryRun_NoPlanNoExecution(t *testing.T) {
	withInstallDir(t, t.TempDir()) // install dir exists → detected as installed
	withNeedsSudo(t, false)
	flush := captureStdout(t)

	// The install dir has no agent/uninstall.sh — if runUninstall were called it
	// would return an error, so a nil return proves dry-run short-circuits before
	// executing anything.
	err := UninstallOneAgentV2(UninstallOptions{DryRun: true})
	out := flush()

	if err != nil {
		t.Fatalf("dry-run returned unexpected error: %v", err)
	}
	if !strings.Contains(out, "Script:") {
		t.Errorf("expected script path in dry-run plan output, got:\n%s", out)
	}
}

func TestUninstallOneAgentV2_Cancelled(t *testing.T) {
	withInstallDir(t, t.TempDir())
	withNeedsSudo(t, false)
	withStdin(t, "n\n")

	err := UninstallOneAgentV2(UninstallOptions{})
	if !errors.Is(err, installer.ErrInstallCancelled) {
		t.Fatalf("expected ErrInstallCancelled on decline, got: %v", err)
	}
}

func TestUninstallOneAgentV2_Confirmed_ScriptMissing(t *testing.T) {
	withInstallDir(t, t.TempDir()) // dir exists but contains no uninstall.sh
	withNeedsSudo(t, false)
	withStdin(t, "y\n")

	err := UninstallOneAgentV2(UninstallOptions{})
	if err == nil {
		t.Fatal("expected error when uninstall script is missing")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found' message", err)
	}
}

// --- cleanupInstallDir ---

func TestCleanupInstallDir_PathAbsent(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "nonexistent")

	if err := cleanupInstallDir(absent, false); err != nil {
		t.Errorf("expected no error for absent path, got: %v", err)
	}
}

func TestCleanupInstallDir_PathIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "somefile")
	if err := os.WriteFile(file, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := cleanupInstallDir(file, false); err != nil {
		t.Errorf("expected no error for file path, got: %v", err)
	}
	if _, err := os.Stat(file); os.IsNotExist(err) {
		t.Error("file was removed, but cleanupInstallDir should skip non-directories")
	}
}

func TestCleanupInstallDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "stub")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}

	if err := cleanupInstallDir(target, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("expected directory to be removed")
	}
}

func TestCleanupInstallDir_NonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "stub")
	if err := os.MkdirAll(filepath.Join(target, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "sub", "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := cleanupInstallDir(target, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("expected directory tree to be removed")
	}
}
