package integration

import (
	"fmt"
	"io"
	"net/http"
	"testing"
)

// TriggerRequestOnPort sends a GET to http://localhost:<port>/ and fatals on error.
func TriggerRequestOnPort(t *testing.T, port int) {
	t.Helper()
	TriggerRequest(t, fmt.Sprintf("http://localhost:%d/", port))
}

// TriggerRequest sends a GET request to url and returns the response body.
// It calls t.Fatal on any error or non-2xx status.
func TriggerRequest(t *testing.T, url string) string {
	t.Helper()

	resp, err := http.Get(url) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("TriggerRequest: GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("TriggerRequest: GET %s returned status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("TriggerRequest: read body: %v", err)
	}

	return string(body)
}
