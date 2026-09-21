package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
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
			wantAttrs: map[string]string{"reason": "authentication_failed"},
		},
		{
			name:      "DependencyMissingError sets dependency attr",
			err:       &installer.DependencyMissingError{Name: "az"},
			wantErr:   string(installer.ErrTypeDependencyMissing),
			wantAttrs: map[string]string{"dependency": "az"},
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
			wantAttrs: map[string]string{"missing": "DT_ENVIRONMENT"},
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
