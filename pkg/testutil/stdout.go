package testutil

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/fatih/color"
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

// CaptureOutput redirects color.Output (used by fatih/color) to a buffer for
// the duration of fn, with colors disabled so assertions aren't fragile against
// terminal capability detection.
func CaptureOutput(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	origOutput := color.Output
	color.Output = &buf
	t.Cleanup(func() { color.Output = origOutput })

	origNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = origNoColor })

	fn()
	return buf.String()
}

// CaptureErrorOutput redirects color.Error (used by fatih/color for stderr) to
// a buffer for the duration of fn, with colors disabled.
func CaptureErrorOutput(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	origError := color.Error
	color.Error = &buf
	t.Cleanup(func() { color.Error = origError })

	origNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = origNoColor })

	fn()
	return buf.String()
}
