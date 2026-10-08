// Package selfmonitoring sends per-invocation telemetry events to the Dynatrace Events v2 API.
package selfmonitoring

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var smHTTPClient = &http.Client{Timeout: 3 * time.Second}

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
	Cmd           string            // top-level command: install, update, uninstall, analyze, etc.
	Sub           string            // subcommand: otel, kubernetes, oneagent, etc.
	Opt           string            // setup menu option (method presented/selected); encoded as opt= in header, distinct from Sub
	StepID        string            // execution step shortcode; defaults to StepInvoked when empty
	Mode          Mode              // ModeDebug, ModeTTY, or ModeNonTTY
	Err           string            // error category; omitted when empty
	Type          string            // event type qualifier; omitted when empty
	K8sDistro     string            // kubernetes distribution (e.g. "GKE", "EKS"); abbreviated in User-Agent, full name in body
	CloudProvider string            // detected cloud provider ("aws", "azure", "gcp"); abbreviated in User-Agent, full name in body
	Install       *InstallReport    // install feature outcomes and work time (ist step); nil when no install work started
	ExtraProps    map[string]string // merged into event body properties; not included in User-Agent
}

const (
	headerKey   = "dtwiz-monitoring"
	headerValue = "dtwiz-start"

	StepInvoked                  = "inv"
	StepAnalyze                  = "ana"
	StepInstall                  = "ist"
	StepSnapshot                 = "snp"
	StepCompleted                = "com"
	StepFailed                   = "fai"
	StepCancelled                = "can"
	StepRecommendationsPresented = "rpr"
	StepRecommendationsSelected  = "rsl"
)

// SendEvent ingests a self-monitoring event into the given classic Dynatrace environment.
// Errors are logged at debug level only — this must never surface to the user.
func SendEvent(classicURL, token string, params EventParams) error {
	if params.StepID == "" {
		params.StepID = StepInvoked
	}

	eventBody, err := buildEventBody(params)
	if err != nil {
		return err
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

func authHeader(token string) string {
	if strings.HasPrefix(token, "dt0c01.") {
		return "Api-Token " + token
	}
	return "Bearer " + token
}
