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
	ec.C.HTTP().SetRetryCount(0)
	return &sdkDTClient{ExtensionClient: ec}
}

// TestEnableMonitoringConfig_FlipsTopLevelAndCredentials drives the full
// GET + PUT round-trip and asserts that both `value.enabled` and every
// `value.aws.credentials[].enabled` flag are set to true in the PUT payload.
// Mirrors `dtctl enable aws monitoring` — without this step the CloudFormation
// stack is deployed but no data ever flows.
func TestEnableMonitoringConfig_FlipsTopLevelAndCredentials(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

// TestInstallExtension_NotFoundTriggersInstall verifies the normal "not yet
// installed" path: a 404 from the version check is treated as absence and
// InstallFromHub is called.
func TestInstallExtension_NotFoundTriggersInstall(t *testing.T) {
	t.Parallel()

	path := "/platform/extensions/v2/extensions/" + extensionName
	var installCalled bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"not found"}}`))
		case http.MethodPost:
			installCalled = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"extensionName":"` + extensionName + `","version":"1.0.0"}`))
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
	}))
	defer srv.Close()

	dtc := newTestDTClient(t, srv.URL)
	fresh, err := dtc.installExtension()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fresh {
		t.Error("expected freshlyInstalled = true")
	}
	if !installCalled {
		t.Error("expected InstallFromHub to be called after a 404 version check")
	}
}

// TestInstallExtension_PropagatesNonNotFoundError guards against a network
// timeout or auth failure during the version check being silently swallowed
// and turned into a confusing double-install attempt.
func TestInstallExtension_PropagatesNonNotFoundError(t *testing.T) {
	t.Parallel()

	path := "/platform/extensions/v2/extensions/" + extensionName
	var installCalled bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":500,"message":"internal error"}}`))
		case http.MethodPost:
			installCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
	}))
	defer srv.Close()

	dtc := newTestDTClient(t, srv.URL)
	if _, err := dtc.installExtension(); err == nil {
		t.Fatal("expected error to propagate for a non-404 version-check failure")
	}
	if installCalled {
		t.Error("InstallFromHub must not be called when the version check fails with a non-404 error")
	}
}

func TestIsExtensionActive_UsesAWSExtensionName(t *testing.T) {
	t.Parallel()

	path := "/platform/extensions/v2/extensions/" + extensionName
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %q", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"version":"1.0.0","active":false},{"version":"1.1.0","active":true}]}`))
	}))
	defer srv.Close()

	active, err := newTestDTClient(t, srv.URL).isExtensionActive()
	if err != nil {
		t.Fatalf("isExtensionActive() returned error: %v", err)
	}
	if !active {
		t.Fatal("isExtensionActive() = false, want true when any AWS extension version is active")
	}
}

func TestLatestExtensionVersion_UsesHighestInstalledAWSVersion(t *testing.T) {
	t.Parallel()

	path := "/platform/extensions/v2/extensions/" + extensionName
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"version":"1.9.0"},{"version":"1.10.0"},{"version":"1.2.0"}]}`))
	}))
	defer srv.Close()

	version, err := newTestDTClient(t, srv.URL).latestExtensionVersion()
	if err != nil {
		t.Fatalf("latestExtensionVersion() returned error: %v", err)
	}
	if version != "1.10.0" {
		t.Fatalf("latestExtensionVersion() = %q, want 1.10.0", version)
	}
}

// TestFindExistingMonitoringConfig_NotFoundReturnsEmpty verifies the normal
// "extension not installed yet" path: a 404 is treated as no existing config.
func TestFindExistingMonitoringConfig_NotFoundReturnsEmpty(t *testing.T) {
	t.Parallel()

	path := "/platform/extensions/v2/extensions/" + extensionName + "/monitoring-configurations"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"not found"}}`))
	}))
	defer srv.Close()

	dtc := newTestDTClient(t, srv.URL)
	id, err := dtc.findExistingMonitoringConfig("111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" {
		t.Errorf("objectId = %q, want empty", id)
	}
}

// TestFindExistingMonitoringConfig_PropagatesOtherErrors guards against a
// real failure (e.g. network timeout, auth error) being misclassified as
// "extension not installed" and silently swallowed.
func TestFindExistingMonitoringConfig_PropagatesOtherErrors(t *testing.T) {
	t.Parallel()

	path := "/platform/extensions/v2/extensions/" + extensionName + "/monitoring-configurations"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"message":"internal error"}}`))
	}))
	defer srv.Close()

	dtc := newTestDTClient(t, srv.URL)
	if _, err := dtc.findExistingMonitoringConfig("111111111111"); err == nil {
		t.Fatal("expected error to propagate for a non-404 failure")
	}
}

func TestFindExistingMonitoringConfig_MatchesAccountID(t *testing.T) {
	t.Parallel()

	path := "/platform/extensions/v2/extensions/" + extensionName + "/monitoring-configurations"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"objectId":"ignored-malformed","value":"not-json"},
				{"objectId":"wrong-account","value":{"aws":{"credentials":[{"accountId":"222222222222"}]}}},
				{"objectId":"matching-config","value":{"aws":{"credentials":[{"accountId":"111111111111"}]}}}
			]
		}`))
	}))
	defer srv.Close()

	id, err := newTestDTClient(t, srv.URL).findExistingMonitoringConfig("111111111111")
	if err != nil {
		t.Fatalf("findExistingMonitoringConfig() returned error: %v", err)
	}
	if id != "matching-config" {
		t.Fatalf("findExistingMonitoringConfig() = %q, want matching-config", id)
	}
}

func TestDeleteMonitoringConfig_UsesAWSExtensionName(t *testing.T) {
	t.Parallel()

	path := "/platform/extensions/v2/extensions/" + extensionName + "/monitoring-configurations/config-123"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected method %q", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestDTClient(t, srv.URL).deleteMonitoringConfig("config-123"); err != nil {
		t.Fatalf("deleteMonitoringConfig() returned error: %v", err)
	}
}

func TestCreateMonitoringConfig_BuildsQuickStartConfig(t *testing.T) {
	t.Parallel()

	path := "/platform/extensions/v2/extensions/" + extensionName + "/monitoring-configurations"
	var receivedPOST map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %q", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedPOST); err != nil {
			t.Errorf("decode POST body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"objectId":"created-config-id"}`))
	}))
	defer srv.Close()

	dtc := newTestDTClient(t, srv.URL)
	id, err := dtc.createMonitoringConfig("111111111111", "eu-west-1", "1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "created-config-id" {
		t.Fatalf("objectId = %q, want created-config-id", id)
	}
	if got, _ := receivedPOST["scope"].(string); got != "integration-aws" {
		t.Fatalf("scope = %q, want integration-aws", got)
	}

	value, ok := receivedPOST["value"].(map[string]any)
	if !ok {
		t.Fatalf("value missing in POST body")
	}
	if enabled, ok := value["enabled"].(bool); !ok || enabled {
		t.Errorf("value.enabled = %v, want false until CloudFormation deploy completes", value["enabled"])
	}
	if got, _ := value["version"].(string); got != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", got)
	}
	if got, _ := value["activationContext"].(string); got != "DATA_ACQUISITION" {
		t.Errorf("activationContext = %q, want DATA_ACQUISITION", got)
	}

	aws, ok := value["aws"].(map[string]any)
	if !ok {
		t.Fatalf("aws missing in POST body")
	}
	if got, _ := aws["deploymentRegion"].(string); got != "eu-west-1" {
		t.Errorf("deploymentRegion = %q, want eu-west-1", got)
	}
	if got, _ := aws["configurationMode"].(string); got != "QUICK_START" {
		t.Errorf("configurationMode = %q, want QUICK_START", got)
	}
	if got, _ := aws["deploymentMode"].(string); got != "AUTOMATED" {
		t.Errorf("deploymentMode = %q, want AUTOMATED", got)
	}

	creds, ok := aws["credentials"].([]any)
	if !ok || len(creds) != 1 {
		t.Fatalf("credentials missing or wrong length: %v", aws["credentials"])
	}
	credential, ok := creds[0].(map[string]any)
	if !ok {
		t.Fatalf("credential is not an object: %v", creds[0])
	}
	if got, _ := credential["accountId"].(string); got != "111111111111" {
		t.Errorf("credential.accountId = %q, want 111111111111", got)
	}
	if enabled, ok := credential["enabled"].(bool); !ok || enabled {
		t.Errorf("credential.enabled = %v, want false until CloudFormation deploy completes", credential["enabled"])
	}

	regions, ok := aws["regionFiltering"].([]any)
	if !ok || len(regions) != 1 || regions[0] != "eu-west-1" {
		t.Errorf("regionFiltering = %v, want [eu-west-1]", aws["regionFiltering"])
	}
}
