package installer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

func newAWSTestPlatformClient(t *testing.T, serverURL string) *client.PlatformClient {
	t.Helper()
	c, err := client.New(serverURL, serverURL, "dt0s16.test", "dt0s16.test", 0)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	c.Platform.HTTP().SetRetryCount(0)
	return c.Platform
}

// TestEnableAWSMonitoringConfig_FlipsTopLevelAndCredentials drives the full
// GET + PUT round-trip and asserts that both `value.enabled` and every
// `value.aws.credentials[].enabled` flag are set to true in the PUT payload.
// This is the "dtctl enable aws monitoring" parity test — without it the
// CloudFormation stack is deployed but no data ever flows.
func TestEnableAWSMonitoringConfig_FlipsTopLevelAndCredentials(t *testing.T) {
	const objectID = "abc-123"

	var receivedPUT map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
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
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
	}))
	defer srv.Close()

	if err := enableAWSMonitoringConfig(newAWSTestPlatformClient(t, srv.URL), objectID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// scope preserved
	if got, _ := receivedPUT["scope"].(string); got != "integration-aws" {
		t.Errorf("scope in PUT = %q, want integration-aws", got)
	}
	value, ok := receivedPUT["value"].(map[string]interface{})
	if !ok {
		t.Fatalf("value missing in PUT body")
	}
	if enabled, _ := value["enabled"].(bool); !enabled {
		t.Errorf("value.enabled = %v, want true", value["enabled"])
	}
	aws, ok := value["aws"].(map[string]interface{})
	if !ok {
		t.Fatalf("aws missing in PUT body")
	}
	creds, ok := aws["credentials"].([]interface{})
	if !ok || len(creds) != 2 {
		t.Fatalf("credentials missing or wrong length: %v", aws["credentials"])
	}
	for i, c := range creds {
		m, ok := c.(map[string]interface{})
		if !ok {
			t.Fatalf("credential[%d] is not an object: %v", i, c)
		}
		if enabled, _ := m["enabled"].(bool); !enabled {
			t.Errorf("credential[%d].enabled = %v, want true", i, m["enabled"])
		}
	}
}

func TestEnableAWSMonitoringConfig_PropagatesGetError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := enableAWSMonitoringConfig(newAWSTestPlatformClient(t, srv.URL), "abc-123")
	if err == nil {
		t.Fatal("expected error when GET returns 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q should mention HTTP status", err.Error())
	}
}
