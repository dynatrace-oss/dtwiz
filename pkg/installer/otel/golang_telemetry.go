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

// noteTierError remembers the first failure of a tier (otelc install or build, client-only
// build) so it is reported even when a later tier rescues the service.
func (p *GoInstrumentationPlan) noteTierError(step string, err error) {
	if p.firstTierErr == nil {
		p.firstTierErr = classifyGoError(step, err)
	}
}

// recordGoInstrumentation reports the overall outcome. started is the number of
// instrumented services that are running after launch. It succeeded when at least
// one is, with the first tier failure (if any) as the error. Otherwise it failed,
// and the reason (manual steps, or a launch failure) is reported as the error.
func (p *GoInstrumentationPlan) recordGoInstrumentation(started int) {
	if started > 0 {
		installer.RecordFeature(installer.FeatureGoInstrumentation, true)
		installer.RecordFeatureError(installer.FeatureGoInstrumentation, p.firstTierErr)
		return
	}
	installer.RecordFeature(installer.FeatureGoInstrumentation, false)
	installer.RecordFeatureError(installer.FeatureGoInstrumentation, p.manualCause())
}

// manualCause explains why no service could be instrumented automatically.
func (p *GoInstrumentationPlan) manualCause() error {
	if p.ManualReason == goReasonNoTool {
		return &installer.DependencyMissingError{Name: "go"}
	}
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
	return &installer.InstallFailedError{Step: "go-manual"}
}
