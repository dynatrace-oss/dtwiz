package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
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

	t.Run("one_event_per_presented_method", func(t *testing.T) {
		var captured []selfmonitoring.EventParams
		original := eventSink
		eventSink = func(p selfmonitoring.EventParams) { captured = append(captured, p) }
		defer func() { eventSink = original }()

		fireSetupMenuEvent(setupCmd, []recommender.Recommendation{k8sRec, awsRec, otelRec})

		if len(captured) != 3 {
			t.Fatalf("expected 3 events, got %d", len(captured))
		}
		wantOpts := []string{"kubernetes", "aws", "otel"}
		for i, want := range wantOpts {
			if captured[i].Opt != want {
				t.Errorf("event %d: opt = %q, want %q", i, captured[i].Opt, want)
			}
			if captured[i].Cmd != "setup" || captured[i].Sub != "" {
				t.Errorf("event %d: cmd/sub = %q/%q, want %q/%q", i, captured[i].Cmd, captured[i].Sub, "setup", "")
			}
			if captured[i].StepID != selfmonitoring.StepRecommendationsPresented {
				t.Errorf("event %d: step = %q, want %q", i, captured[i].StepID, selfmonitoring.StepRecommendationsPresented)
			}
		}
	})

	t.Run("no_recommendations_fires_nothing", func(t *testing.T) {
		var captured []selfmonitoring.EventParams
		original := eventSink
		eventSink = func(p selfmonitoring.EventParams) { captured = append(captured, p) }
		defer func() { eventSink = original }()

		fireSetupMenuEvent(setupCmd, nil)

		if len(captured) != 0 {
			t.Fatalf("expected no events, got %d", len(captured))
		}
	})
}

// captureInstallEvents replaces the event sink and resets the install telemetry state
// for the duration of the test.
func captureInstallEvents(t *testing.T) *[]selfmonitoring.EventParams {
	t.Helper()
	var captured []selfmonitoring.EventParams
	original := eventSink
	eventSink = func(p selfmonitoring.EventParams) { captured = append(captured, p) }
	installer.ResetInstallTelemetry()
	t.Cleanup(func() {
		eventSink = original
		installer.ResetInstallTelemetry()
	})
	return &captured
}

func singleEvent(t *testing.T, captured *[]selfmonitoring.EventParams) selfmonitoring.EventParams {
	t.Helper()
	if len(*captured) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(*captured))
	}
	return (*captured)[0]
}

func TestInstallEvent_CarriesRecordedOutcomesAndWorkTime(t *testing.T) {
	captured := captureInstallEvents(t)
	installer.StartInstallTimer()
	installer.RecordFeature(installer.FeatureOtelConfig, true)
	installer.RecordFeature(installer.FeatureHostMonitoring, false)

	fireInstallEvent(installOtelCollectorCmd, nil)

	p := singleEvent(t, captured)
	if p.StepID != selfmonitoring.StepInstall || p.Cmd != "install" || p.Sub != "otel-collector" {
		t.Errorf("step/cmd/sub = %q/%q/%q, want ist/install/otel-collector", p.StepID, p.Cmd, p.Sub)
	}
	if p.Install == nil {
		t.Fatal("Install report must be attached when install work started")
	}
	want := map[installer.Feature]installer.Outcome{
		installer.FeatureOtelConfig:     installer.OutcomeSucceeded,
		installer.FeatureHostMonitoring: installer.OutcomeFailed,
	}
	if len(p.Install.Features) != len(want) {
		t.Errorf("Features = %v, want %v", p.Install.Features, want)
	}
	for f, o := range want {
		if p.Install.Features[f] != o {
			t.Errorf("Features[%v] = %v, want %v", f, p.Install.Features[f], o)
		}
	}
	if p.Install.Duration < 0 {
		t.Errorf("Duration = %v, want a non-negative work time", p.Install.Duration)
	}
}

func TestInstallEvent_NoReportWhenNoWorkStarted(t *testing.T) {
	captured := captureInstallEvents(t)
	// Dry-run: the installer returned without confirming or doing anything.
	fireInstallEvent(installOtelCmd, nil)

	if p := singleEvent(t, captured); p.Install != nil {
		t.Errorf("Install = %+v, want nil for an install that did no work", p.Install)
	}
}

func TestInstallEvent_CancelledOmitsReportEvenWhenTimerStarted(t *testing.T) {
	captured := captureInstallEvents(t)
	// OneAgent starts the timer in the cmd layer and can still be declined at its update prompt.
	installer.StartInstallTimer()

	fireInstallEvent(installOneAgentCmd, installer.ErrInstallCancelled)

	p := singleEvent(t, captured)
	if p.Err != "user_cancelled" {
		t.Errorf("Err = %q, want user_cancelled", p.Err)
	}
	if p.Install != nil {
		t.Errorf("Install = %+v, want nil for a cancelled install", p.Install)
	}
}

func TestInstallEvent_FailureStillCarriesReport(t *testing.T) {
	captured := captureInstallEvents(t)
	installer.StartInstallTimer()
	installer.RecordFeature(installer.FeatureHostMonitoring, false)

	fireInstallEvent(installOneAgentCmd, &installer.InstallFailedError{Step: "download"})

	p := singleEvent(t, captured)
	if p.Err == "" {
		t.Error("Err must carry the classified error type")
	}
	if p.Install == nil || p.Install.Features[installer.FeatureHostMonitoring] != installer.OutcomeFailed {
		t.Errorf("Install = %+v, want a report with host monitoring failed", p.Install)
	}
	if p.ExtraProps["install.step"] != "download" {
		t.Errorf("error attributes must be kept next to the report, got %v", p.ExtraProps)
	}
}

func TestSetupInstallEvent_CarriesSameReportAsDirectInstall(t *testing.T) {
	captured := captureInstallEvents(t)
	installer.StartInstallTimer()
	installer.RecordFeature(installer.FeatureOtelConfig, true)
	installer.RecordFeature(installer.FeatureHostMonitoring, true)

	fireInstallEvent(installOtelCollectorCmd, nil)
	fireSetupInstallEvent(setupCmd, "otel-collector", nil)

	if len(*captured) != 2 {
		t.Fatalf("expected 2 events, got %d", len(*captured))
	}
	direct, setup := (*captured)[0], (*captured)[1]
	if setup.Cmd != "setup" || setup.Sub != "otel-collector" {
		t.Errorf("setup event cmd/sub = %q/%q", setup.Cmd, setup.Sub)
	}
	if setup.Install == nil || direct.Install == nil {
		t.Fatalf("both events must carry a report, got direct=%v setup=%v", direct.Install, setup.Install)
	}
	for f, o := range direct.Install.Features {
		if setup.Install.Features[f] != o {
			t.Errorf("setup Features[%v] = %v, direct = %v", f, setup.Install.Features[f], o)
		}
	}
	if len(setup.Install.Features) != len(direct.Install.Features) {
		t.Errorf("setup Features = %v, direct = %v", setup.Install.Features, direct.Install.Features)
	}
}

func TestSetupInstallEvent_FailureCarriesReport(t *testing.T) {
	captured := captureInstallEvents(t)
	installer.StartInstallTimer()
	installer.RecordFeature(installer.FeatureHostMonitoring, false)

	fireSetupInstallEvent(setupCmd, "oneagent", &installer.InstallFailedError{Step: "download"})

	p := singleEvent(t, captured)
	if p.Err == "" || p.Install == nil || p.ExtraProps["install.step"] != "download" {
		t.Errorf("unexpected failure event: err=%q install=%v props=%v", p.Err, p.Install, p.ExtraProps)
	}
}

func TestSetupInstallEvent_NoEventOnCancelOrUpToDate(t *testing.T) {
	for name, err := range map[string]error{
		"cancelled":  installer.ErrInstallCancelled,
		"up_to_date": otel.ErrUpToDate,
	} {
		t.Run(name, func(t *testing.T) {
			captured := captureInstallEvents(t)
			installer.StartInstallTimer()

			fireSetupInstallEvent(setupCmd, "otel-update", err)

			if len(*captured) != 0 {
				t.Errorf("expected no ist event, got %d", len(*captured))
			}
		})
	}
}

func TestSetupInstallEvent_NoReportOnDryRun(t *testing.T) {
	captured := captureInstallEvents(t)

	fireSetupInstallEvent(setupCmd, "kubernetes", nil)

	if p := singleEvent(t, captured); p.Install != nil {
		t.Errorf("Install = %+v, want nil for a dry-run", p.Install)
	}
}
