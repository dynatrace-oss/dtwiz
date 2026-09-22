package grail

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

// stepFullNames maps the selfmonitoring shortcode constants to the full name
// stored in the event body by selfmonitoring.SendEvent. The DQL step filter
// must use the full name, not the shortcode.
var stepFullNames = map[string]string{
	"inv": "invoked",
	"ana": "analyze",
	"rec": "recommend",
	"ist": "install",
	"com": "completed",
}

// SelfMonitoringQuery holds the filter parameters for a self-monitoring event DQL query.
// EventName and TestRun are optional; when empty their filters are omitted.
// Step accepts either the selfmonitoring shortcode (e.g. "inv") or the full name
// (e.g. "invoked") — shortcodes are resolved internally.
// From is the absolute lower bound — events before this timestamp are excluded.
type SelfMonitoringQuery struct {
	EventName string    // optional: filter on event.name (exact match)
	Step      string    // required: selfmonitoring shortcode or full step name
	TestRun   string    // optional: filter on test_run property
	From      time.Time // required: absolute lower bound
}

func selfMonitoringEventQuery(q SelfMonitoringQuery) string {
	stepName := q.Step
	if full, ok := stepFullNames[q.Step]; ok {
		stepName = full
	}
	fromLiteral := `"` + q.From.UTC().Format(time.RFC3339) + `"`
	dql := fmt.Sprintf(
		`fetch events, from: %s | filter event.type == "CUSTOM_INFO" | filter step == %q`,
		fromLiteral, stepName,
	)
	if q.EventName != "" {
		dql += fmt.Sprintf(` | filter event.name == %q`, q.EventName)
	}
	if q.TestRun != "" {
		dql += fmt.Sprintf(` | filter test_run == %q`, q.TestRun)
	}
	dql += ` | limit 10`
	return dql
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
