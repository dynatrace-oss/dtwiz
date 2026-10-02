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

func TestInstallEvent_FeatureOutcomes(t *testing.T) {
	tests := []struct {
		name         string
		record       func()
		wantFeatures string
		wantBody     map[string]string
	}{
		{
			name: "otel_config_and_host_monitoring_succeeded",
			record: func() {
				installer.RecordFeature(installer.FeatureOtelConfig, true)
				installer.RecordFeature(installer.FeatureHostMonitoring, true)
			},
			wantFeatures: "11---",
			wantBody: map[string]string{
				"install.otel_config_written":  "succeeded",
				"install.host_monitoring":      "succeeded",
				"install.rum":                  "not_tried",
				"install.synthetic_monitoring": "not_tried",
				"install.rds_extensions":       "not_tried",
			},
		},
		{
			name: "host_monitoring_failed_install_succeeded",
			record: func() {
				installer.RecordFeature(installer.FeatureOtelConfig, true)
				installer.RecordFeature(installer.FeatureHostMonitoring, false)
			},
			wantFeatures: "10---",
			wantBody:     map[string]string{"install.otel_config_written": "succeeded", "install.host_monitoring": "failed"},
		},
		{
			name:         "failed_before_reaching_config_write",
			record:       func() { installer.RecordFeature(installer.FeatureHostMonitoring, true) },
			wantFeatures: "-1---",
			wantBody:     map[string]string{"install.otel_config_written": "not_tried", "install.host_monitoring": "succeeded"},
		},
		{
			name:         "nothing_tried",
			record:       func() {},
			wantFeatures: "-----",
			wantBody:     map[string]string{"install.otel_config_written": "not_tried", "install.host_monitoring": "not_tried"},
		},
		{
			name: "reserved_features_stay_not_tried_even_when_recorded_failed_elsewhere",
			record: func() {
				installer.RecordFeature(installer.FeatureOtelConfig, true)
			},
			wantFeatures: "1----",
			wantBody:     map[string]string{"install.rum": "not_tried", "install.synthetic_monitoring": "not_tried", "install.rds_extensions": "not_tried"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captured := captureInstallEvents(t)
			installer.StartInstallTimer()
			tt.record()

			fireInstallEvent(installOtelCollectorCmd, nil)

			p := singleEvent(t, captured)
			if p.StepID != selfmonitoring.StepInstall {
				t.Errorf("step = %q, want %q", p.StepID, selfmonitoring.StepInstall)
			}
			if p.Features != tt.wantFeatures {
				t.Errorf("Features = %q, want %q", p.Features, tt.wantFeatures)
			}
			if p.DurationS == "" {
				t.Error("DurationS must be present when install work started")
			}
			if p.ExtraProps["install.duration_ms"] == "" {
				t.Error("body install.duration_ms must be present when install work started")
			}
			for k, want := range tt.wantBody {
				if got := p.ExtraProps[k]; got != want {
					t.Errorf("body[%q] = %q, want %q", k, got, want)
				}
			}
			if len(p.ExtraProps) != len(installFeatures)+1 {
				t.Errorf("body has %d install properties, want all %d features plus the duration: %v", len(p.ExtraProps), len(installFeatures), p.ExtraProps)
			}
		})
	}
}

func TestInstallEvent_NoOutcomesWhenNoWorkStarted(t *testing.T) {
	captured := captureInstallEvents(t)
	// Dry-run: the installer returned without confirming or doing anything.
	fireInstallEvent(installOtelCmd, nil)

	p := singleEvent(t, captured)
	if p.Features != "" || p.DurationS != "" {
		t.Errorf("Features/DurationS = %q/%q, want both empty", p.Features, p.DurationS)
	}
	for k := range p.ExtraProps {
		t.Errorf("unexpected body property %q for an install that did no work", k)
	}
}

func TestInstallEvent_CancelledOmitsOutcomesEvenWhenTimerStarted(t *testing.T) {
	captured := captureInstallEvents(t)
	// OneAgent starts the timer in the cmd layer and can still be declined at its update prompt.
	installer.StartInstallTimer()

	fireInstallEvent(installOneAgentCmd, installer.ErrInstallCancelled)

	p := singleEvent(t, captured)
	if p.Err != "user_cancelled" {
		t.Errorf("Err = %q, want user_cancelled", p.Err)
	}
	if p.Features != "" || p.DurationS != "" || len(p.ExtraProps) != 0 {
		t.Errorf("a cancelled install must carry no outcomes, got Features=%q DurationS=%q props=%v", p.Features, p.DurationS, p.ExtraProps)
	}
}

func TestInstallEvent_FailureStillCarriesWorkTimeAndOutcomes(t *testing.T) {
	captured := captureInstallEvents(t)
	installer.StartInstallTimer()
	installer.RecordFeature(installer.FeatureHostMonitoring, false)

	fireInstallEvent(installOneAgentCmd, &installer.InstallFailedError{Step: "download"})

	p := singleEvent(t, captured)
	if p.Err == "" {
		t.Error("Err must carry the classified error type")
	}
	if p.Features != "-0---" || p.DurationS == "" {
		t.Errorf("Features/DurationS = %q/%q, want -0---/non-empty", p.Features, p.DurationS)
	}
	if p.ExtraProps["install.step"] != "download" {
		t.Errorf("error attributes must be kept next to the outcomes, got %v", p.ExtraProps)
	}
}

func TestInstallOutcomeFields_ClampsHeaderOnly(t *testing.T) {
	features, durationS, body := installOutcomeFields(3*time.Hour, nil)

	if features != "-----" {
		t.Errorf("features = %q, want -----", features)
	}
	if durationS != "9999" {
		t.Errorf("durationS = %q, want 9999 (clamped)", durationS)
	}
	if body["install.duration_ms"] != "10800000" {
		t.Errorf("body install.duration_ms = %q, want the unclamped 10800000", body["install.duration_ms"])
	}
}

func TestInstallOutcomeFields_RoundsDownToWholeSeconds(t *testing.T) {
	_, durationS, body := installOutcomeFields(2999*time.Millisecond, nil)

	if durationS != "2" {
		t.Errorf("durationS = %q, want 2", durationS)
	}
	if body["install.duration_ms"] != "2999" {
		t.Errorf("body install.duration_ms = %q, want 2999", body["install.duration_ms"])
	}
}

func TestInstallFeatures_PositionsAreStable(t *testing.T) {
	// Positions in f= are a contract with whoever queries the data: add-only.
	want := []installer.Feature{
		installer.FeatureOtelConfig,
		installer.FeatureHostMonitoring,
		installer.FeatureRUM,
		installer.FeatureSynthetic,
		installer.FeatureRDSExtensions,
	}
	if len(installFeatures) != len(want) {
		t.Fatalf("installFeatures has %d entries, want %d", len(installFeatures), len(want))
	}
	for i, f := range want {
		if installFeatures[i].feature != f {
			t.Errorf("position %d holds feature %v, want %v", i+1, installFeatures[i].feature, f)
		}
	}
}

func TestSetupInstallEvent_CarriesSameOutcomesAsDirectInstall(t *testing.T) {
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
	if setup.Features != direct.Features || setup.Features != "11---" {
		t.Errorf("setup Features = %q, direct = %q, want both 11---", setup.Features, direct.Features)
	}
	for k, v := range direct.ExtraProps {
		if k == "install.duration_ms" {
			continue // differs by the time elapsed between the two calls
		}
		if setup.ExtraProps[k] != v {
			t.Errorf("setup body[%q] = %q, direct = %q", k, setup.ExtraProps[k], v)
		}
	}
	if setup.ExtraProps["install.duration_ms"] == "" || setup.DurationS == "" {
		t.Error("setup event must carry the work time")
	}
}

func TestSetupInstallEvent_FailureCarriesOutcomes(t *testing.T) {
	captured := captureInstallEvents(t)
	installer.StartInstallTimer()
	installer.RecordFeature(installer.FeatureHostMonitoring, false)

	fireSetupInstallEvent(setupCmd, "oneagent", &installer.InstallFailedError{Step: "download"})

	p := singleEvent(t, captured)
	if p.Err == "" || p.Features != "-0---" || p.ExtraProps["install.step"] != "download" {
		t.Errorf("unexpected failure event: err=%q features=%q props=%v", p.Err, p.Features, p.ExtraProps)
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

func TestSetupInstallEvent_NoOutcomesOnDryRun(t *testing.T) {
	captured := captureInstallEvents(t)

	fireSetupInstallEvent(setupCmd, "kubernetes", nil)

	p := singleEvent(t, captured)
	if p.Features != "" || p.DurationS != "" || len(p.ExtraProps) != 0 {
		t.Errorf("a dry-run must carry no outcomes, got Features=%q DurationS=%q props=%v", p.Features, p.DurationS, p.ExtraProps)
	}
}
