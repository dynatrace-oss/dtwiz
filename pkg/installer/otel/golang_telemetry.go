package otel

import (
	"errors"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// classifyGoError turns a Go instrumentation failure into a typed installer error so
// self-monitoring can report its category without messages or paths. step names the
// stage that failed (for example "otelc-build").
func classifyGoError(step string, err error) error {
	var tooOld *GoVersionTooOldError
	switch {
	case errors.As(err, &tooOld):
		return &installer.DependencyMissingError{Name: "go>=" + tooOld.Required}
	case isGoNetworkError(err.Error()):
		return &installer.NetworkError{Reason: step}
	default:
		return &installer.InstallFailedError{Step: step}
	}
}

// noteOtelcError remembers the first otelc failure (install or build). A later tier may
// rescue the service, so it is only reported when nothing ends up instrumented.
func (p *GoInstrumentationPlan) noteOtelcError(step string, err error) {
	if p.otelcErr == nil {
		p.otelcErr = classifyGoError(step, err)
	}
}

// noInstrumentationError explains why no service ended up instrumented, as a typed
// installer error so the failed event can classify it without messages or paths. It
// returns nil when nothing failed: a project handled with manual steps from the start
// (no toolchain, no automatic method) is a planned outcome, not a failure. When otelc
// failed first, its error is joined with the final cause, so both reach the event.
func (p *GoInstrumentationPlan) noInstrumentationError() error {
	if p.ManualReason != "" {
		return nil
	}
	cause := p.finalCause()
	if cause == nil {
		return nil
	}
	if p.otelcErr != nil {
		return errors.Join(p.otelcErr, cause)
	}
	return cause
}

// finalCause is the last stage that failed: a client-only build, the pre-flight or the launch.
func (p *GoInstrumentationPlan) finalCause() error {
	for _, s := range p.Services {
		if s.Err != nil {
			return classifyGoError("go-client-build", s.Err)
		}
		if s.Reason != "" {
			return &installer.InstallFailedError{Step: "go-preflight"}
		}
	}
	for _, s := range p.Services {
		if s.Method != goMethodManual {
			return &installer.InstallFailedError{Step: "go-launch"}
		}
	}
	return nil
}
