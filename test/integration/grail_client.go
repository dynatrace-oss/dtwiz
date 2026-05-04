package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

// TraceRecord is a single record returned by a DQL trace query.
type TraceRecord map[string]interface{}

// PollOption configures the polling behavior of WaitForTraces.
type PollOption func(*pollConfig)

type pollConfig struct {
	timeout  time.Duration
	interval time.Duration
}

type dqlResponse struct {
	State  string `json:"state"`
	Result struct {
		Records []TraceRecord `json:"records"`
	} `json:"result"`
}

// WithTimeout overrides the default 60s polling timeout.
func WithTimeout(d time.Duration) PollOption {
	return func(c *pollConfig) { c.timeout = d }
}

// WithInterval overrides the default 2s poll interval.
func WithInterval(d time.Duration) PollOption {
	return func(c *pollConfig) { c.interval = d }
}

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

const grailHTTPPath = "/platform/storage/query/v1/query:execute"

// WaitForTraces polls the DQL endpoint via PlatformClient until traces for
// serviceName are found or the timeout is exceeded. It returns the matching
// trace records or an error that includes the service name.
func WaitForTraces(ctx context.Context, c *client.Client, serviceName string, options ...PollOption) ([]TraceRecord, error) {
	cfg := &pollConfig{
		timeout:  60 * time.Second,
		interval: 2 * time.Second,
	}
	for _, option := range options {
		option(cfg)
	}

	dql := tracesByServiceQuery(serviceName)

	deadline := time.Now().Add(cfg.timeout)
	var lastErr error
	for {
		records, err := executeDQL(ctx, c.Platform, dql)
		if err != nil {
			lastErr = err
		} else if len(records) > 0 {
			return records, nil
		}

		if time.Now().After(deadline) {
			if lastErr != nil {
				return nil, fmt.Errorf("WaitForTraces: timeout waiting for traces of service %q: last error: %w", serviceName, lastErr)
			}
			return nil, fmt.Errorf("WaitForTraces: timeout waiting for traces of service %q", serviceName)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("WaitForTraces: context cancelled waiting for traces of service %q", serviceName)
		case <-time.After(cfg.interval):
		}
	}
}

func executeDQL(ctx context.Context, platform *client.PlatformClient, dql string) ([]TraceRecord, error) {
	payload := map[string]interface{}{
		"query":                      dql,
		"requestTimeoutMilliseconds": 10000,
		"maxResultRecords":           200,
	}

	var result dqlResponse
	resp, err := platform.HTTP().R().
		SetContext(ctx).
		SetBody(payload).
		SetResult(&result).
		Post(grailHTTPPath)
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("DQL query returned HTTP %d", resp.StatusCode())
	}

	switch result.State {
	case "SUCCEEDED":
		return result.Result.Records, nil
	case "RUNNING":
		return nil, nil
	default:
		return nil, fmt.Errorf("DQL query in unexpected state %q", result.State)
	}
}
