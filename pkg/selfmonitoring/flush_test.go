package selfmonitoring

import (
	"testing"
	"time"
)

func TestFlush_waitsForInFlightSends(t *testing.T) {
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
	// Register a send that will never complete.
	TrackSend()
	defer SendDone()

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
	start := time.Now()
	Flush(200 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("Flush with no sends took too long: %v", elapsed)
	}
}
