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

// A total failure must come back from Execute as a typed error so the command fires the
// failed event with its category. Partial results and planned manual outcomes succeed.
func TestExecute_ReturnsClassifiedErrorWhenNothingIsInstrumented(t *testing.T) {
	type errKind string
	const (
		noErr   errKind = ""
		depErr  errKind = "dependency"
		stepErr errKind = "install_failed"
	)

	tests := []struct {
		name  string
		setup func(h *executeHarness, p *GoInstrumentationPlan)
		want  errKind
	}{
		{name: "all compile-time"},
		{
			name: "otelc unavailable, services built client-only",
			setup: func(h *executeHarness, _ *GoInstrumentationPlan) {
				h.otelcErr = &GoVersionTooOldError{Required: "1.26.0"}
			},
		},
		{
			name:  "one otelc build fails, rescued by client-only",
			setup: func(h *executeHarness, _ *GoInstrumentationPlan) { h.otelcFails["worker"] = true },
		},
		{
			name: "everything fails to build",
			setup: func(h *executeHarness, _ *GoInstrumentationPlan) {
				h.otelcErr = errors.New("install failed")
				h.fallbackFails["api"] = true
				h.fallbackFails["worker"] = true
			},
			want: stepErr,
		},
		{
			name: "built, but no service could be started",
			setup: func(h *executeHarness, _ *GoInstrumentationPlan) {
				h.launchFails["api"], h.launchFails["worker"] = true, true
			},
			want: stepErr,
		},
		{
			name:  "one service fails to start, the other runs",
			setup: func(h *executeHarness, _ *GoInstrumentationPlan) { h.launchFails["api"] = true },
		},
		{
			name:  "no Go toolchain is a planned manual outcome",
			setup: func(_ *executeHarness, p *GoInstrumentationPlan) { p.setManual(goReasonNoTool) },
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
			h := newHarness(t)
			plan, _ := newTwoServicePlan(t)
			if tt.setup != nil {
				tt.setup(h, plan)
			}

			var err error
			helpers.CaptureStdout(t, func() { err = plan.Execute() })

			if got := kind(err); got != tt.want {
				t.Errorf("Execute() error = %q (%v), want %q", got, err, tt.want)
			}
		})
	}
}

// When otelc fails first and the client-only build then fails too, the error carries
// both, so classification and the message do not lose the otelc cause.
func TestExecute_TotalFailureKeepsOtelcAndClientOnlyErrors(t *testing.T) {
	h := newHarness(t)
	h.otelcErr = errors.New("dial tcp: lookup proxy.golang.org: no such host")
	h.fallbackFails["api"] = true
	h.fallbackFails["worker"] = true
	plan, _ := newTwoServicePlan(t)

	var err error
	helpers.CaptureStdout(t, func() { err = plan.Execute() })

	var netErr *installer.NetworkError
	if !errors.As(err, &netErr) || netErr.Reason != "otelc-install" {
		t.Errorf("otelc network failure missing from %v", err)
	}
	var failed *installer.InstallFailedError
	if !errors.As(err, &failed) || failed.Step != "go-client-build" {
		t.Errorf("client-only failure missing from %v", err)
	}
}
