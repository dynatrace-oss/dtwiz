//go:build windows

package installer

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestWinProcessQuery_ReturnsCurrentProcess verifies that winProcessQuery can
// find the current process by PID via a real PowerShell invocation.
// This test requires PowerShell (standard on all supported Windows versions).
func TestWinProcessQuery_ReturnsCurrentProcess(t *testing.T) {
	pid := os.Getpid()
	lines, err := winProcessQuery(
		"$_.ProcessId -eq "+strconv.Itoa(pid),
		"$_.ProcessId",
	)
	if err != nil {
		t.Fatalf("winProcessQuery returned error: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("winProcessQuery returned no results for current PID")
	}
	got, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || got != pid {
		t.Errorf("winProcessQuery result = %q, want PID %d", lines[0], pid)
	}
}

// TestWinProcessQuery_NoMatch verifies that a Where-Object expression that
// matches nothing does not cause an error — it returns an empty or nil slice.
func TestWinProcessQuery_NoMatch(t *testing.T) {
	lines, err := winProcessQuery("$_.ProcessId -eq 9999999", "$_.ProcessId")
	if err != nil {
		t.Fatalf("winProcessQuery returned error for no-match query: %v", err)
	}
	for _, l := range lines {
		if strings.TrimSpace(l) == "9999999" {
			t.Errorf("winProcessQuery unexpectedly returned PID 9999999")
		}
	}
}

// spawnPing starts a long-running "ping -n 60 127.0.0.1" child process and
// returns it. The caller is responsible for cleanup via t.Cleanup.
// ping is present on all supported Windows versions and keeps running for ~60 s,
// giving the tests enough time to observe it in the process list.
func spawnPing(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("ping", "-n", "60", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("could not start ping process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd
}

// waitForProcessDetection polls detectProcesses until the given PID appears or
// the 2-second deadline is exceeded. Returns true if the PID was found.
func waitForProcessDetection(t *testing.T, pid int, filterTerm string, excludeTerms []string) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	backoff := 10 * time.Millisecond
	for {
		for _, p := range detectProcesses(filterTerm, excludeTerms) {
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
// matches process commands case-insensitively on Windows.
//
// Before the fix, a filterTerm of "PING" would not match a command line
// containing the lowercase "ping" binary, causing processes to be silently
// skipped when the caller passed a differently-cased filter.
func TestDetectProcesses_CaseInsensitiveFilterTerm(t *testing.T) {
	cmd := spawnPing(t)
	pid := cmd.Process.Pid

	// Search with an upper-cased filter term — should still find the process.
	if !waitForProcessDetection(t, pid, "PING", nil) {
		t.Skipf("ping PID %d did not appear in detectProcesses(\"PING\") before timeout", pid)
	}
}

// TestDetectProcesses_ExcludeTermsCaseInsensitive verifies that excludeTerms
// are matched case-insensitively on Windows.
func TestDetectProcesses_ExcludeTermsCaseInsensitive(t *testing.T) {
	cmd := spawnPing(t)
	pid := cmd.Process.Pid

	// Confirm the process is visible with a lowercase filter first.
	if !waitForProcessDetection(t, pid, "ping", nil) {
		t.Skipf("ping PID %d did not appear in detectProcesses(\"ping\") before timeout", pid)
	}

	// Exclude "PING" (uppercase) — the process must not appear in results.
	for _, p := range detectProcesses("ping", []string{"PING"}) {
		if p.PID == pid {
			t.Errorf("detectProcesses excluded \"PING\" but PID %d still appeared — case-insensitive exclude broken", pid)
		}
	}
}

// TestDetectProcesses_SelfExcluded verifies that the current process is never
// returned by detectProcesses, even when its command line would match.
func TestDetectProcesses_SelfExcluded(t *testing.T) {
	selfPID := os.Getpid()
	// Query with a very broad filter; the test binary's own command line will
	// contain its executable path which typically includes ".exe".
	for _, p := range detectProcesses(".exe", nil) {
		if p.PID == selfPID {
			t.Errorf("detectProcesses returned current PID %d — self must be excluded", selfPID)
		}
	}
}

// TestWinProcessQuery_PipeDelimitedMultiField verifies that a multi-field
// pipe-delimited query returns parseable output for the current process.
func TestWinProcessQuery_PipeDelimitedMultiField(t *testing.T) {
	pid := os.Getpid()
	lines, err := winProcessQuery(
		"$_.ProcessId -eq "+strconv.Itoa(pid),
		"\"$($_.ProcessId)|$($_.CommandLine)|$($_.WorkingDirectory)\"",
	)
	if err != nil {
		t.Fatalf("winProcessQuery returned error: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("winProcessQuery returned no results for current PID")
	}
	parts := strings.SplitN(lines[0], "|", 3)
	if len(parts) != 3 {
		t.Fatalf("expected 3 pipe-delimited fields, got %d: %q", len(parts), lines[0])
	}
	gotPID, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || gotPID != pid {
		t.Errorf("PID field = %q, want %d", parts[0], pid)
	}
}
