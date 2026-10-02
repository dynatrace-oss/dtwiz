package selfmonitoring

import (
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// Every step constant must map to a full name, otherwise SendEvent falls back to the
// shortcode and the event body carries e.g. "can" instead of "cancelled".
func TestStepFullNames(t *testing.T) {
	want := map[string]string{
		StepInvoked:                  "invoked",
		StepAnalyze:                  "analyze",
		StepRecommend:                "recommend",
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
		Type:   "retry",
	}

	props := buildEventProps(params)
	ua := buildUserAgent(params)

	want := map[string]string{
		"command":    "uninstall",
		"subcommand": "otel-collector",
		"step":       "cancelled",
		"error":      "platform_unsupported",
		"type":       "retry",
	}
	for k, v := range want {
		if props[k] != v {
			t.Errorf("body[%q] = %q, want full name %q", k, props[k], v)
		}
		// The whole point: the header is abbreviated, the body is not.
		// "type" full value may legitimately appear since it passes through
		// unabbreviated when no short form applies, so skip the header check for it.
		if k != "type" && strings.Contains(ua, v) {
			t.Errorf("User-Agent %q carries full-length %q; it should be abbreviated", ua, v)
		}
	}
	if strings.Contains(ua, "kd=") {
		t.Errorf("User-Agent %q carries kd= for a non-analyze step", ua)
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
