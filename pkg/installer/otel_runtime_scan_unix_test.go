//go:build !windows

package installer

import (
	"os/exec"
	"testing"
	"time"
)

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

	// Give the OS a moment to register the process in the ps table.
	time.Sleep(50 * time.Millisecond)

	pid := cmd.Process.Pid

	// Search with an upper-cased filter term — should still find the process.
	procs := detectProcesses("SLEEP", nil)

	found := false
	for _, p := range procs {
		if p.PID == pid {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("detectProcesses(\"SLEEP\") did not find sleep PID %d — case-insensitive match broken", pid)
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

	time.Sleep(50 * time.Millisecond)

	pid := cmd.Process.Pid

	// Exclude "SLEEP" (uppercase) — the process must not appear in results.
	procs := detectProcesses("sleep", []string{"SLEEP"})

	for _, p := range procs {
		if p.PID == pid {
			t.Errorf("detectProcesses excluded \"SLEEP\" but PID %d still appeared — case-insensitive exclude broken", pid)
		}
	}
}
