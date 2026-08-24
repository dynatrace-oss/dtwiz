// Package selfmonitoring is a PoC for verifying signal propagation through the Dynatrace pipeline.
// Gated by DTWIZ_SELF_MONITORING_POC=true — not intended for production use.
package selfmonitoring

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

const (
	headerKey   = "dtwiz-monitoring"
	headerValue = "dtwiz-start"
	eventBody   = `{"eventType":"CUSTOM_INFO","title":"dtwiz started","properties":{"monitoring-event":"dtwiz-start"}}`
)

// SendEvent ingests a self-monitoring event into the given classic Dynatrace environment.
// The custom header dtwiz-monitoring is included so we can test whether it survives the pipeline.
// Errors are logged at debug level only — this must never surface to the user.
func SendEvent(classicURL, token string) error {
	url := strings.TrimRight(classicURL, "/") + "/api/v2/events/ingest"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(eventBody))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", authHeader(token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "dtwiz/"+version.Version+";sfmid="+headerValue)
	req.Header.Set(headerKey, headerValue)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	logger.Debug(fmt.Sprintf("selfmonitoring: event sent to %s (status %d): %s", url, resp.StatusCode, string(body)))
	return nil
}

func authHeader(token string) string {
	if strings.HasPrefix(token, "dt0c01.") {
		return "Api-Token " + token
	}
	return "Bearer " + token
}
