package selfmonitoring

import (
	"sync"
	"testing"
	"time"
)

func TestFlush_waitsForInFlightSends(t *testing.T) {
	// Reset the package-level WaitGroup for this test.
	wg = sync.WaitGroup{}

	ready := make(chan struct{})
	TrackSend()
	go func() {
		<-ready
		SendDone()
	}()

	// Flush should block until we unblock the goroutine.
	done := make(chan struct{})
	go func() {
		close(ready)
		Flush(500 * time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Flush did not return after send completed")
	}
}

func TestFlush_timeoutRespected(t *testing.T) {
	wg = sync.WaitGroup{}

	// Register a send that will never complete.
	wg.Add(1)
	defer wg.Done()

	start := time.Now()
	Flush(50 * time.Millisecond)
	elapsed := time.Since(start)

	if elapsed < 50*time.Millisecond {
		t.Errorf("Flush returned too early: %v", elapsed)
	}
	// Give a generous upper bound; CI can be slow.
	if elapsed > 500*time.Millisecond {
		t.Errorf("Flush took too long: %v", elapsed)
	}
}

func TestFlush_returnsImmediatelyWhenNoSends(t *testing.T) {
	wg = sync.WaitGroup{}

	start := time.Now()
	Flush(200 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("Flush with no sends took too long: %v", elapsed)
	}
}
