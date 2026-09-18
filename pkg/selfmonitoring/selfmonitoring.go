// Package selfmonitoring sends per-invocation telemetry events to the Dynatrace Events v2 API.
// Gated by DTWIZ_SELF_MONITORING_POC feature flag — not intended for production use yet.
package selfmonitoring

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

var smHTTPClient = &http.Client{Timeout: 3 * time.Second}

var wg sync.WaitGroup

// TrackSend registers one in-flight send with the WaitGroup. Must be called before the goroutine is spawned.
func TrackSend() { wg.Add(1) }

// SendDone signals that one in-flight send has completed.
func SendDone() { wg.Done() }

// Flush waits for all in-flight self-monitoring sends to complete, or until timeout elapses.
// Silent on timeout — any remaining sends are accepted as lost.
func Flush(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}

var execID string

func init() {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		execID = "000"
		return
	}
	execID = fmt.Sprintf("%02x%x", b[0], b[1]>>4)
}

// Mode identifies how dtwiz was invoked.
type Mode string

const (
	ModeDebug  Mode = "debug"
	ModeTTY    Mode = "tty"
	ModeNonTTY Mode = "non-tty"
)

// EventParams holds per-invocation metadata embedded in the request User-Agent and event body.
// Cmd and Sub carry the full, natural command names (e.g. "install", "kubernetes").
// Abbreviation for the 64-char User-Agent header is handled internally by this package.
type EventParams struct {
	Cmd        string            // top-level command: install, update, uninstall, analyze, etc.
	Sub        string            // subcommand: otel, kubernetes, oneagent, etc.
	StepID     string            // execution step shortcode; defaults to StepInvoked when empty
	Mode       Mode              // ModeDebug, ModeTTY, or ModeNonTTY
	Err        string            // error category; omitted when empty
	Type       string            // event type qualifier; omitted when empty
	ExtraProps map[string]string // merged into event body properties; not included in User-Agent
}

// stepFullNames maps step shortcodes to human-readable names used in the event body.
var stepFullNames = map[string]string{
	StepInvoked:   "invoked",
	StepAnalyze:   "analyze",
	StepRecommend: "recommend",
	StepInstall:   "install",
	StepCompleted: "completed",
}

// cmdShortMap and subShortMap abbreviate natural command names for the User-Agent header.
var cmdShortMap = map[string]string{
	"install":   "ins",
	"uninstall": "uni",
	"update":    "upd",
	"analyze":   "ana",
	"recommend": "rec",
	"status":    "sta",
	"watch":     "wch",
	"setup":     "set",
	"version":   "ver",
}

var subShortMap = map[string]string{
	"otel":           "otel",
	"otel-collector": "otlc",
	"otel-python":    "otlp",
	"otel-node":      "otln",
	"otel-java":      "otlj",
	"kubernetes":     "k8s",
	"oneagent":       "oa",
	"gcp":            "gcp",
	"azure":          "az",
	"aws":            "aws",
	"aws-lambda":     "awsl",
	"docker":         "dock",
	"demo":           "demo",
	"self":           "self",
	"uninstall":      "uni",
	"otel-update":    "otlu",
	"azure-update":   "azu",
	"gcp-update":     "gcpu",
}

const (
	headerKey   = "dtwiz-monitoring"
	headerValue = "dtwiz-start"

	StepInvoked   = "inv"
	StepAnalyze   = "ana"
	StepRecommend = "rec"
	StepInstall   = "ist"
	StepCompleted = "com"
	StepFailed    = "fai"
	StepCancelled = "can"

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

	stepFull := stepFullNames[params.StepID]
	if stepFull == "" {
		stepFull = params.StepID
	}

	props := map[string]string{
		"executionId": execID,
		"step":        stepFull,
		"version":     version.Version,
		"mode":        string(params.Mode),
		"os":          runtime.GOOS,
	}
	for _, kv := range []struct{ k, v string }{
		{"command", params.Cmd},
		{"subcommand", params.Sub},
		{"error", params.Err},
	} {
		if kv.v != "" {
			props[kv.k] = kv.v
		}
	}
	maps.Copy(props, params.ExtraProps)
	title := "dtwiz"
	if params.Cmd != "" {
		title += " " + params.Cmd
	}
	if params.Sub != "" {
		title += " " + params.Sub
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
// Format: dtwiz/<version>[;c=<cmd>];st=<step>[;s=<sub>][;er=<err>][;t=<type>]
// c= is omitted when Cmd is empty. ExecID, mode, and OS go into Tab-Id via buildTabID.
func buildUserAgent(p EventParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dtwiz/%s", version.Version)
	for _, kv := range []struct{ k, v string }{
		{propCmd, shortCmd(p.Cmd)},
		{propStep, p.StepID},
		{propSub, shortSub(p.Sub)},
		{propErr, p.Err},
		{propType, p.Type},
	} {
		if kv.v != "" {
			fmt.Fprintf(&b, ";%s=%s", kv.k, kv.v)
		}
	}
	return b.String()
}

func shortCmd(name string) string {
	if s, ok := cmdShortMap[name]; ok {
		return s
	}
	return name
}

func shortSub(name string) string {
	if s, ok := subShortMap[name]; ok {
		return s
	}
	return name
}

// buildTabID encodes execution context into Tab-Id (16-char HAProxy capture limit).
// Format: <execid>;m=<mode>;o=<os> — execid is positional (always 3 hex chars), mode and os are 3 chars each.
// Worst case: "3ab;m=deb;o=win" = 15 chars.
func buildTabID(p EventParams) string {
	return execID + ";m=" + shortMode(p.Mode) + ";o=" + resolveOS()
}

func shortMode(m Mode) string {
	switch m {
	case ModeDebug:
		return "deb"
	case ModeNonTTY:
		return "ntt"
	default:
		return string(m)
	}
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
