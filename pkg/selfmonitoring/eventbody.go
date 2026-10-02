package selfmonitoring

import (
	"maps"
	"runtime"

	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

// stepFullNames maps step shortcodes to human-readable names used in the event body.
var stepFullNames = map[string]string{
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

type eventPayload struct {
	EventType  string            `json:"eventType"`
	Title      string            `json:"title"`
	Properties map[string]string `json:"properties"`
}

// buildEventProps assembles the event body properties. Unlike the User-Agent, these carry
// full, unabbreviated values: the body is the query surface, so its encoding stays stable
// while the header is free to be shortened to fit the capture limit.
// Callers must resolve an empty StepID to its default first.
func buildEventProps(p EventParams) map[string]string {
	stepFull := stepFullNames[p.StepID]
	if stepFull == "" {
		stepFull = p.StepID
	}

	props := map[string]string{
		"executionId": execID,
		"step":        stepFull,
		"version":     version.Version,
		"mode":        string(p.Mode),
		"os":          runtime.GOOS,
	}
	for _, kv := range []struct{ k, v string }{
		{"command", p.Cmd},
		{"subcommand", p.Sub},
		{"error", p.Err},
		{"type", p.Type},
		{"k8s.distro", p.K8sDistro},
		{"cloud.provider", p.CloudProvider},
		{"option", p.Opt},
	} {
		if kv.v != "" {
			props[kv.k] = kv.v
		}
	}
	maps.Copy(props, p.ExtraProps)
	return props
}
