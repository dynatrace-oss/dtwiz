package selfmonitoring

import (
	"encoding/json"
	"fmt"
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

// Event body property names. These are the query surface, so they stay stable.
const (
	bodyExecutionID   = "executionId"
	bodyStep          = "step"
	bodyVersion       = "version"
	bodyMode          = "mode"
	bodyOS            = "os"
	bodyCommand       = "command"
	bodySubcommand    = "subcommand"
	bodyError         = "error"
	bodyType          = "type"
	bodyCloudProvider = "cloud.provider"
	bodyK8sDistro     = "k8s.distro"
	bodyOption        = "option"
)

type eventPayload struct {
	EventType  string            `json:"eventType"`
	Title      string            `json:"title"`
	Properties map[string]string `json:"properties"`
}

// buildEventBody marshals the Events v2 ingest payload for params.
// Callers must resolve an empty StepID to its default first.
func buildEventBody(p EventParams) ([]byte, error) {
	body, err := json.Marshal(eventPayload{
		EventType:  "CUSTOM_INFO",
		Title:      buildEventTitle(p),
		Properties: buildEventProps(p),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	return body, nil
}

// buildEventTitle returns "dtwiz", followed by the command and subcommand when set.
func buildEventTitle(p EventParams) string {
	title := "dtwiz"
	if p.Cmd != "" {
		title += " " + p.Cmd
	}
	if p.Sub != "" {
		title += " " + p.Sub
	}
	return title
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
		bodyExecutionID: execID,
		bodyStep:        stepFull,
		bodyVersion:     version.Version,
		bodyMode:        string(p.Mode),
		bodyOS:          runtime.GOOS,
	}
	for _, kv := range []struct{ k, v string }{
		{bodyCommand, p.Cmd},
		{bodySubcommand, p.Sub},
		{bodyError, p.Err},
		{bodyType, p.Type},
		{bodyK8sDistro, p.K8sDistro},
		{bodyCloudProvider, p.CloudProvider},
		{bodyOption, p.Opt},
	} {
		if kv.v != "" {
			props[kv.k] = kv.v
		}
	}
	maps.Copy(props, p.ExtraProps)
	return props
}
