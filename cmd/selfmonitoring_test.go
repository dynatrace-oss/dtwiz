package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/recommender"
	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
)

func TestFireSelfMonitoringEventWithError_setsErrAndAttrs(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantErr   string
		wantAttrs map[string]string
	}{
		{
			name:      "AuthError sets err and reason attr",
			err:       &installer.AuthError{Reason: "authentication_failed"},
			wantErr:   string(installer.ErrTypeAuthError),
			wantAttrs: map[string]string{"auth.failure_reason": "authentication_failed"},
		},
		{
			name:      "DependencyMissingError sets dependency attr",
			err:       &installer.DependencyMissingError{Name: "az"},
			wantErr:   string(installer.ErrTypeDependencyMissing),
			wantAttrs: map[string]string{"dependency.name": "az"},
		},
		{
			name:    "ErrInstallCancelled maps to user_cancelled with no attrs",
			err:     installer.ErrInstallCancelled,
			wantErr: string(installer.ErrTypeUserCancelled),
		},
		{
			name:      "ConfigError sets missing attr",
			err:       &installer.ConfigError{MissingFields: []string{"DT_ENVIRONMENT"}},
			wantErr:   string(installer.ErrTypeConfigError),
			wantAttrs: map[string]string{"config.missing_fields": "DT_ENVIRONMENT"},
		},
		{
			name:    "unknown error falls back to install_failed",
			err:     errors.New("something went wrong"),
			wantErr: string(installer.ErrTypeInstallFailed),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured selfmonitoring.EventParams
			original := eventSink
			eventSink = func(p selfmonitoring.EventParams) { captured = p }
			defer func() { eventSink = original }()

			params := selfmonitoring.EventParams{Cmd: "ins", StepID: selfmonitoring.StepFailed}
			fireSelfMonitoringEventWithError(params, tt.err)

			if captured.Err != tt.wantErr {
				t.Errorf("params.Err = %q, want %q", captured.Err, tt.wantErr)
			}
			for k, wantV := range tt.wantAttrs {
				if gotV := captured.ExtraProps[k]; gotV != wantV {
					t.Errorf("ExtraProps[%q] = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

// A missing or rejected token must not stop the event: the request still reaches the
// tenant's HAProxy, whose User-Agent capture is what makes auth failures observable.
// Only an unknown tenant URL is fatal, because there is no destination.
func TestEventSink_sendsEvenWithoutToken(t *testing.T) {
	type capture struct {
		auth string
		ua   string
	}
	got := make(chan capture, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- capture{auth: r.Header.Get("Authorization"), ua: r.Header.Get("User-Agent")}
		w.WriteHeader(http.StatusUnauthorized) // tenant rejects the missing token
	}))
	defer srv.Close()

	withCleanCredentialFlags(t)
	t.Setenv("DTWIZ_SELF_MONITORING_POC", "true")
	t.Setenv("DT_ENVIRONMENT", srv.URL)
	t.Setenv("DT_PLATFORM_TOKEN", "")

	eventSink(selfmonitoring.EventParams{
		Cmd:    "install",
		StepID: selfmonitoring.StepFailed,
		Err:    string(installer.ErrTypeAuthError),
	})
	selfmonitoring.Flush(5 * time.Second)

	select {
	case c := <-got:
		if !strings.Contains(c.ua, "er=aut") {
			t.Errorf("User-Agent = %q, want it to carry er=aut", c.ua)
		}
	default:
		t.Fatal("event was dropped, but the tenant URL was known — the attempt should have been sent")
	}
}

func TestEventSink_dropsEventWhenTenantUnknown(t *testing.T) {
	withCleanCredentialFlags(t)
	t.Setenv("DTWIZ_SELF_MONITORING_POC", "true")
	t.Setenv("DT_ENVIRONMENT", "")
	t.Setenv("DT_PLATFORM_TOKEN", "dt0s16.doesnotmatter")

	// Nothing to assert beyond "does not panic and does not hang": with no URL there is
	// no destination, so the event is intentionally discarded.
	eventSink(selfmonitoring.EventParams{Cmd: "install", StepID: selfmonitoring.StepFailed})
	selfmonitoring.Flush(5 * time.Second)
}

// withCleanCredentialFlags clears the CLI flag vars so env vars are the only source.
func withCleanCredentialFlags(t *testing.T) {
	t.Helper()
	origEnv, origTok := environmentFlag, platformTokenFlag
	environmentFlag, platformTokenFlag = "", ""
	t.Cleanup(func() { environmentFlag, platformTokenFlag = origEnv, origTok })
}

func TestFireSetupMenuEvent(t *testing.T) {
	k8sRec := recommender.Recommendation{Method: recommender.MethodKubernetes}
	awsRec := recommender.Recommendation{Method: recommender.MethodAWS}
	otelRec := recommender.Recommendation{Method: recommender.MethodOtelCollector}

	t.Run("no_techs_fires_one_event_per_method", func(t *testing.T) {
		var captured []selfmonitoring.EventParams
		original := eventSink
		eventSink = func(p selfmonitoring.EventParams) { captured = append(captured, p) }
		defer func() { eventSink = original }()

		fireSetupMenuEvent(nil, []recommender.Recommendation{k8sRec, awsRec}, nil)

		if len(captured) != 2 {
			t.Fatalf("expected 2 events, got %d", len(captured))
		}
		if captured[0].Opt != "kubernetes" || captured[1].Opt != "aws" {
			t.Errorf("unexpected opt values: %q, %q", captured[0].Opt, captured[1].Opt)
		}
		for _, p := range captured {
			if p.StepID != selfmonitoring.StepRecommendationsPresented {
				t.Errorf("expected step %q, got %q", selfmonitoring.StepRecommendationsPresented, p.StepID)
			}
			if p.Tech != "" {
				t.Errorf("expected no tech for non-otel method, got %q", p.Tech)
			}
		}
	})

	t.Run("otel_with_two_techs_fires_two_events", func(t *testing.T) {
		var captured []selfmonitoring.EventParams
		original := eventSink
		eventSink = func(p selfmonitoring.EventParams) { captured = append(captured, p) }
		defer func() { eventSink = original }()

		fireSetupMenuEvent(nil, []recommender.Recommendation{otelRec}, []string{"Node.js", "Python"})

		if len(captured) != 2 {
			t.Fatalf("expected 2 events for otel with 2 techs, got %d", len(captured))
		}
		if captured[0].Tech != "Node.js" || captured[1].Tech != "Python" {
			t.Errorf("unexpected tech values: %q, %q", captured[0].Tech, captured[1].Tech)
		}
		for _, p := range captured {
			if p.Opt != "otel" {
				t.Errorf("expected opt=otel, got %q", p.Opt)
			}
			if p.StepID != selfmonitoring.StepRecommendationsPresented {
				t.Errorf("expected step %q, got %q", selfmonitoring.StepRecommendationsPresented, p.StepID)
			}
		}
	})

	t.Run("mixed_methods_correct_event_count", func(t *testing.T) {
		var captured []selfmonitoring.EventParams
		original := eventSink
		eventSink = func(p selfmonitoring.EventParams) { captured = append(captured, p) }
		defer func() { eventSink = original }()

		// k8s + otel with 2 techs → 1 + 2 = 3 events
		fireSetupMenuEvent(nil, []recommender.Recommendation{k8sRec, otelRec}, []string{"Node.js", "Python"})

		if len(captured) != 3 {
			t.Fatalf("expected 3 events (1 for k8s + 2 for otel), got %d", len(captured))
		}
		if captured[0].Opt != "kubernetes" {
			t.Errorf("first event should be kubernetes, got %q", captured[0].Opt)
		}
		if captured[1].Opt != "otel" || captured[2].Opt != "otel" {
			t.Errorf("events 2 and 3 should be otel, got %q, %q", captured[1].Opt, captured[2].Opt)
		}
	})

	t.Run("non_otel_does_not_carry_techs_even_when_present", func(t *testing.T) {
		var captured []selfmonitoring.EventParams
		original := eventSink
		eventSink = func(p selfmonitoring.EventParams) { captured = append(captured, p) }
		defer func() { eventSink = original }()

		fireSetupMenuEvent(nil, []recommender.Recommendation{k8sRec}, []string{"Node.js"})

		if len(captured) != 1 {
			t.Fatalf("expected 1 event, got %d", len(captured))
		}
		if captured[0].Tech != "" {
			t.Errorf("k8s event should have no tech, got %q", captured[0].Tech)
		}
	})
}
