package aws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

func newTestDTClient(t *testing.T, serverURL string) *sdkDTClient {
	t.Helper()
	ec, err := installer.NewExtensionClient(serverURL, "dt0s16.test")
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	return &sdkDTClient{ExtensionClient: ec}
}

// TestEnableMonitoringConfig_FlipsTopLevelAndCredentials drives the full
// GET + PUT round-trip and asserts that both `value.enabled` and every
// `value.aws.credentials[].enabled` flag are set to true in the PUT payload.
// Mirrors `dtctl enable aws monitoring` — without this step the CloudFormation
// stack is deployed but no data ever flows.
func TestEnableMonitoringConfig_FlipsTopLevelAndCredentials(t *testing.T) {
	const objectID = "abc-123"
	const configPath = "/platform/extensions/v2/extensions/" + extensionName + "/monitoring-configurations/" + objectID

	var receivedPUT map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != configPath {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"objectId": "abc-123",
				"scope": "integration-aws",
				"value": {
					"enabled": false,
					"aws": {
						"credentials": [
							{"enabled": false, "accountId": "111111111111"},
							{"enabled": false, "accountId": "222222222222"}
						]
					}
				}
			}`))
		case http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&receivedPUT); err != nil {
				t.Errorf("decode PUT body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"objectId":"abc-123"}`))
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
	}))
	defer srv.Close()

	dtc := newTestDTClient(t, srv.URL)
	if err := dtc.enableMonitoringConfig(objectID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// scope preserved
	if got, _ := receivedPUT["scope"].(string); got != "integration-aws" {
		t.Errorf("scope in PUT = %q, want integration-aws", got)
	}
	value, ok := receivedPUT["value"].(map[string]any)
	if !ok {
		t.Fatalf("value missing in PUT body")
	}
	if enabled, _ := value["enabled"].(bool); !enabled {
		t.Errorf("value.enabled = %v, want true", value["enabled"])
	}
	aws, ok := value["aws"].(map[string]any)
	if !ok {
		t.Fatalf("aws missing in PUT body")
	}
	creds, ok := aws["credentials"].([]any)
	if !ok || len(creds) != 2 {
		t.Fatalf("credentials missing or wrong length: %v", aws["credentials"])
	}
	for i, c := range creds {
		m, ok := c.(map[string]any)
		if !ok {
			t.Fatalf("credential[%d] is not an object: %v", i, c)
		}
		if enabled, _ := m["enabled"].(bool); !enabled {
			t.Errorf("credential[%d].enabled = %v, want true", i, m["enabled"])
		}
	}
}

func TestEnableMonitoringConfig_PropagatesGetError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"not found"}}`))
	}))
	defer srv.Close()

	dtc := newTestDTClient(t, srv.URL)
	err := dtc.enableMonitoringConfig("abc-123")
	if err == nil {
		t.Fatal("expected error when GET returns 404")
	}
	if !strings.Contains(err.Error(), "404") && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("error %q should mention HTTP status or not found", err.Error())
	}
}
