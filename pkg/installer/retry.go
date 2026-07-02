package installer

import (
	"math/rand"
	"time"
)

// RetryConfig parameterizes Retry: how many times to try, how long to wait
// between attempts, and which errors are worth retrying at all.
type RetryConfig struct {
	// MaxAttempts is the total number of attempts, including the first.
	MaxAttempts int
	// Delay returns the wait time before the given attempt (1-indexed: called
	// with 1 before the second attempt, 2 before the third, ...).
	Delay func(attempt int) time.Duration
	// Retryable decides whether a failed attempt's error is worth retrying.
	// A nil Retryable treats every error as retryable.
	Retryable func(err error) bool
	// OnRetry, if set, is called just before sleeping ahead of a retry —
	// useful for progress messages and debug logging.
	OnRetry func(attempt int, delay time.Duration, err error)
}

// Jitter adds up to 50% random extra wait on top of d. Fixed-delay retries against a
// cloud CLI/API decorrelate poorly under concurrent installs (e.g. a CI matrix) — every
// instance sleeps for exactly the same interval and retries in lockstep, turning a single
// propagation delay into a synchronized burst against the same endpoint. Jitter spreads
// those retries out instead.
func Jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	return d + time.Duration(rand.Int63n(int64(d)/2+1)) //nolint:gosec // non-cryptographic backoff jitter
}

// Retry runs op until it succeeds, cfg.Retryable rejects the latest error, or
// cfg.MaxAttempts is exhausted. It returns the last error seen (nil on success).
func Retry(sleeper func(time.Duration), cfg RetryConfig, op func() error) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := cfg.Delay(attempt)
			if cfg.OnRetry != nil {
				cfg.OnRetry(attempt, delay, lastErr)
			}
			sleeper(delay)
		}
		lastErr = op()
		if lastErr == nil {
			return nil
		}
		if cfg.Retryable != nil && !cfg.Retryable(lastErr) {
			return lastErr
		}
	}
	return lastErr
}
