package installer

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagedProcessHelper(t *testing.T) {
	if os.Getenv("GO_WANT_MANAGED_PROCESS_HELPER") != "1" {
		return
	}

	switch os.Getenv("MANAGED_PROCESS_MODE") {
	case "exit1":
		fmt.Fprintln(os.Stderr, "boom")
		os.Exit(1)
	case "exit0":
		os.Exit(0)
	case "block":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	default:
		fmt.Fprintln(os.Stderr, "unknown helper mode")
		os.Exit(2)
	}
}

func TestWaitResult_Idempotent(t *testing.T) {
	cmd := managedProcessHelperCommand(t, "exit1")
	logFile := createManagedProcessLogFile(t)
	mp, err := StartManagedProcess("svc", filepath.Base(logFile.Name()), cmd, logFile)
	if err != nil {
		t.Fatalf("StartManagedProcess() error = %v", err)
	}

	firstExited, firstErr := waitForManagedProcessExit(t, mp)
	secondExited, secondErr := mp.WaitResult()

	if !firstExited || firstErr == nil {
		t.Fatalf("first WaitResult() = (%v, %v), want (true, non-nil)", firstExited, firstErr)
	}
	if !secondExited || secondErr == nil {
		t.Fatalf("second WaitResult() = (%v, %v), want (true, non-nil)", secondExited, secondErr)
	}
}

func TestWaitResult_StillRunning(t *testing.T) {
	cmd := managedProcessHelperCommand(t, "block")
	logFile := createManagedProcessLogFile(t)
	mp, err := StartManagedProcess("svc", filepath.Base(logFile.Name()), cmd, logFile)
	if err != nil {
		t.Fatalf("StartManagedProcess() error = %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	exited, waitErr := mp.WaitResult()
	if exited || waitErr != nil {
		t.Fatalf("WaitResult() = (%v, %v), want (false, nil)", exited, waitErr)
	}
}

func TestPrintProcessSummary_AllCrashed_NoAliveNames(t *testing.T) {
	out := captureStdout(t, func() {
		aliveNames, _ := PrintProcessSummary([]*ManagedProcess{
			crashedManagedProcess("svc-a", errors.New("exit status 1")),
			crashedManagedProcess("svc-b", errors.New("exit status 2")),
		}, 0)
		if len(aliveNames) != 0 {
			t.Fatalf("aliveNames = %v, want empty", aliveNames)
		}
	})
	if !strings.Contains(out, "[crashed:") {
		t.Fatalf("expected crash summary output, got %q", out)
	}
}

func TestPrintProcessSummary_SomeCrashed_OnlyAliveReturned(t *testing.T) {
	aliveNames, _ := PrintProcessSummary([]*ManagedProcess{
		crashedManagedProcess("svc-crashed", errors.New("exit status 1")),
		runningManagedProcess("svc-running"),
	}, 0)

	if len(aliveNames) != 1 || aliveNames[0] != "svc-running" {
		t.Fatalf("aliveNames = %v, want [svc-running]", aliveNames)
	}
}

func TestPrintProcessSummary_CrashedNonZeroExit_SummaryLabel(t *testing.T) {
	out := captureStdout(t, func() {
		PrintProcessSummary([]*ManagedProcess{
			crashedManagedProcess("svc", errors.New("exit status 1")),
		}, 0)
	})
	if !strings.Contains(out, "[crashed:") {
		t.Fatalf("expected crashed label, got %q", out)
	}
}

func TestPrintProcessSummary_CleanExit_SummaryLabel(t *testing.T) {
	out := captureStdout(t, func() {
		PrintProcessSummary([]*ManagedProcess{
			cleanExitedManagedProcess("svc"),
		}, 0)
	})
	if !strings.Contains(out, "[exited cleanly]") {
		t.Fatalf("expected clean exit label, got %q", out)
	}
}

func managedProcessHelperCommand(t *testing.T, mode string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestManagedProcessHelper")
	cmd.Env = append(os.Environ(),
		"GO_WANT_MANAGED_PROCESS_HELPER=1",
		"MANAGED_PROCESS_MODE="+mode,
	)
	return cmd
}

func createManagedProcessLogFile(t *testing.T) *os.File {
	t.Helper()
	logFile, err := os.CreateTemp(t.TempDir(), "managed-process-*.log")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	return logFile
}

func waitForManagedProcessExit(t *testing.T, proc *ManagedProcess) (bool, error) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		exited, err := proc.WaitResult()
		if exited {
			return true, err
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("managed process did not exit within timeout")
	return false, nil
}

func crashedManagedProcess(name string, exitErr error) *ManagedProcess {
	exitCh := make(chan error, 1)
	exitCh <- exitErr
	return &ManagedProcess{Name: name, PID: 999999, LogName: name + ".log", exitResultCh: exitCh}
}

func cleanExitedManagedProcess(name string) *ManagedProcess {
	exitCh := make(chan error, 1)
	exitCh <- nil
	return &ManagedProcess{Name: name, PID: 999999, LogName: name + ".log", exitResultCh: exitCh}
}

func runningManagedProcess(name string) *ManagedProcess {
	return &ManagedProcess{Name: name, PID: 999999, LogName: name + ".log", exitResultCh: make(chan error, 1)}
}

func TestStartManagedProcess_CleanExit(t *testing.T) {
	cmd := managedProcessHelperCommand(t, "exit0")
	logFile := createManagedProcessLogFile(t)
	mp, err := StartManagedProcess("svc", filepath.Base(logFile.Name()), cmd, logFile)
	if err != nil {
		t.Fatalf("StartManagedProcess() error = %v", err)
	}

	exited, waitErr := waitForManagedProcessExit(t, mp)
	if !exited || waitErr != nil {
		t.Fatalf("WaitResult() = (%v, %v), want (true, nil)", exited, waitErr)
	}
}

func TestPrintSummaryLine_Crashed_IncludesLabel(t *testing.T) {
	p := crashedManagedProcess("my-svc", errors.New("exit status 1"))
	out := captureStdout(t, func() { p.PrintSummaryLine() })
	if !strings.Contains(out, "[crashed:") {
		t.Fatalf("expected [crashed: in output, got %q", out)
	}
	if !strings.Contains(out, "my-svc") {
		t.Fatalf("expected service name in output, got %q", out)
	}
}

func TestPrintSummaryLine_CleanExit_IncludesLabel(t *testing.T) {
	p := cleanExitedManagedProcess("my-svc")
	out := captureStdout(t, func() { p.PrintSummaryLine() })
	if !strings.Contains(out, "[exited cleanly]") {
		t.Fatalf("expected [exited cleanly] in output, got %q", out)
	}
}

// TestPrintSummaryLine_Running verifies that a still-running process prints
// either "[running, port not detected]" (no open port for the fake PID) or
// a localhost URL if lsof happens to find one — both are valid "running" states.
func TestPrintSummaryLine_Running_IncludesRunningStatus(t *testing.T) {
	p := runningManagedProcess("my-svc")
	out := captureStdout(t, func() { p.PrintSummaryLine() })
	if !strings.Contains(out, "running") && !strings.Contains(out, "localhost") {
		t.Fatalf("expected running status in output, got %q", out)
	}
}

func TestPrintSummaryLine_LogNameIncluded(t *testing.T) {
	p := cleanExitedManagedProcess("my-svc") // LogName is "my-svc.log"
	out := captureStdout(t, func() { p.PrintSummaryLine() })
	if !strings.Contains(out, "my-svc.log") {
		t.Fatalf("expected log name in output, got %q", out)
	}
}