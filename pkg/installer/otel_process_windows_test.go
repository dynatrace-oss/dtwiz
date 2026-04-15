//go:build windows

package installer

import (
	"errors"
	"testing"
)

// TestAdoptExeclChildren_NoExitedProcesses verifies that adoptExeclChildren
// does not modify counters when all processes are still running.
func TestAdoptExeclChildren_NoExitedProcesses(t *testing.T) {
	started := 2
	notStarted := 0
	procs := []*ManagedProcess{
		runningManagedProcess("svc-a"),
		runningManagedProcess("svc-b"),
	}
	adoptExeclChildren(procs, &started, &notStarted)
	if started != 2 || notStarted != 0 {
		t.Errorf("counters changed unexpectedly: started=%d notStarted=%d", started, notStarted)
	}
}

// TestAdoptExeclChildren_CrashedProcessSkipped verifies that a process that
// exited with a non-zero error code is not eligible for adoption.
func TestAdoptExeclChildren_CrashedProcessSkipped(t *testing.T) {
	started := 0
	notStarted := 1
	originalPID := 999999
	procs := []*ManagedProcess{
		crashedManagedProcess("svc", errors.New("exit status 1")),
	}
	adoptExeclChildren(procs, &started, &notStarted)
	if started != 0 || notStarted != 1 {
		t.Errorf("counters changed for a crashed process: started=%d notStarted=%d", started, notStarted)
	}
	if procs[0].PID != originalPID {
		t.Errorf("PID was modified for a crashed process: got %d, want %d", procs[0].PID, originalPID)
	}
}

// TestAdoptExeclChildren_CleanExitNoChildrenSkipped verifies that a process
// that exited cleanly (exit code 0) but has no Python children is left as-is
// — not adopted, counters unchanged.
func TestAdoptExeclChildren_CleanExitNoChildrenSkipped(t *testing.T) {
	// Use the current process as the parent PID — it has no python.exe children
	// in a Go test binary, so pythonChildPIDs should return empty.
	p := cleanExitedManagedProcess("svc")
	// Override PID to the current process so pythonChildPIDs queries a real PID
	// (one with no python children).
	p.PID = 99999999 // nonexistent — pythonChildPIDs will return nil/empty

	started := 0
	notStarted := 1
	adoptExeclChildren([]*ManagedProcess{p}, &started, &notStarted)

	// No children found → no adoption → counters unchanged, PID unchanged.
	if started != 0 || notStarted != 1 {
		t.Errorf("counters changed despite no python child: started=%d notStarted=%d", started, notStarted)
	}
	if p.PID != 99999999 {
		t.Errorf("PID was changed despite no python child: got %d", p.PID)
	}
}
