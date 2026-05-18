package installer

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// withStdin replaces os.Stdin with a pipe that delivers input and restores it
// after fn returns. Tests using this helper must NOT call t.Parallel() because
// they mutate the process-global os.Stdin.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := io.WriteString(w, input); err != nil {
		t.Fatalf("write to stdin pipe: %v", err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()
	fn()
}

// ── ErrInstallCancelled ───────────────────────────────────────────────────────

func TestErrInstallCancelled_IsError(t *testing.T) {
	if ErrInstallCancelled == nil {
		t.Fatal("ErrInstallCancelled must not be nil")
	}
}

func TestErrInstallCancelled_Message(t *testing.T) {
	if !strings.Contains(ErrInstallCancelled.Error(), "cancelled") {
		t.Errorf("ErrInstallCancelled message %q does not contain 'cancelled'", ErrInstallCancelled.Error())
	}
}

func TestErrInstallCancelled_ErrorsIsMatch(t *testing.T) {
	if !errors.Is(ErrInstallCancelled, ErrInstallCancelled) {
		t.Error("errors.Is(ErrInstallCancelled, ErrInstallCancelled) must be true")
	}
}

func TestErrInstallCancelled_IsDistinctFromOtherErrors(t *testing.T) {
	other := errors.New("some other error")
	if errors.Is(other, ErrInstallCancelled) {
		t.Error("arbitrary error must not match ErrInstallCancelled")
	}
	if errors.Is(ErrInstallCancelled, other) {
		t.Error("ErrInstallCancelled must not match an arbitrary error")
	}
}

func TestErrInstallCancelled_WrappedIsDetected(t *testing.T) {
	wrapped := errors.Join(ErrInstallCancelled, errors.New("context"))
	if !errors.Is(wrapped, ErrInstallCancelled) {
		t.Error("errors.Is must detect ErrInstallCancelled through errors.Join wrapping")
	}
}

// ── confirmProceed ────────────────────────────────────────────────────────────

func TestConfirmProceed_AutoConfirmBypassesPrompt(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = true
	defer func() { AutoConfirm = old }()

	// stdin is empty — would fail if actually read
	var ok bool
	var err error
	withStdin(t, "", func() {
		ok, err = confirmProceed("  Proceed?")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("AutoConfirm=true must return true without reading stdin")
	}
}

func TestConfirmProceed_EmptyInputIsYes(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()

	var ok bool
	withStdin(t, "\n", func() {
		var err error
		ok, err = confirmProceed("  Proceed?")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !ok {
		t.Error("empty input (Enter) must be treated as yes")
	}
}

func TestConfirmProceed_YLowercaseIsYes(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()

	for _, input := range []string{"y\n", "Y\n", "yes\n", "YES\n", "Yes\n"} {
		input := input
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			var ok bool
			withStdin(t, input, func() {
				var err error
				ok, err = confirmProceed("  Proceed?")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			if !ok {
				t.Errorf("input %q must be treated as yes, got false", strings.TrimSpace(input))
			}
		})
	}
}

func TestConfirmProceed_NIsNo(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()

	for _, input := range []string{"n\n", "N\n", "no\n", "NO\n"} {
		input := input
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			var ok bool
			withStdin(t, input, func() {
				var err error
				ok, err = confirmProceed("  Proceed?")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			if ok {
				t.Errorf("input %q must be treated as no, got true", strings.TrimSpace(input))
			}
		})
	}
}

func TestConfirmProceed_EOFIsNo(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()

	// EOF on stdin (empty pipe, no newline) — scanner.Scan() returns false.
	var ok bool
	withStdin(t, "", func() {
		var err error
		ok, err = confirmProceed("  Proceed?")
		// err may be nil on EOF; ok must be false
		if err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("unexpected error on EOF: %v", err)
		}
	})
	if ok {
		t.Error("EOF on stdin must be treated as no")
	}
}
