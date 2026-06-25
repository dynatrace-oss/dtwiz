package azure

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestSDKClient(t *testing.T, serverURL string) *sdkDTClient {
	t.Helper()
	c, err := newSDKDTClient(serverURL, "dt0s16.test")
	if err != nil {
		t.Fatalf("create test SDK client: %v", err)
	}
	c.c.HTTP().SetRetryCount(0)
	return c
}

// ─── createConnection ─────────────────────────────────────────────────────────

func TestSDKCreateConnection_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != settingsAPI {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"objectId":"conn-abc123"}]`))
	}))
	defer srv.Close()

	id, err := newTestSDKClient(t, srv.URL).createConnection("test-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "conn-abc123" {
		t.Errorf("objectId = %q, want %q", id, "conn-abc123")
	}
}

func TestSDKCreateConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := newTestSDKClient(t, srv.URL).createConnection("test-conn")
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error %q does not mention status 400", err.Error())
	}
}

func TestSDKCreateConnection_APIErrorField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"objectId":"","error":{"message":"schema validation failed"}}]`))
	}))
	defer srv.Close()

	_, err := newTestSDKClient(t, srv.URL).createConnection("test-conn")
	if err == nil {
		t.Fatal("expected error for API error field, got nil")
	}
	if !strings.Contains(err.Error(), "schema validation failed") {
		t.Errorf("error %q does not contain API error message", err.Error())
	}
}

func TestSDKCreateConnection_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	_, err := newTestSDKClient(t, srv.URL).createConnection("test-conn")
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("error %q does not mention empty response", err.Error())
	}
}

// ─── findConnection ───────────────────────────────────────────────────────────

func TestSDKFindConnection_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("schemaIds") != connectionSchemaID {
			t.Errorf("schemaIds query param missing or wrong: %q", r.URL.Query().Get("schemaIds"))
		}
		w.Header().Set("Content-Type", "application/json")
		body := map[string]interface{}{
			"items": []map[string]interface{}{{
				"objectId": "obj-001",
				"value": map[string]interface{}{
					"name": "my-conn",
					"type": "federatedIdentityCredential",
					"federatedIdentityCredential": map[string]interface{}{
						"applicationId": "app-client-id",
						"consumers":     []string{"SVC:com.dynatrace.da"},
					},
				},
			}},
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	objID, clientID, err := newTestSDKClient(t, srv.URL).findConnection("my-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objID != "obj-001" {
		t.Errorf("objectId = %q, want %q", objID, "obj-001")
	}
	if clientID != "app-client-id" {
		t.Errorf("clientId = %q, want %q", clientID, "app-client-id")
	}
}

func TestSDKFindConnection_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	objID, clientID, err := newTestSDKClient(t, srv.URL).findConnection("missing-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objID != "" || clientID != "" {
		t.Errorf("expected empty strings, got objID=%q clientID=%q", objID, clientID)
	}
}

func TestSDKFindConnection_OtherConnectionsIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := map[string]interface{}{
			"items": []map[string]interface{}{
				{"objectId": "other-obj", "value": map[string]interface{}{"name": "other-conn", "type": "federatedIdentityCredential"}},
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	objID, _, err := newTestSDKClient(t, srv.URL).findConnection("my-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objID != "" {
		t.Errorf("expected empty objectId for unmatched connection, got %q", objID)
	}
}

func TestSDKFindConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, _, err := newTestSDKClient(t, srv.URL).findConnection("my-conn")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error %q does not mention 403", err.Error())
	}
}

// ─── deleteConnection ─────────────────────────────────────────────────────────

func TestSDKDeleteConnection_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if want := settingsAPI + "/obj-001"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestSDKClient(t, srv.URL).deleteConnection("obj-001"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSDKDeleteConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := newTestSDKClient(t, srv.URL).deleteConnection("obj-001")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q does not mention 404", err.Error())
	}
}

// ─── updateConnection ─────────────────────────────────────────────────────────

func TestSDKUpdateConnection_HappyPath(t *testing.T) {
	var putBody map[string]interface{}
	var putIfMatch string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"schemaVersion":"42","value":{"name":"my-conn","type":"federatedIdentityCredential"}}`))
		case http.MethodPut:
			putIfMatch = r.Header.Get("If-Match")
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Errorf("decode PUT body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
	}))
	defer srv.Close()

	err := newTestSDKClient(t, srv.URL).updateConnection("obj-001", "my-conn", "tenant-xyz", "client-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if putIfMatch != "42" {
		t.Errorf("If-Match = %q, want %q", putIfMatch, "42")
	}
	val, ok := putBody["value"].(map[string]interface{})
	if !ok {
		t.Fatalf("put body.value missing or wrong type: %T", putBody["value"])
	}
	fedCred, ok := val["federatedIdentityCredential"].(map[string]interface{})
	if !ok {
		t.Fatalf("federatedIdentityCredential missing: %+v", val)
	}
	if fedCred["directoryId"] != "tenant-xyz" {
		t.Errorf("directoryId = %v, want tenant-xyz", fedCred["directoryId"])
	}
	if fedCred["applicationId"] != "client-abc" {
		t.Errorf("applicationId = %v, want client-abc", fedCred["applicationId"])
	}
}

func TestSDKUpdateConnection_GetFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()

	err := newTestSDKClient(t, srv.URL).updateConnection("obj-001", "my-conn", "tenant-xyz", "client-abc")
	if err == nil {
		t.Fatal("expected error when GET fails, got nil")
	}
	if !strings.Contains(err.Error(), "get current") {
		t.Errorf("error %q does not mention get current", err.Error())
	}
}

func TestSDKUpdateConnection_PutFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"schemaVersion":"1","value":{"name":"my-conn","type":"federatedIdentityCredential"}}`))
		case http.MethodPut:
			w.WriteHeader(http.StatusConflict)
		}
	}))
	defer srv.Close()

	err := newTestSDKClient(t, srv.URL).updateConnection("obj-001", "my-conn", "tenant-xyz", "client-abc")
	if err == nil {
		t.Fatal("expected error when PUT fails, got nil")
	}
	if !strings.Contains(err.Error(), "409") {
		t.Errorf("error %q does not mention 409", err.Error())
	}
}

// ─── cmpSemver ───────────────────────────────────────────────────────────────

func TestCmpSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.2.3", "1.2.4", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.10.0", "1.9.0", 1},  // numeric, not lexicographic
		{"1.0.11", "1.0.9", 1},  // numeric: 11 > 9
		{"1.0", "1.0.0", 0},
		{"1", "1.0.0", 0},
		{"", "", 0},
	}
	for _, tc := range cases {
		if got := cmpSemver(tc.a, tc.b); got != tc.want {
			t.Errorf("cmpSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// ─── latestExtensionVersion ───────────────────────────────────────────────────

func TestSDKLatestExtensionVersion_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != extensionAPI {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"version":"1.2.0"},{"version":"1.0.0"},{"version":"1.1.3"}]}`))
	}))
	defer srv.Close()

	v, err := newTestSDKClient(t, srv.URL).latestExtensionVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "1.2.0" {
		t.Errorf("version = %q, want %q", v, "1.2.0")
	}
}

func TestSDKLatestExtensionVersion_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	_, err := newTestSDKClient(t, srv.URL).latestExtensionVersion()
	if err == nil {
		t.Fatal("expected error for empty versions, got nil")
	}
}

func TestSDKLatestExtensionVersion_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newTestSDKClient(t, srv.URL).latestExtensionVersion()
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

// ─── fetchExtensionSchema / enumValues ────────────────────────────────────────

func TestSDKFetchExtensionSchema_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := extensionAPI + "/1.2.0/schema"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"enums":{
			"dynatrace.datasource.azure:location":{"items":[{"value":"eastus"},{"value":"westeurope"}]},
			"FeatureSetsType":{"items":[{"value":"essential_one"},{"value":""}]}
		}}`))
	}))
	defer srv.Close()

	schema, err := newTestSDKClient(t, srv.URL).fetchExtensionSchema("1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	locs := schema.enumValues(azureLocationEnumKey)
	if len(locs) != 2 {
		t.Errorf("expected 2 locations, got %d: %v", len(locs), locs)
	}
	// blank value must be filtered out
	fs := schema.enumValues(azureFeatureSetEnumKey)
	if len(fs) != 1 || fs[0] != "essential_one" {
		t.Errorf("expected [essential_one], got: %v", fs)
	}
	// missing key returns nil
	if schema.enumValues("nonexistent-key") != nil {
		t.Error("expected nil for missing enum key")
	}
}

func TestSDKFetchExtensionSchema_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := newTestSDKClient(t, srv.URL).fetchExtensionSchema("1.2.0")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

// ─── createMonitoring ─────────────────────────────────────────────────────────

// stockDAAzureSchemaJSON is a minimal valid da-azure schema with 2 locations and 3 feature
// sets (2 _essential, 1 _premium that must be excluded by createMonitoring).
const stockDAAzureSchemaJSON = `{"enums":{
	"dynatrace.datasource.azure:location":{"items":[{"value":"eastus"},{"value":"westeurope"}]},
	"FeatureSetsType":{"items":[{"value":"compute_essential"},{"value":"storage_essential"},{"value":"compute_premium"}]}
}}`

type monitoringServerOpts struct {
	versionErr     bool
	noVersions     bool
	schemaErr      bool
	emptyLocations bool
	noEssential    bool
	postErr        bool
	captureBody    interface{}
}

func newMonitoringTestServer(t *testing.T, opts monitoringServerOpts) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == extensionAPI:
			if opts.versionErr {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if opts.noVersions {
				_, _ = w.Write([]byte(`{"items":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"items":[{"version":"1.2.0"}]}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/schema"):
			if opts.schemaErr {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if opts.emptyLocations {
				_, _ = w.Write([]byte(`{"enums":{"FeatureSetsType":{"items":[{"value":"compute_essential"}]}}}`))
				return
			}
			if opts.noEssential {
				_, _ = w.Write([]byte(`{"enums":{"dynatrace.datasource.azure:location":{"items":[{"value":"eastus"}]},"FeatureSetsType":{"items":[{"value":"compute_premium"}]}}}`))
				return
			}
			_, _ = w.Write([]byte(stockDAAzureSchemaJSON))
		case r.Method == http.MethodPost && r.URL.Path == monitoringAPI:
			if opts.postErr {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if opts.captureBody != nil {
				_ = json.NewDecoder(r.Body).Decode(opts.captureBody)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"objectId":"mon-001"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestSDKCreateMonitoring_HappyPath(t *testing.T) {
	var body map[string]interface{}
	srv := newMonitoringTestServer(t, monitoringServerOpts{captureBody: &body})
	defer srv.Close()

	if err := newTestSDKClient(t, srv.URL).createMonitoring("cfg-name", "conn-obj-001", "client-app-001", "sub-abc123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body["scope"] != "integration-azure" {
		t.Errorf("scope = %v, want integration-azure", body["scope"])
	}
	val, _ := body["value"].(map[string]interface{})
	if val == nil {
		t.Fatal("body.value missing")
	}
	if val["enabled"] != true {
		t.Errorf("enabled = %v, want true", val["enabled"])
	}
	if val["description"] != "cfg-name" {
		t.Errorf("description = %v, want cfg-name", val["description"])
	}

	// only _essential feature sets included
	fs, _ := val["featureSets"].([]interface{})
	if len(fs) != 2 {
		t.Errorf("expected 2 essential featureSets, got %d: %v", len(fs), fs)
	}
	for _, f := range fs {
		if strings.HasSuffix(f.(string), "_premium") {
			t.Errorf("featureSets must not include _premium, got: %v", fs)
		}
	}

	azure, _ := val["azure"].(map[string]interface{})
	if azure == nil {
		t.Fatal("azure block missing")
	}
	if azure["subscriptionFilteringMode"] != "INCLUDE" {
		t.Errorf("subscriptionFilteringMode = %v, want INCLUDE", azure["subscriptionFilteringMode"])
	}
	subs, _ := azure["subscriptionFiltering"].([]interface{})
	if len(subs) != 1 || subs[0] != "sub-abc123" {
		t.Errorf("subscriptionFiltering = %v, want [sub-abc123]", subs)
	}
	locs, _ := azure["locationFiltering"].([]interface{})
	if len(locs) != 2 {
		t.Errorf("expected 2 locationFiltering entries, got %d", len(locs))
	}
	creds, _ := azure["credentials"].([]interface{})
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	cred, _ := creds[0].(map[string]interface{})
	if cred["connectionId"] != "conn-obj-001" {
		t.Errorf("connectionId = %v, want conn-obj-001", cred["connectionId"])
	}
	if cred["servicePrincipalId"] != "client-app-001" {
		t.Errorf("servicePrincipalId = %v, want client-app-001", cred["servicePrincipalId"])
	}
	if cred["type"] != "FEDERATED" {
		t.Errorf("type = %v, want FEDERATED", cred["type"])
	}
}

func TestSDKCreateMonitoring_VersionFetchFails(t *testing.T) {
	srv := newMonitoringTestServer(t, monitoringServerOpts{versionErr: true})
	defer srv.Close()
	if err := newTestSDKClient(t, srv.URL).createMonitoring("cfg", "conn", "client", "sub"); err == nil {
		t.Fatal("expected error when version fetch fails, got nil")
	}
}

func TestSDKCreateMonitoring_NoVersions(t *testing.T) {
	srv := newMonitoringTestServer(t, monitoringServerOpts{noVersions: true})
	defer srv.Close()
	if err := newTestSDKClient(t, srv.URL).createMonitoring("cfg", "conn", "client", "sub"); err == nil {
		t.Fatal("expected error for no versions, got nil")
	}
}

func TestSDKCreateMonitoring_SchemaFetchFails(t *testing.T) {
	srv := newMonitoringTestServer(t, monitoringServerOpts{schemaErr: true})
	defer srv.Close()
	if err := newTestSDKClient(t, srv.URL).createMonitoring("cfg", "conn", "client", "sub"); err == nil {
		t.Fatal("expected error when schema fetch fails, got nil")
	}
}

func TestSDKCreateMonitoring_NoLocations(t *testing.T) {
	srv := newMonitoringTestServer(t, monitoringServerOpts{emptyLocations: true})
	defer srv.Close()
	err := newTestSDKClient(t, srv.URL).createMonitoring("cfg", "conn", "client", "sub")
	if err == nil {
		t.Fatal("expected error for no locations, got nil")
	}
	if !strings.Contains(err.Error(), "no locations") {
		t.Errorf("error %q does not mention no locations", err.Error())
	}
}

func TestSDKCreateMonitoring_NoEssentialFeatureSets(t *testing.T) {
	srv := newMonitoringTestServer(t, monitoringServerOpts{noEssential: true})
	defer srv.Close()
	err := newTestSDKClient(t, srv.URL).createMonitoring("cfg", "conn", "client", "sub")
	if err == nil {
		t.Fatal("expected error for no essential feature sets, got nil")
	}
	if !strings.Contains(err.Error(), "_essential") {
		t.Errorf("error %q does not mention _essential feature sets", err.Error())
	}
}

func TestSDKCreateMonitoring_PostFails(t *testing.T) {
	srv := newMonitoringTestServer(t, monitoringServerOpts{postErr: true})
	defer srv.Close()
	err := newTestSDKClient(t, srv.URL).createMonitoring("cfg", "conn", "client", "sub")
	if err == nil {
		t.Fatal("expected error when POST fails, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error %q does not mention 400", err.Error())
	}
}

// ─── findMonitoringConfig ─────────────────────────────────────────────────────

func TestSDKFindMonitoringConfig_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != monitoringAPI {
			t.Errorf("path = %q, want %q", r.URL.Path, monitoringAPI)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"objectId":"mon-001","value":{"description":"my-config"}},
			{"objectId":"mon-002","value":{"description":"other-config"}}
		]}`))
	}))
	defer srv.Close()

	id, err := newTestSDKClient(t, srv.URL).findMonitoringConfig("my-config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "mon-001" {
		t.Errorf("objectId = %q, want mon-001", id)
	}
}

func TestSDKFindMonitoringConfig_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	id, err := newTestSDKClient(t, srv.URL).findMonitoringConfig("missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty id for not-found, got %q", id)
	}
}

func TestSDKFindMonitoringConfig_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := newTestSDKClient(t, srv.URL).findMonitoringConfig("my-config")
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q does not mention 401", err.Error())
	}
}

// ─── deleteMonitoring ─────────────────────────────────────────────────────────

func TestSDKDeleteMonitoring_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if want := monitoringAPI + "/mon-001"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestSDKClient(t, srv.URL).deleteMonitoring("mon-001"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSDKDeleteMonitoring_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := newTestSDKClient(t, srv.URL).deleteMonitoring("mon-001")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q does not mention 404", err.Error())
	}
}
