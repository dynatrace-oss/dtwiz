package grail

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

// selfMonitoringEventQuery returns a DQL query for dtwiz CUSTOM_INFO events.
// from is the test's own start time; using an absolute bound ensures events
// from a prior run (even with the same test_run marker) are never matched.
// The Events v2 title maps to event.name in Grail (not "title").
// Custom properties (step, test_run, command, etc.) are top-level fields.
// SelfMonitoringQuery holds the filter parameters for a self-monitoring event DQL query.
type SelfMonitoringQuery struct {
	EventName string
	Step      string
	TestRun   string
	From      time.Time // absolute lower bound — events before this are excluded
}

func selfMonitoringEventQuery(q SelfMonitoringQuery) string {
	fromLiteral := `"` + q.From.UTC().Format(time.RFC3339) + `"`
	return fmt.Sprintf(
		`fetch events, from: %s`+
			` | filter event.type == "CUSTOM_INFO"`+
			` | filter event.name == %q`+
			` | filter step == %q`+
			` | filter test_run == %q`+
			` | limit 10`,
		fromLiteral, q.EventName, q.Step, q.TestRun,
	)
}

// WaitForSelfMonitoringEvent polls until a dtwiz CUSTOM_INFO event matching q
// appears in Grail, or the timeout elapses.
func WaitForSelfMonitoringEvent(ctx context.Context, c *client.Client, q SelfMonitoringQuery, opts ...PollOption) ([]TraceRecord, error) {
	label := fmt.Sprintf("dtwiz event name=%q step=%q", q.EventName, q.Step)
	return waitForRecords(ctx, c, selfMonitoringEventQuery(q), label, opts...)
}

// RequireSelfMonitoringEvent calls WaitForSelfMonitoringEvent and fatals if
// no matching event is found within the configured timeout.
func RequireSelfMonitoringEvent(t *testing.T, c *client.Client, q SelfMonitoringQuery, opts ...PollOption) []TraceRecord {
	t.Helper()
	records, err := WaitForSelfMonitoringEvent(context.Background(), c, q, opts...)
	if err != nil {
		t.Fatalf("WaitForSelfMonitoringEvent(name=%q step=%q): %v", q.EventName, q.Step, err)
	}
	if len(records) == 0 {
		t.Fatalf("expected dtwiz event name=%q step=%q testRun=%q, got none", q.EventName, q.Step, q.TestRun)
	}
	return records
}
