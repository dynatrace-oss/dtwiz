package installer

import (
	"io"
	"os"
	"sync"
	"testing"
)

// cwdMu serializes tests that temporarily change the process working directory.
// os.Chdir mutates global process state; tests must hold this lock for the duration.
var cwdMu sync.Mutex
var stdoutMu sync.Mutex

// setTestWorkingDir changes CWD to dir for the duration of the test, then restores it.
// Acquires cwdMu to prevent concurrent CWD mutation if t.Parallel() is ever added.
func setTestWorkingDir(t *testing.T, dir string) {
	t.Helper()
	cwdMu.Lock()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		cwdMu.Unlock()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chdir(orig) //nolint:errcheck
		cwdMu.Unlock()
	})
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = w

	defer func() {
		os.Stdout = old
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return string(out)
}
