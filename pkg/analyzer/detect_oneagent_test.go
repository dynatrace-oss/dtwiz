//go:build !windows

package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSystemdAvailable(t *testing.T) {
	orig := systemdRunDir
	t.Cleanup(func() { systemdRunDir = orig })

	t.Run("directory exists", func(t *testing.T) {
		systemdRunDir = t.TempDir()
		if !systemdAvailable() {
			t.Error("expected systemdAvailable() = true when the directory exists")
		}
	})

	t.Run("directory missing", func(t *testing.T) {
		systemdRunDir = filepath.Join(t.TempDir(), "does-not-exist")
		if systemdAvailable() {
			t.Error("expected systemdAvailable() = false when the directory is missing")
		}
	})

	t.Run("path is a file", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "system")
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		systemdRunDir = f
		if systemdAvailable() {
			t.Error("expected systemdAvailable() = false when the path is a regular file")
		}
	})
}

// writeShim writes an executable script named name into dir.
func writeShim(t *testing.T, dir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDetectOneAgentIgnoresSystemctlShimWithoutSystemd(t *testing.T) {
	orig := systemdRunDir
	t.Cleanup(func() { systemdRunDir = orig })

	// PATH contains only a systemctl shim that exits 0 for any invocation,
	// mimicking devcontainer images. No oneagentctl is available.
	bin := t.TempDir()
	writeShim(t, bin, "systemctl", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)

	t.Run("no systemd: shim exit code is not trusted", func(t *testing.T) {
		systemdRunDir = filepath.Join(t.TempDir(), "does-not-exist")
		if detectOneAgent() {
			t.Error("expected detectOneAgent() = false: systemctl shim must be ignored when systemd is not running")
		}
	})

	t.Run("systemd running: systemctl result is trusted", func(t *testing.T) {
		systemdRunDir = t.TempDir()
		if !detectOneAgent() {
			t.Error("expected detectOneAgent() = true: systemctl reported the oneagent unit active")
		}
	})
}

func TestDetectOneAgentFallsBackToOneagentctl(t *testing.T) {
	orig := systemdRunDir
	t.Cleanup(func() { systemdRunDir = orig })
	systemdRunDir = filepath.Join(t.TempDir(), "does-not-exist")

	bin := t.TempDir()
	t.Setenv("PATH", bin)

	t.Run("oneagentctl present", func(t *testing.T) {
		writeShim(t, bin, "oneagentctl", "#!/bin/sh\nexit 0\n")
		if !detectOneAgent() {
			t.Error("expected detectOneAgent() = true when oneagentctl is in PATH")
		}
	})

	t.Run("oneagentctl absent", func(t *testing.T) {
		if err := os.Remove(filepath.Join(bin, "oneagentctl")); err != nil {
			t.Fatal(err)
		}
		if detectOneAgent() {
			t.Error("expected detectOneAgent() = false when neither systemd nor oneagentctl is available")
		}
	})
}
