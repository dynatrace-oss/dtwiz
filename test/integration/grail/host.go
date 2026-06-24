package grail

import (
	"context"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

// RequireHost calls WaitForHost and fatals if it errors or returns no records.
func RequireHost(t *testing.T, c *client.Client, hostName string, opts ...PollOption) []TraceRecord {
	t.Helper()
	hosts, err := WaitForHost(context.Background(), c, hostName, opts...)
	if err != nil {
		t.Fatalf("WaitForHost: %v", err)
	}
	if len(hosts) == 0 {
		t.Fatalf("expected host %q to appear in topology, got none", hostName)
	}
	return hosts
}

// WaitForHost polls the DQL endpoint via PlatformClient until a HOST entity
// named hostName appears in the Smartscape topology or the timeout is exceeded.
// It is used to confirm that a freshly installed OneAgent has connected to the
// tenant and registered its host.
func WaitForHost(ctx context.Context, c *client.Client, hostName string, options ...PollOption) ([]TraceRecord, error) {
	return waitForRecords(ctx, c, hostByNameQuery(hostName), "host "+hostName+" in topology", options...)
}
