package selfmonitoring

import (
	"context"
	"sync"
	"time"
)

var wg sync.WaitGroup

// TrackSend registers one in-flight send with the WaitGroup. Must be called before the goroutine is spawned.
func TrackSend() { wg.Add(1) }

// SendDone signals that one in-flight send has completed.
func SendDone() { wg.Done() }

// Flush waits for all in-flight self-monitoring sends to complete, or until timeout elapses.
// Silent on timeout — any remaining sends are accepted as lost.
func Flush(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}
