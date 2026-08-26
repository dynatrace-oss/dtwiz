package installer

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestJitter_WithinExpectedBounds(t *testing.T) {
	const base = 100
	for i := 0; i < 1000; i++ {
		got := Jitter(base)
		if got < base || got > base+base/2 {
			t.Fatalf("Jitter(%d) = %d, want within [%d, %d]", base, got, base, base+base/2)
		}
	}
}

func TestJitter_ZeroOrNegativeIsUnchanged(t *testing.T) {
	if got := Jitter(0); got != 0 {
		t.Errorf("Jitter(0) = %d, want 0", got)
	}
	if got := Jitter(-5); got != -5 {
		t.Errorf("Jitter(-5) = %d, want -5", got)
	}
}

func TestRetry_SucceedsAfterRetry(t *testing.T) {
	t.Parallel()

	attempts := 0
	var slept []time.Duration
	var retryAttempts []int
	err := Retry(func(d time.Duration) { slept = append(slept, d) }, RetryConfig{
		MaxAttempts: 3,
		Delay:       func(attempt int) time.Duration { return time.Duration(attempt) * time.Millisecond },
		OnRetry: func(attempt int, delay time.Duration, err error) {
			retryAttempts = append(retryAttempts, attempt)
			if err == nil || !strings.Contains(err.Error(), "temporary") {
				t.Fatalf("retry error = %v, want previous temporary error", err)
			}
		},
	}, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Retry() returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if len(slept) != 2 || slept[0] != time.Millisecond || slept[1] != 2*time.Millisecond {
		t.Fatalf("slept = %v", slept)
	}
	if len(retryAttempts) != 2 || retryAttempts[0] != 1 || retryAttempts[1] != 2 {
		t.Fatalf("retry attempts = %v", retryAttempts)
	}
}

func TestRetry_StopsOnNonRetryableError(t *testing.T) {
	t.Parallel()

	attempts := 0
	wantErr := errors.New("fatal")
	err := Retry(func(time.Duration) { t.Fatal("must not sleep after non-retryable error") }, RetryConfig{
		MaxAttempts: 3,
		Delay:       func(int) time.Duration { return time.Millisecond },
		Retryable:   func(error) bool { return false },
	}, func() error {
		attempts++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Retry() error = %v, want %v", err, wantErr)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestWaitForExtensionActive(t *testing.T) {
	t.Parallel()

	attempts := 0
	var slept []time.Duration
	err := WaitForExtensionActive(func() (bool, error) {
		attempts++
		return attempts == 2, nil
	}, func(d time.Duration) { slept = append(slept, d) })
	if err != nil {
		t.Fatalf("WaitForExtensionActive() returned error: %v", err)
	}
	if len(slept) != 1 || slept[0] != ExtensionActiveRetryDelay {
		t.Fatalf("slept = %v, want one extension retry delay", slept)
	}
}

func TestWaitForExtensionActive_ReturnsLastError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("api failed")
	err := WaitForExtensionActive(func() (bool, error) { return false, wantErr }, func(time.Duration) {})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WaitForExtensionActive() error = %v, want %v", err, wantErr)
	}
}
