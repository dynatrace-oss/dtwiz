package grail

import (
	"context"
	"fmt"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

func hostMetricsQuery(hostname string) string {
	// Query for any system.cpu.utilization data point from the given host in the
	// last five minutes. OTel host metrics are ingested via OTLP and are
	// queryable via the timeseries command using their original metric name.
	return fmt.Sprintf(
		`timeseries avg(system.cpu.utilization), from: now()-5m, by: {host.name}`+
			` | filter host.name == %q`+
			` | limit 1`,
		hostname,
	)
}

// WaitForHostMetrics polls the DQL endpoint until a system.cpu.utilization
// metric for hostname appears or the timeout elapses.
func WaitForHostMetrics(ctx context.Context, c *client.Client, hostname string, opts ...PollOption) ([]TraceRecord, error) {
	return waitForRecords(ctx, c, hostMetricsQuery(hostname), "host metrics for "+hostname, opts...)
}

// RequireHostMetrics calls WaitForHostMetrics and fatals if no records are found.
func RequireHostMetrics(t *testing.T, c *client.Client, hostname string, opts ...PollOption) []TraceRecord {
	t.Helper()
	records, err := WaitForHostMetrics(context.Background(), c, hostname, opts...)
	if err != nil {
		t.Fatalf("WaitForHostMetrics: %v", err)
	}
	if len(records) == 0 {
		t.Fatalf("expected host metrics for host %q, got none", hostname)
	}
	return records
}
