package testutil

import (
	"io"
	"os"
	"sync"
	"testing"
)

// stdoutMu serializes tests that temporarily replace os.Stdout.
// os.Stdout is process-global; tests must hold this lock for the duration.
var stdoutMu sync.Mutex

// CaptureStdout redirects os.Stdout to a pipe for the duration of fn and
// returns everything written to it. Tests using this helper must not call
// t.Parallel() — os.Stdout is process-global state.
func CaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	r.Close()
	return string(out)
}
