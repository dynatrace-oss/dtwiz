package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

const (
	GrailHttpPath string = "\"/platform/storage/query/v1/query:execute\""
)

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

	queryURL := c.Platform.BaseURL() + GrailHttpPath
	dql := tracesByServiceQuery(serviceName)

	deadline := time.Now().Add(cfg.timeout)
	for {
		records, err := executeDQL(ctx, queryURL, c.Platform.AuthHeader(), dql)
		if err == nil && len(records) > 0 {
			return records, nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("WaitForTraces: timeout waiting for traces of the service %q", serviceName)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("WaitForTraces: context cancelled while waiting for traces of the service %q", serviceName)
		case <-time.After(cfg.interval):
		}
	}
}

func executeDQL(ctx context.Context, queryURL, token, dql string) ([]TraceRecord, error) {
	payload := map[string]interface{}{
		"query":                      dql,
		"requestTimeoutMilliseconds": 10000,
		"maxResultRecords":           200,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("DQL query returned HTTP %d", resp.StatusCode)
	}

	var result dqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Result.Records, nil
}
