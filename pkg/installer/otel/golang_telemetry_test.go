package otel

import (
	"errors"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func TestClassifyGoError(t *testing.T) {
	var dep *installer.DependencyMissingError
	var netErr *installer.NetworkError
	var failed *installer.InstallFailedError

	if err := classifyGoError("otelc-install", &GoVersionTooOldError{Required: "1.26.0"}); !errors.As(err, &dep) || dep.Name != "go>=1.26.0" {
		t.Errorf("too old Go = %#v, want dependency go>=1.26.0", err)
	}
	if err := classifyGoError("otelc-build", errors.New("dial tcp: lookup proxy.golang.org: no such host")); !errors.As(err, &netErr) || netErr.Reason != "otelc-build" {
		t.Errorf("network failure = %#v, want network error for otelc-build", err)
	}
	if err := classifyGoError("go-client-build", errors.New("syntax error")); !errors.As(err, &failed) || failed.Step != "go-client-build" {
		t.Errorf("build failure = %#v, want install failed at go-client-build", err)
	}
}

func TestExecute_RecordsGoInstrumentationOutcome(t *testing.T) {
	type errKind string
	const (
		noErr   errKind = ""
		depErr  errKind = "dependency"
		stepErr errKind = "install_failed"
	)

	tests := []struct {
		name    string
		setup   func(h *executeHarness, p *GoInstrumentationPlan)
		outcome installer.Outcome
		err     errKind
	}{
		{name: "all compile-time", outcome: installer.OutcomeSucceeded},
		{
			name: "otelc unavailable, services built client-only",
			setup: func(h *executeHarness, _ *GoInstrumentationPlan) {
				h.otelcErr = &GoVersionTooOldError{Required: "1.26.0"}
			},
			outcome: installer.OutcomeSucceeded, err: depErr,
		},
		{
			name:    "one otelc build fails, rescued by client-only",
			setup:   func(h *executeHarness, _ *GoInstrumentationPlan) { h.otelcFails["worker"] = true },
			outcome: installer.OutcomeSucceeded, err: stepErr,
		},
		{
			name: "everything fails",
			setup: func(h *executeHarness, _ *GoInstrumentationPlan) {
				h.otelcErr = errors.New("install failed")
				h.fallbackFails["api"] = true
				h.fallbackFails["worker"] = true
			},
			outcome: installer.OutcomeFailed, err: stepErr,
		},
		{
			name: "built, but no service could be started",
			setup: func(h *executeHarness, _ *GoInstrumentationPlan) {
				h.launchFails["api"], h.launchFails["worker"] = true, true
			},
			outcome: installer.OutcomeFailed, err: stepErr,
		},
		{
			name:    "one service fails to start, the other runs",
			setup:   func(h *executeHarness, _ *GoInstrumentationPlan) { h.launchFails["api"] = true },
			outcome: installer.OutcomeSucceeded,
		},
		{
			name:    "no Go toolchain",
			setup:   func(_ *executeHarness, p *GoInstrumentationPlan) { p.setManual(goReasonNoTool) },
			outcome: installer.OutcomeFailed, err: depErr,
		},
	}

	kind := func(err error) errKind {
		var dep *installer.DependencyMissingError
		var failed *installer.InstallFailedError
		switch {
		case err == nil:
			return noErr
		case errors.As(err, &dep):
			return depErr
		case errors.As(err, &failed):
			return stepErr
		}
		return errKind("other: " + err.Error())
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer.ResetInstallTelemetry()
			t.Cleanup(installer.ResetInstallTelemetry)
			h := newHarness(t)
			plan, _ := newTwoServicePlan(t)
			if tt.setup != nil {
				tt.setup(h, plan)
			}

			helpers.CaptureStdout(t, func() { _ = plan.Execute() })

			if got := installer.FeatureOutcomes()[installer.FeatureGoInstrumentation]; got != tt.outcome {
				t.Errorf("outcome = %v, want %v", got, tt.outcome)
			}
			if k := kind(installer.FeatureErrors()[installer.FeatureGoInstrumentation]); k != tt.err {
				t.Errorf("error = %q, want %q", k, tt.err)
			}
		})
	}
}
