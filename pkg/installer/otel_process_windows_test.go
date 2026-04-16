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

// TestAdoptExeclChildren_NotExeclLauncher_Skipped verifies that a process that
// exited cleanly but is not marked as an execl launcher is not adopted.
func TestAdoptExeclChildren_NotExeclLauncher_Skipped(t *testing.T) {
	p := cleanExitedManagedProcess("svc") // IsExeclLauncher defaults to false
	started := 0
	notStarted := 1
	adoptExeclChildren([]*ManagedProcess{p}, &started, &notStarted)
	if started != 0 || notStarted != 1 {
		t.Errorf("counters changed for non-launcher: started=%d notStarted=%d", started, notStarted)
	}
	if p.PID != 999999 {
		t.Errorf("PID was modified for non-launcher: got %d", p.PID)
	}
}

// TestAdoptExeclChildren_NoChildFound_Skipped verifies that an execl launcher
// that exited cleanly but has no matching Python child is not adopted —
// counters unchanged, PID unchanged.
func TestAdoptExeclChildren_NoChildFound_Skipped(t *testing.T) {
	p := cleanExitedManagedProcess("svc")
	p.IsExeclLauncher = true
	// Use an entrypoint that cannot appear in any real process CommandLine.
	p.Entrypoint = `C:\nonexistent_dtwiz_test_xyz_app.py`

	started := 0
	notStarted := 1
	adoptExeclChildren([]*ManagedProcess{p}, &started, &notStarted)

	if started != 0 || notStarted != 1 {
		t.Errorf("counters changed despite no python child: started=%d notStarted=%d", started, notStarted)
	}
	if p.PID != 999999 {
		t.Errorf("PID was changed despite no python child: got %d", p.PID)
	}
}

// TestWatchPID_NonexistentPID verifies that watchPID handles an OpenProcess
// failure gracefully — the channel receives nil (process treated as gone)
// without blocking.
func TestWatchPID_NonexistentPID(t *testing.T) {
	ch := watchPID(99999999) // PID almost certainly does not exist
	select {
	case err := <-ch:
		if err != nil {
			t.Errorf("watchPID returned non-nil error for nonexistent PID: %v", err)
		}
	default:
		t.Error("watchPID channel was empty; expected immediate nil send on OpenProcess failure")
	}
}
