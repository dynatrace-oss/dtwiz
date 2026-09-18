//go:build integration

package e2e_test

import (
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
	"github.com/dynatrace-oss/dtwiz/test/integration"
	"github.com/dynatrace-oss/dtwiz/test/integration/grail"
)

// selfMonitoringCase describes one self-monitoring event to send and assert.
// wantFields lists fields to check on the returned Grail record (command,
// subcommand, etc.). Only fields the instrumentation derives independently
// are worth asserting on, not values we fabricate in the test.
type selfMonitoringCase struct {
	label      string
	params     selfmonitoring.EventParams
	wantFields map[string]string
}

// eventName derives the expected Grail event.name from a case's params,
// matching the title logic in selfmonitoring.SendEvent.
func eventName(tc selfMonitoringCase) string {
	name := "dtwiz"
	if tc.params.CmdID != "" {
		name += " " + tc.params.CmdID
	}
	if tc.params.SubID != "" {
		name += " " + tc.params.SubID
	}
	return name
}

// TestSelfMonitoringEventsIngested verifies that self-monitoring events for
// dtwiz watch and dtwiz setup are sent via the Events v2 API, appear in Grail,
// and carry the expected field values.
//
// To add coverage for a new command or step: append one entry to cases.
func TestSelfMonitoringEventsIngested(t *testing.T) {
	integration.Parallelize(t)
	env := integration.SetupIntegration(t)

	// Absolute lower bound for DQL queries — events older than this test run
	// cannot produce a false positive even if test_run is somehow reused.
	startTime := time.Now()

	classicURL := installer.APIURL(env.EnvURL)
	token := env.PlatformToken
	testRun := env.TestID

	cases := []selfMonitoringCase{
		// --- dtwiz watch ---
		// StepInvoked: fired at command start via fireInvokedEvent.
		{
			label:      "watch StepInvoked",
			params:     selfmonitoring.EventParams{CmdID: "wch", StepID: selfmonitoring.StepInvoked, Mode: "ntt"},
			wantFields: map[string]string{"command": "wch"},
		},
		// StepCompleted: fired by buildWatchEventCallback when first data arrives.
		// CmdID is cleared, signal timing goes into ExtraProps.
		{
			label: "watch StepCompleted",
			params: selfmonitoring.EventParams{
				StepID:     selfmonitoring.StepCompleted,
				Mode:       "ntt",
				Type:       "0,0,1,0,0,0,0,0",
				ExtraProps: map[string]string{"hosts": "850"},
			},
		},

		// --- dtwiz setup ---
		// StepInvoked, StepAnalyze, StepRecommend, StepInstall fired in sequence.
		{
			label:      "setup StepInvoked",
			params:     selfmonitoring.EventParams{CmdID: "set", StepID: selfmonitoring.StepInvoked, Mode: "ntt"},
			wantFields: map[string]string{"command": "set"},
		},
		{
			label:      "setup StepAnalyze",
			params:     selfmonitoring.EventParams{CmdID: "set", StepID: selfmonitoring.StepAnalyze, Mode: "ntt"},
			wantFields: map[string]string{"command": "set"},
		},
		{
			label:      "setup StepRecommend",
			params:     selfmonitoring.EventParams{CmdID: "set", SubID: "otel", StepID: selfmonitoring.StepRecommend, Mode: "ntt"},
			wantFields: map[string]string{"command": "set", "subcommand": "otel"},
		},
		{
			label:      "setup StepInstall",
			params:     selfmonitoring.EventParams{CmdID: "set", SubID: "otel", StepID: selfmonitoring.StepInstall, Mode: "ntt"},
			wantFields: map[string]string{"command": "set", "subcommand": "otel"},
		},
	}

	// Inject test_run into every case's ExtraProps.
	for i := range cases {
		if cases[i].params.ExtraProps == nil {
			cases[i].params.ExtraProps = map[string]string{}
		}
		cases[i].params.ExtraProps["test_run"] = testRun
	}

	// Phase 1: send all events before polling so Grail has the full set by the
	// time the first DQL query runs.
	for _, tc := range cases {
		if err := selfmonitoring.SendEvent(classicURL, token, tc.params); err != nil {
			t.Fatalf("SendEvent (%s): %v", tc.label, err)
		}
	}

	opts := []grail.PollOption{
		grail.WithTimeout(3 * time.Minute),
		grail.WithInterval(10 * time.Second),
	}

	// Phase 2: assert each event appears in Grail with the expected fields.
	for _, tc := range cases {
		step := tc.params.StepID
		if step == "" {
			step = selfmonitoring.StepInvoked
		}
		q := grail.SelfMonitoringQuery{
			EventName: eventName(tc),
			Step:      step,
			TestRun:   testRun,
			From:      startTime,
		}
		t.Logf("waiting for %s (event.name=%q step=%q)", tc.label, q.EventName, q.Step)

		records := grail.RequireSelfMonitoringEvent(t, env.Client, q, opts...)

		for field, want := range tc.wantFields {
			got, _ := records[0][field].(string)
			if got != want {
				t.Errorf("%s: field %q = %q, want %q", tc.label, field, got, want)
			}
		}
	}
}
