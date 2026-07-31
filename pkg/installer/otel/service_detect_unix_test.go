//go:build !windows

package otel

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func alive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func TestStopService_TerminatesProcess(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	pid := cmd.Process.Pid
	go func() { _, _ = cmd.Process.Wait() }() // reap

	if !alive(pid) {
		t.Fatalf("sleep %d not alive after start", pid)
	}
	if err := stopService(pid); err != nil {
		t.Fatalf("stopService: %v", err)
	}
	if alive(pid) {
		t.Fatalf("process %d still alive after stopService", pid)
	}
}

func TestStopService_AlreadyGone(t *testing.T) {
	cmd := exec.Command("sleep", "0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	_, _ = cmd.Process.Wait()
	// Process already exited — stopService must not error.
	if err := stopService(pid); err != nil {
		t.Fatalf("stopService on dead pid: %v", err)
	}
}

func TestRelaunchService_StartsDetachedWithEnvAndDir(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")

	// Shell script that records cwd and env var, then sleeps to stay observable.
	script := "pwd > cwd.txt; printf '%s' \"$MARKER_VAL\" > marker.txt; sleep 30"
	svc := connectedService{
		pid:     424242,
		name:    "sh probe.sh",
		command: "/bin/sh -c " + script,
		workDir: dir,
		env:     append(os.Environ(), "MARKER_VAL=schnitzel"),
	}

	// Space-split would break the inline script; use a temp file instead.
	scriptFile := filepath.Join(dir, "probe.sh")
	if err := os.WriteFile(scriptFile, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	svc.command = "/bin/sh " + scriptFile

	newPID, err := relaunchService(svc)
	if err != nil {
		t.Fatalf("relaunchService: %v", err)
	}
	defer func() { _ = syscall.Kill(newPID, syscall.SIGKILL) }()

	if newPID <= 0 {
		t.Fatalf("expected positive PID, got %d", newPID)
	}

	// Wait for the marker file the relaunched process writes.
	deadline := time.Now().Add(3 * time.Second)
	var data []byte
	for time.Now().Before(deadline) {
		if data, err = os.ReadFile(marker); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if string(data) != "schnitzel" {
		t.Fatalf("relaunched process did not apply env/dir; marker=%q err=%v", string(data), err)
	}

	// Resolve symlinks: on macOS /var → /private/var.
	cwd, _ := os.ReadFile(filepath.Join(dir, "cwd.txt"))
	gotCwd, _ := filepath.EvalSymlinks(string(trimNL(cwd)))
	wantCwd, _ := filepath.EvalSymlinks(dir)
	if gotCwd != wantCwd {
		t.Fatalf("cwd = %q, want %q", gotCwd, wantCwd)
	}

	// A detached log file was created next to the workdir.
	if _, err := os.Stat(filepath.Join(dir, "dtwiz-sh-probe.sh.log")); err != nil {
		t.Errorf("expected relaunch log file: %v", err)
	}
}

func trimNL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
