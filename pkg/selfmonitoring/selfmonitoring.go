// Package selfmonitoring sends per-invocation telemetry events to the Dynatrace Events v2 API.
// Gated by DTWIZ_SELF_MONITORING_POC feature flag — not intended for production use yet.
package selfmonitoring

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

var smHTTPClient = &http.Client{Timeout: 3 * time.Second}

var execID string

func init() {
	b := make([]byte, 1)
	if _, err := rand.Read(b); err != nil {
		execID = "00"
		return
	}
	execID = fmt.Sprintf("%x", b)
}

// EventParams holds per-invocation metadata embedded in the request User-Agent and event body.
type EventParams struct {
	CmdID      string            // top-level command: install, update, uninstall, etc.
	SubID      string            // subcommand: otel, gcp, kubernetes, etc.
	StepID     string            // execution step; defaults to StepInvoked when empty
	Mode       string            // deb, tty, or ntt
	Err        string            // error category; omitted from event body when empty
	Type       string            // event type qualifier; omitted from event body when empty
	ExtraProps map[string]string // merged into event body properties; not included in User-Agent
}

const (
	headerKey   = "dtwiz-monitoring"
	headerValue = "dtwiz-start"

	StepInvoked   = "inv"
	StepCompleted = "com"

	propExecID = "e"
	propCmd    = "c"
	propStep   = "st"
	propSub    = "s"
	propErr    = "er"
	propType   = "t"
)

type eventPayload struct {
	EventType  string            `json:"eventType"`
	Title      string            `json:"title"`
	Properties map[string]string `json:"properties"`
}

// SendEvent ingests a self-monitoring event into the given classic Dynatrace environment.
// Errors are logged at debug level only — this must never surface to the user.
func SendEvent(classicURL, token string, params EventParams) error {
	if params.StepID == "" {
		params.StepID = StepInvoked
	}

	props := map[string]string{
		propExecID: execID,
		propCmd:    params.CmdID,
		propStep:   params.StepID,
	}
	for _, kv := range []struct{ k, v string }{
		{propSub, params.SubID},
		{propErr, params.Err},
		{propType, params.Type},
	} {
		if kv.v != "" {
			props[kv.k] = kv.v
		}
	}
	maps.Copy(props, params.ExtraProps)
	title := "dtwiz " + params.CmdID
	if params.SubID != "" {
		title += " " + params.SubID
	}
	eventBody, err := json.Marshal(eventPayload{
		EventType:  "CUSTOM_INFO",
		Title:      title,
		Properties: props,
	})
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	url := strings.TrimRight(classicURL, "/") + "/api/v2/events/ingest"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(eventBody))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", authHeader(token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", buildUserAgent(params))
	req.Header.Set("Tab-Id", buildTabID(params))
	req.Header.Set(headerKey, headerValue)

	logger.Debug(fmt.Sprintf("selfmonitoring: POST %s  User-Agent: %s  Tab-Id: %s  body: %s",
		url, req.Header.Get("User-Agent"), req.Header.Get("Tab-Id"), string(eventBody)))

	resp, err := smHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	logger.Debug(fmt.Sprintf("selfmonitoring: event sent to %s (status %d): %s", url, resp.StatusCode, string(respBody)))
	return nil
}

// buildUserAgent encodes operation identity into User-Agent (64-char HAProxy capture limit).
// Format: dtwiz/<version>;c=<cmd>;st=<step>[;s=<sub>][;er=<err>][;t=<type>]
// ExecID, mode, and OS are omitted here — they go into Tab-Id via buildTabID.
func buildUserAgent(p EventParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dtwiz/%s;%s=%s;%s=%s",
		version.Version,
		propCmd, p.CmdID,
		propStep, p.StepID)
	for _, kv := range []struct{ k, v string }{
		{propSub, p.SubID},
		{propErr, p.Err},
		{propType, p.Type},
	} {
		if kv.v != "" {
			fmt.Fprintf(&b, ";%s=%s", kv.k, kv.v)
		}
	}
	return b.String()
}

// buildTabID encodes execution context into Tab-Id (16-char HAProxy capture limit).
// Format: <execid>;m=<mode>;o=<os> — execid is positional (always 2 hex chars), mode and os are 3 chars each.
// Worst case: "3a;m=deb;o=win" = 14 chars.
func buildTabID(p EventParams) string {
	return execID + ";m=" + p.Mode + ";o=" + resolveOS()
}

func resolveOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "linux":
		return "lin"
	case "windows":
		return "win"
	default:
		return runtime.GOOS
	}
}

func authHeader(token string) string {
	if strings.HasPrefix(token, "dt0c01.") {
		return "Api-Token " + token
	}
	return "Bearer " + token
}
