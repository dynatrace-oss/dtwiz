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

// withRunCommand overrides runCommandFn for the duration of the test.
func withRunCommand(t *testing.T, fn func(name string, args ...string) error) {
	t.Helper()
	orig := runCommandFn
	runCommandFn = fn
	t.Cleanup(func() { runCommandFn = orig })
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

func TestRunUninstall_ScriptMissing_Error(t *testing.T) {
	withInstallDir(t, t.TempDir()) // dir exists but has no agent/uninstall.sh
	withNeedsSudo(t, false)

	err := runUninstall()
	if err == nil {
		t.Fatal("expected error when uninstall script is missing")
	}
	if !strings.Contains(err.Error(), "agent/uninstall.sh") {
		t.Errorf("error = %q, want script path in message", err.Error())
	}
}

func TestRunUninstall_NeedsNoSudo_ArgvStartsWithScript(t *testing.T) {
	dir := t.TempDir()
	withInstallDir(t, dir)
	withNeedsSudo(t, false)
	createStubScript(t, dir, 0)

	expectedScript := uninstallScriptPath()
	var capturedName string
	withRunCommand(t, func(name string, _ ...string) error {
		capturedName = name
		return nil
	})

	flush := captureStdout(t)
	if err := runUninstall(); err != nil {
		flush()
		t.Fatalf("unexpected error: %v", err)
	}
	flush()

	if capturedName != expectedScript {
		t.Errorf("argv[0] = %q, want script path %q", capturedName, expectedScript)
	}
}

func TestRunUninstall_NeedsSudo_ArgvStartsWithSudo(t *testing.T) {
	dir := t.TempDir()
	withInstallDir(t, dir)
	withNeedsSudo(t, true)
	createStubScript(t, dir, 0)

	const stubSudo = "/stub/sudo"
	withSudoPath(t, stubSudo)

	var capturedName string
	var capturedArgs []string
	withRunCommand(t, func(name string, args ...string) error {
		capturedName = name
		capturedArgs = args
		return nil
	})

	flush := captureStdout(t)
	// runUninstall will attempt cleanupInstallDir with sudo via installer.RunCommand
	// (not runCommandFn). That call will fail since /stub/sudo doesn't exist — the
	// failure is only a warning and does not affect the return value we care about.
	_ = runUninstall()
	flush()

	if capturedName != stubSudo {
		t.Errorf("argv[0] = %q, want sudo path %q", capturedName, stubSudo)
	}
	if len(capturedArgs) == 0 || capturedArgs[0] != uninstallScriptPath() {
		t.Errorf("argv[1] = %v, want uninstall script path %q", capturedArgs, uninstallScriptPath())
	}
}
