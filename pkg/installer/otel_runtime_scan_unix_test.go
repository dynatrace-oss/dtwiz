//go:build !windows

package installer

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func waitForProcessDetection(t *testing.T, pid int, filterTerm string, excludeTerms []string) bool {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	backoff := 10 * time.Millisecond

	for {
		procs := detectProcesses(filterTerm, excludeTerms)
		for _, p := range procs {
			if p.PID == pid {
				return true
			}
		}

		if time.Now().After(deadline) {
			return false
		}

		time.Sleep(backoff)
	}
}

// TestDetectProcesses_CaseInsensitiveFilterTerm verifies that detectProcesses
// matches process commands case-insensitively on Unix.
//
// Before the fix, a filterTerm of "SLEEP" would not match the lowercase "sleep"
// binary returned by `ps`, causing Python processes with a capital "Python" in
// their path (e.g. macOS framework installs) to be silently skipped.
func TestDetectProcesses_CaseInsensitiveFilterTerm(t *testing.T) {
	// Spawn a sleep process we can look for.
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Skipf("could not start sleep process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	pid := cmd.Process.Pid

	// Search with an upper-cased filter term — should still find the process.
	if !waitForProcessDetection(t, pid, "SLEEP", nil) {
		t.Skipf("sleep PID %d did not appear in detectProcesses(\"SLEEP\") before timeout", pid)
	}
}

// TestDetectProcesses_ExcludeTermsCaseInsensitive verifies that excludeTerms are
// also matched case-insensitively.
func TestDetectProcesses_ExcludeTermsCaseInsensitive(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Skipf("could not start sleep process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	pid := cmd.Process.Pid

	if !waitForProcessDetection(t, pid, "sleep", nil) {
		t.Skipf("sleep PID %d did not appear in detectProcesses(\"sleep\") before timeout", pid)
	}

	// Exclude "SLEEP" (uppercase) — the process must not appear in results.
	procs := detectProcesses("sleep", []string{"SLEEP"})

	for _, p := range procs {
		if p.PID == pid {
			t.Errorf("detectProcesses excluded \"SLEEP\" but PID %d still appeared — case-insensitive exclude broken", pid)
		}
	}
}

// TestIsOtelProcess_WithOtelVars verifies that isOtelProcess
// returns true for a process launched with OTel env vars in a terminal session.
// Since ps eww only exposes env vars for processes in the same session on macOS,
// we verify the current process (which does have a terminal session) when
// OTEL_SERVICE_NAME is already set in the environment.
// This test is skipped if neither OTel marker var is present in the current env.
func TestIsOtelProcess_WithOtelVars(t *testing.T) {
	hasMarker := false
	for _, marker := range otelEnvVarMarkers {
		if os.Getenv(marker) != "" {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		t.Skip("no OTel env vars set in current process — skipping (set OTEL_SERVICE_NAME to run)")
	}

	if !isOtelProcess(os.Getpid()) {
		t.Error("isOtelProcess returned false for current process with OTel env vars set")
	}
}

// TestIsOtelProcess_NoOtelVars verifies that isOtelProcess
// returns false for a non-existent PID (ps/proc will error and we return false).
func TestIsOtelProcess_NoOtelVars(t *testing.T) {
	// PID 0 is never a real user process — ps and /proc will fail.
	if isOtelProcess(0) {
		t.Error("isOtelProcess returned true for PID 0")
	}
}
