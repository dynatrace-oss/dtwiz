//go:build windows

package otel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// processAlive reports whether the given PID is still listed in the process table.
func processAlive(pid int) bool {
	pidStr := strconv.Itoa(pid)
	out, err := exec.Command("tasklist", "/FI", "PID eq "+pidStr, "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), pidStr)
}

func TestStopService_TerminatesProcess(t *testing.T) {
	// powershell Start-Sleep works in non-interactive (no-console) environments.
	// "cmd /c timeout" exits immediately when stdin is not an interactive console.
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Start-Sleep -Seconds 60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	// Give the process a moment to appear in the process table.
	time.Sleep(200 * time.Millisecond)

	if !processAlive(pid) {
		t.Fatalf("process %d not alive after start", pid)
	}
	if err := stopService(pid); err != nil {
		t.Fatalf("stopService: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("process %d still alive after stopService", pid)
}

func TestStopService_AlreadyGone(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	_, _ = cmd.Process.Wait()
	if err := stopService(pid); err != nil {
		t.Fatalf("stopService on dead pid: %v", err)
	}
}

func TestRelaunchService_StartsDetachedWithEnvAndDir(t *testing.T) {
	dir := t.TempDir()
	markerPath := filepath.Join(dir, "marker.txt")

	// Batch script: write env var to file, then spin via ping (no console required,
	// unlike "pause" which exits immediately in a detached/non-interactive process).
	scriptPath := filepath.Join(dir, "probe.bat")
	script := "@echo off\r\necho %MARKER_VAL%> marker.txt\r\nping /n 30 127.0.0.1 > nul\r\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	// "cmd /c <path>" has no spaces when the temp path has no spaces (CI runner:
	// runneradmin — no spaces), so strings.Fields in relaunchService parses it correctly.
	svc := connectedService{
		pid:     424242,
		name:    "probe",
		command: "cmd /c " + scriptPath,
		workDir: dir,
		env:     append(os.Environ(), "MARKER_VAL=schnitzel"),
	}

	newPID, err := relaunchService(svc)
	if err != nil {
		t.Fatalf("relaunchService: %v", err)
	}
	t.Cleanup(func() { _ = stopService(newPID) })

	if newPID <= 0 {
		t.Fatalf("expected positive PID, got %d", newPID)
	}

	deadline := time.Now().Add(5 * time.Second)
	var data []byte
	for time.Now().Before(deadline) {
		if data, err = os.ReadFile(markerPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if strings.TrimSpace(string(data)) != "schnitzel" {
		t.Fatalf("relaunched process did not write expected marker; content=%q err=%v", strings.TrimSpace(string(data)), err)
	}
}

func TestPortAfterLastColon(t *testing.T) {
	cases := []struct {
		addr string
		want string
	}{
		{"127.0.0.1:4317", "4317"},
		{"[::1]:4318", "4318"},
		{"0.0.0.0:12345", "12345"},
		{"", ""},
		{"noport", ""},
	}
	for _, c := range cases {
		if got := portAfterLastColon(c.addr); got != c.want {
			t.Errorf("portAfterLastColon(%q) = %q, want %q", c.addr, got, c.want)
		}
	}
}

func TestDetectServicesOnPorts_DoesNotPanic(t *testing.T) {
	// Smoke test: parsing must complete without panicking regardless of
	// what connections are active on the machine at test time.
	selfPID := os.Getpid()
	result := detectServicesOnPorts([]string{"4317", "4318"})
	for _, svc := range result {
		if svc.pid <= 0 {
			t.Errorf("service has non-positive PID: %+v", svc)
		}
		if svc.pid == selfPID {
			t.Errorf("own PID %d leaked into detectServicesOnPorts results", selfPID)
		}
	}
}
