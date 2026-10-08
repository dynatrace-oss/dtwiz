package selfmonitoring

import (
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

func TestInstallReport_FeaturesHeader(t *testing.T) {
	ok, failed := installer.OutcomeSucceeded, installer.OutcomeFailed
	tests := []struct {
		name     string
		features map[installer.Feature]installer.Outcome
		want     string
	}{
		{"nothing_tried", nil, "-----"},
		{"otel_config_and_host_monitoring_succeeded", map[installer.Feature]installer.Outcome{installer.FeatureOtelConfig: ok, installer.FeatureHostMonitoring: ok}, "11---"},
		{"host_monitoring_failed", map[installer.Feature]installer.Outcome{installer.FeatureOtelConfig: ok, installer.FeatureHostMonitoring: failed}, "10---"},
		{"only_host_monitoring_reached", map[installer.Feature]installer.Outcome{installer.FeatureHostMonitoring: ok}, "-1---"},
		{"each_reserved_feature_in_its_own_position", map[installer.Feature]installer.Outcome{installer.FeatureRUM: ok, installer.FeatureSynthetic: failed, installer.FeatureRDSExtensions: ok}, "--101"},
		{"explicit_not_tried_outcome", map[installer.Feature]installer.Outcome{installer.FeatureOtelConfig: installer.OutcomeNotTried}, "-----"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (InstallReport{Features: tt.features}).featuresHeader(); got != tt.want {
				t.Errorf("featuresHeader() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstallReport_BodyPropsAreReadableAndComplete(t *testing.T) {
	props := InstallReport{
		Features: map[installer.Feature]installer.Outcome{
			installer.FeatureOtelConfig:     installer.OutcomeSucceeded,
			installer.FeatureHostMonitoring: installer.OutcomeFailed,
		},
		Duration: 1500 * time.Millisecond,
	}.bodyProps()

	want := map[string]string{
		"install.otel_config_written":  "succeeded",
		"install.host_monitoring":      "failed",
		"install.rum":                  "not_tried",
		"install.synthetic_monitoring": "not_tried",
		"install.rds_extensions":       "not_tried",
		"install.duration_ms":          "1500",
	}
	if len(props) != len(want) {
		t.Errorf("body has %d properties, want all %d features plus the duration: %v", len(props), len(want), props)
	}
	for k, v := range want {
		if props[k] != v {
			t.Errorf("body[%q] = %q, want %q", k, props[k], v)
		}
	}
}

// The header and the body describe the same outcomes: each position in f= and each body
// property must agree for every feature and outcome.
func TestInstallReport_HeaderAndBodyAgree(t *testing.T) {
	headerChar := map[installer.Outcome]string{installer.OutcomeSucceeded: "1", installer.OutcomeFailed: "0", installer.OutcomeNotTried: "-"}
	bodyValue := map[installer.Outcome]string{installer.OutcomeSucceeded: "succeeded", installer.OutcomeFailed: "failed", installer.OutcomeNotTried: "not_tried"}

	for i, f := range installFeatures {
		for outcome := range headerChar {
			r := InstallReport{Features: map[installer.Feature]installer.Outcome{f.feature: outcome}}
			if got := string(r.featuresHeader()[i]); got != headerChar[outcome] {
				t.Errorf("feature %s outcome %v: header position %d = %q, want %q", f.prop, outcome, i+1, got, headerChar[outcome])
			}
			if got := r.bodyProps()[f.prop]; got != bodyValue[outcome] {
				t.Errorf("feature %s outcome %v: body = %q, want %q", f.prop, outcome, got, bodyValue[outcome])
			}
		}
	}
}

// Positions in f= are a contract with whoever queries the data: add-only. Adding a feature
// means appending here; this test must keep passing unchanged for the existing ones.
func TestInstallFeatures_PositionsAreStable(t *testing.T) {
	want := []installer.Feature{
		installer.FeatureOtelConfig,
		installer.FeatureHostMonitoring,
		installer.FeatureRUM,
		installer.FeatureSynthetic,
		installer.FeatureRDSExtensions,
	}
	if len(installFeatures) < len(want) {
		t.Fatalf("installFeatures has %d entries, fewer than the %d shipped positions", len(installFeatures), len(want))
	}
	for i, f := range want {
		if installFeatures[i].feature != f {
			t.Errorf("position %d holds feature %v, want %v", i+1, installFeatures[i].feature, f)
		}
	}
}

func TestInstallReport_DurationHeaderRoundsDownToWholeSeconds(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0"},
		{999 * time.Millisecond, "0"},
		{2999 * time.Millisecond, "2"},
		{42 * time.Second, "42"},
	}
	for _, tt := range tests {
		if got := (InstallReport{Duration: tt.d}).durationHeader(); got != tt.want {
			t.Errorf("durationHeader(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestInstallReport_DurationClampedInHeaderOnly(t *testing.T) {
	r := InstallReport{Duration: 3 * time.Hour}

	if got := r.durationHeader(); got != "9999" {
		t.Errorf("durationHeader() = %q, want 9999 (clamped)", got)
	}
	if got := r.bodyProps()["install.duration_ms"]; got != "10800000" {
		t.Errorf("body install.duration_ms = %q, want the unclamped 10800000", got)
	}
}
