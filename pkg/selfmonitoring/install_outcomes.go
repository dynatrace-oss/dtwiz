package selfmonitoring

import (
	"strconv"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// InstallReport is what an ist event reports about an install: the outcome of each feature
// the install tried to enable and the install work time. Callers pass the readable values;
// this package encodes them as the compact f= and d= User-Agent fields and as the readable
// install.* body properties.
type InstallReport struct {
	Features map[installer.Feature]installer.Outcome // features absent from the map were not tried
	Duration time.Duration                           // install work time, excluding prompts and the watch
}

// installFeatures lists the reported features in the fixed order of their characters in
// the f= field, with the body property of each. Positions are add-only: never reorder or
// reuse them, so a position keeps its meaning across versions. A shorter f= from an older
// version means the trailing features were unknown to it, which is not the same as "-"
// (not tried).
var installFeatures = []struct {
	feature installer.Feature
	prop    string
}{
	{installer.FeatureOtelConfig, "install.otel_config_written"},
	{installer.FeatureHostMonitoring, "install.host_monitoring"},
	{installer.FeatureRUM, "install.rum"},
	{installer.FeatureSynthetic, "install.synthetic_monitoring"},
	{installer.FeatureRDSExtensions, "install.rds_extensions"},
}

const (
	bodyInstallDurationMs = "install.duration_ms"

	// installDurationSecondsCap bounds d= to four digits. The body carries the unclamped
	// milliseconds, so clamping only degrades the fallback channel.
	installDurationSecondsCap = 9999
)

// featuresHeader encodes the outcomes as one character per feature: 1 succeeded,
// 0 failed, - not tried.
func (r InstallReport) featuresHeader() string {
	var b strings.Builder
	for _, f := range installFeatures {
		switch r.Features[f.feature] {
		case installer.OutcomeSucceeded:
			b.WriteByte('1')
		case installer.OutcomeFailed:
			b.WriteByte('0')
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// durationHeader encodes the work time in whole seconds, rounded down and clamped.
func (r InstallReport) durationHeader() string {
	secs := int64(r.Duration / time.Second)
	if secs > installDurationSecondsCap {
		secs = installDurationSecondsCap
	}
	return strconv.FormatInt(secs, 10)
}

// bodyProps returns the readable body properties: one per feature, always all of them,
// plus the unclamped work time in milliseconds.
func (r InstallReport) bodyProps() map[string]string {
	props := make(map[string]string, len(installFeatures)+1)
	for _, f := range installFeatures {
		switch r.Features[f.feature] {
		case installer.OutcomeSucceeded:
			props[f.prop] = "succeeded"
		case installer.OutcomeFailed:
			props[f.prop] = "failed"
		default:
			props[f.prop] = "not_tried"
		}
	}
	props[bodyInstallDurationMs] = strconv.FormatInt(r.Duration.Milliseconds(), 10)
	return props
}
