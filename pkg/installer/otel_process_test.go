package installer

import (
	"errors"
	"fmt"
	"io"
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return string(out)
}