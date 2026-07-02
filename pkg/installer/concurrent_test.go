package installer

import (
	"errors"
	"fmt"
	"testing"
)

func TestRunConcurrently_AllSucceed(t *testing.T) {
	err := RunConcurrently(
		func() error { return nil },
		func() error { return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunConcurrently_JoinsAllErrors(t *testing.T) {
	errA := fmt.Errorf("error A")
	errB := fmt.Errorf("error B")

	err := RunConcurrently(
		func() error { return errA },
		func() error { return nil },
		func() error { return errB },
	)
	if err == nil {
		t.Fatal("expected joined error, got nil")
	}
	if !errors.Is(err, errA) {
		t.Errorf("expected joined error to include errA, got: %v", err)
	}
	if !errors.Is(err, errB) {
		t.Errorf("expected joined error to include errB, got: %v", err)
	}
}
