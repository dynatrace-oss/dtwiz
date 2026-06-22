package grail

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

// RequireTraces calls WaitForTraces and fatals if it errors or returns no records.
func RequireTraces(t *testing.T, c *client.Client, svcName string, opts ...PollOption) []TraceRecord {
	t.Helper()
	traces, err := WaitForTraces(context.Background(), c, svcName, opts...)
	if err != nil {
		t.Fatalf("WaitForTraces: %v", err)
	}
	if len(traces) == 0 {
		t.Fatalf("expected traces for service %q, got none", svcName)
	}
	return traces
}

// WaitForTraces polls the DQL endpoint via PlatformClient until traces for
// serviceName are found or the timeout is exceeded.
func WaitForTraces(ctx context.Context, c *client.Client, serviceName string, options ...PollOption) ([]TraceRecord, error) {
	records, err := waitForRecords(ctx, c, tracesByServiceQuery(serviceName), fmt.Sprintf("traces of service %q", serviceName), options...)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// waitForRecords runs dql repeatedly until it returns at least one record or the
// configured timeout elapses. label describes what is being awaited and is used
// only in error messages.
func waitForRecords(ctx context.Context, c *client.Client, dql, label string, options ...PollOption) ([]TraceRecord, error) {
	cfg := &pollConfig{
		timeout:  60 * time.Second,
		interval: 2 * time.Second,
	}
	for _, option := range options {
		option(cfg)
	}

	deadline := time.Now().Add(cfg.timeout)

	for {
		records, err := executeDQL(ctx, c.Platform, dql)
		if err != nil {
			return nil, fmt.Errorf("waitForRecords: error executing DQL query: %w", err)
		}
		if len(records) > 0 {
			return records, nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("waitForRecords: timeout waiting for %s", label)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waitForRecords: context cancelled waiting for %s", label)
		case <-time.After(cfg.interval):
		}
	}
}
