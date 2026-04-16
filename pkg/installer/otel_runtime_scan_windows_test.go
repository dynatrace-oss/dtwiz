//go:build windows

package installer

import (
	"os"
	"strconv"
	"strings"
	"testing"
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

// TestPythonChildPIDs_NoChildren verifies that pythonChildPIDs returns a valid
// (possibly empty) result for a process known to have no Python children.
func TestPythonChildPIDs_NoChildren(t *testing.T) {
	pids, err := pythonChildPIDs(os.Getpid())
	if err != nil {
		t.Fatalf("pythonChildPIDs returned error: %v", err)
	}
	for _, pid := range pids {
		if pid <= 0 {
			t.Errorf("pythonChildPIDs returned non-positive PID: %d", pid)
		}
	}
}
