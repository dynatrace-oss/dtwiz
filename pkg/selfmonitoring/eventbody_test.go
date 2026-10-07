package selfmonitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// Every step constant must map to a full name, otherwise SendEvent falls back to the
// shortcode and the event body carries e.g. "can" instead of "cancelled".
func TestStepFullNames(t *testing.T) {
	want := map[string]string{
		StepInvoked:                  "invoked",
		StepAnalyze:                  "analyze",
		StepInstall:                  "install",
		StepSnapshot:                 "snapshot",
		StepCompleted:                "completed",
		StepFailed:                   "failed",
		StepCancelled:                "cancelled",
		StepRecommendationsPresented: "recommendations_presented",
		StepRecommendationsSelected:  "recommendations_selected",
	}

	for code, name := range want {
		if got := stepFullNames[code]; got != name {
			t.Errorf("stepFullNames[%q] = %q, want %q", code, got, name)
		}
	}
	if len(stepFullNames) != len(want) {
		t.Errorf("stepFullNames has %d entries, want %d — a step constant is missing a full name", len(stepFullNames), len(want))
	}
}

func TestBuildEventPropsOpt(t *testing.T) {
	params := EventParams{
		Cmd:    "setup",
		StepID: StepRecommendationsPresented,
		Mode:   ModeTTY,
		Opt:    "otel",
	}
	props := buildEventProps(params)
	if props["option"] != "otel" {
		t.Errorf("body[option] = %q, want %q", props["option"], "otel")
	}
}

// Queries read the event body, so every field encoded in the abbreviated User-Agent must
// also appear in the body at full length. That invariant is what lets the header encoding
// change freely without any query being updated.
func TestEventBodyCarriesFullNamesForAllHeaderFields(t *testing.T) {
	params := EventParams{
		Cmd:    "uninstall",
		Sub:    "otel-collector",
		StepID: StepCancelled,
		Mode:   ModeTTY,
		Err:    string(installer.ErrTypePlatformUnsupported),
	}

	props := buildEventProps(params)
	ua := buildUserAgent(params)

	want := map[string]string{
		"command":    "uninstall",
		"subcommand": "otel-collector",
		"step":       "cancelled",
		"error":      "platform_unsupported",
	}
	for k, v := range want {
		if props[k] != v {
			t.Errorf("body[%q] = %q, want full name %q", k, props[k], v)
		}
		// The whole point: the header is abbreviated, the body is not.
		if strings.Contains(ua, v) {
			t.Errorf("User-Agent %q carries full-length %q; it should be abbreviated", ua, v)
		}
	}
	if strings.Contains(ua, "kd=") {
		t.Errorf("User-Agent %q carries kd= for a non-analyze step", ua)
	}
}

func TestBuildEventPropsStepSpecificFields(t *testing.T) {
	all := EventParams{
		Cmd:           "setup",
		Type:          "0,0,1",
		Opt:           "otel",
		CloudProvider: "aws",
		K8sDistro:     "GKE",
	}
	tests := []struct {
		step    string
		want    map[string]string
		wantNot []string
	}{
		{StepSnapshot, map[string]string{"type": "0,0,1"}, []string{"option", "cloud.provider", "k8s.distro"}},
		{StepAnalyze, map[string]string{"cloud.provider": "aws", "k8s.distro": "GKE"}, []string{"type", "option"}},
		{StepRecommendationsPresented, map[string]string{"option": "otel"}, []string{"type", "cloud.provider", "k8s.distro"}},
		{StepRecommendationsSelected, map[string]string{"option": "otel"}, []string{"type", "cloud.provider", "k8s.distro"}},
		{StepInstall, nil, []string{"type", "option", "cloud.provider", "k8s.distro"}},
	}
	for _, tt := range tests {
		t.Run(tt.step, func(t *testing.T) {
			p := all
			p.StepID = tt.step
			props := buildEventProps(p)
			for k, v := range tt.want {
				if props[k] != v {
					t.Errorf("body[%q] = %q, want %q", k, props[k], v)
				}
			}
			for _, k := range tt.wantNot {
				if _, ok := props[k]; ok {
					t.Errorf("body[%q] should be absent for step %q", k, tt.step)
				}
			}
		})
	}
}

// Watch snapshot events are the only ones carrying st=snp; the body must spell it out
// in full so queries filter on "snapshot" rather than the shortcode.
func TestSnapshotStepEncoding(t *testing.T) {
	params := EventParams{Cmd: "watch", StepID: StepSnapshot, Type: "0,0,1,0,0,0,0,8"}

	if got := buildEventProps(params)["step"]; got != "snapshot" {
		t.Errorf("body step = %q, want %q", got, "snapshot")
	}
	if ua := buildUserAgent(params); !strings.Contains(ua, ";st=snp") {
		t.Errorf("User-Agent %q missing %q", ua, ";st=snp")
	}
}

func TestBuildEventTitle(t *testing.T) {
	tests := []struct {
		name   string
		params EventParams
		want   string
	}{
		{"no_command", EventParams{}, "dtwiz"},
		{"command_only", EventParams{Cmd: "analyze"}, "dtwiz analyze"},
		{"command_and_subcommand", EventParams{Cmd: "install", Sub: "otel"}, "dtwiz install otel"},
		{"subcommand_only", EventParams{Sub: "otel"}, "dtwiz otel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildEventTitle(tt.params); got != tt.want {
				t.Errorf("buildEventTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The install report reaches the body as readable install.* properties, never under the
// header keys.
func TestBuildEventPropsInstallReport(t *testing.T) {
	props := buildEventProps(EventParams{
		Cmd:    "install",
		Sub:    "otel",
		StepID: StepInstall,
		Install: &InstallReport{
			Features: map[installer.Feature]installer.Outcome{installer.FeatureHostMonitoring: installer.OutcomeFailed},
			Duration: 42123 * time.Millisecond,
		},
	})

	want := map[string]string{
		"install.otel_config_written":  "not_tried",
		"install.host_monitoring":      "failed",
		"install.rum":                  "not_tried",
		"install.synthetic_monitoring": "not_tried",
		"install.rds_extensions":       "not_tried",
		"install.duration_ms":          "42123",
	}
	for k, v := range want {
		if props[k] != v {
			t.Errorf("body[%q] = %q, want %q", k, props[k], v)
		}
	}
	for _, k := range []string{"f", "d", "Install", "features", "duration"} {
		if _, ok := props[k]; ok {
			t.Errorf("body[%q] must be absent: header keys do not appear in the body", k)
		}
	}
}

func TestBuildEventPropsNoInstallReport(t *testing.T) {
	props := buildEventProps(EventParams{Cmd: "install", Sub: "otel", StepID: StepInstall, Err: "user_cancelled"})
	for k := range props {
		if strings.HasPrefix(k, "install.") {
			t.Errorf("body[%q] must be absent when no install work started", k)
		}
	}
}

func TestBuildEventPropsInstallReportOnlyOnInstallStep(t *testing.T) {
	props := buildEventProps(EventParams{Cmd: "install", StepID: StepFailed, Install: &InstallReport{}})
	for k := range props {
		if strings.HasPrefix(k, "install.") {
			t.Errorf("body[%q] must be absent on a non-install step", k)
		}
	}
}
