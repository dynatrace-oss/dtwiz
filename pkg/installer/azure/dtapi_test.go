package azure

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/internal/extensiontest"
)

func newTestSDKClient(t *testing.T, serverURL string) *sdkDTClient {
	t.Helper()
	c, err := newSDKDTClient(serverURL, "dt0s16.test")
	if err != nil {
		t.Fatalf("create test SDK client: %v", err)
	}
	c.C.HTTP().SetRetryCount(0)
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
	if !strings.Contains(err.Error(), "no items returned") {
		t.Errorf("error %q does not mention no items returned", err.Error())
	}
}

// ─── findAllConnections ───────────────────────────────────────────────────────

func TestSDKFindAllConnections_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("schemaIds") != connectionSchemaID {
			t.Errorf("schemaIds query param missing or wrong: %q", r.URL.Query().Get("schemaIds"))
		}
		if _, present := r.URL.Query()["scopes"]; present {
			t.Errorf("scopes query param must be omitted, got %q", r.URL.Query().Get("scopes"))
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

	conns, err := newTestSDKClient(t, srv.URL).findAllConnections("my-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("len(conns) = %d, want 1", len(conns))
	}
	if conns[0].objectID != "obj-001" {
		t.Errorf("objectId = %q, want %q", conns[0].objectID, "obj-001")
	}
	if conns[0].clientID != "app-client-id" {
		t.Errorf("clientId = %q, want %q", conns[0].clientID, "app-client-id")
	}
}

func TestSDKFindAllConnections_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	conns, err := newTestSDKClient(t, srv.URL).findAllConnections("missing-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 0 {
		t.Errorf("expected no connections, got %v", conns)
	}
}

func TestSDKFindAllConnections_OtherConnectionsIgnored(t *testing.T) {
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

	conns, err := newTestSDKClient(t, srv.URL).findAllConnections("my-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 0 {
		t.Errorf("expected no connections for unmatched name, got %v", conns)
	}
}

func TestSDKFindAllConnections_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := newTestSDKClient(t, srv.URL).findAllConnections("my-conn")
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
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"schemaVersion":"1","value":{}}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
	}))
	defer srv.Close()

	if err := newTestSDKClient(t, srv.URL).deleteConnection("obj-001"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSDKDeleteConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden) // non-404 error must propagate
	}))
	defer srv.Close()

	err := newTestSDKClient(t, srv.URL).deleteConnection("obj-001")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error %q does not mention 403", err.Error())
	}
}

func TestSDKDeleteConnection_404IsIdempotent(t *testing.T) {
	// Simulates DT cascade-deletion: the monitoring config delete caused DT to
	// also remove the connection, so deleteConnection gets a 404 on GET.
	// This must be treated as "already gone" and return nil, not an error.
	deleteCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := newTestSDKClient(t, srv.URL).deleteConnection("obj-001"); err != nil {
		t.Errorf("404 on GET must be treated as already deleted, got: %v", err)
	}
	if deleteCalled {
		t.Error("DELETE must not be issued when GET returns 404")
	}
}

func TestSDKDeleteConnection_404OnDeleteIsIdempotent(t *testing.T) {
	// Race: GET succeeds but DELETE returns 404 (object deleted between the two calls).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"schemaVersion":"1","value":{}}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
	}))
	defer srv.Close()

	if err := newTestSDKClient(t, srv.URL).deleteConnection("obj-001"); err != nil {
		t.Errorf("404 on DELETE must be treated as already deleted, got: %v", err)
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

// cmpSemver, latestExtensionVersion, and fetchExtensionSchema/enumValues are
// shared logic covered by pkg/installer's own test suite; createMonitoring
// below still exercises them end-to-end through buildMonitoringConfig.

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
	putErr         bool
}

func newMonitoringTestClient(t *testing.T, opts monitoringServerOpts) (*sdkDTClient, *extensiontest.FakeExtensionAPI) {
	t.Helper()
	sdk := &extensiontest.FakeExtensionAPI{
		Versions:  monitoringTestVersions(opts),
		GetErr:    monitoringTestVersionErr(opts),
		Schema:    monitoringTestSchema(opts),
		SchemaErr: monitoringTestSchemaErr(opts),
		CreateErr: monitoringTestCreateErr(opts),
		UpdateErr: monitoringTestUpdateErr(opts),
	}
	return newExtensionTestClient(sdk), sdk
}

func newExtensionTestClient(sdk *extensiontest.FakeExtensionAPI) *sdkDTClient {
	return &sdkDTClient{ExtensionClient: &installer.ExtensionClient{Extension: sdk}}
}

func monitoringTestCreateErr(opts monitoringServerOpts) error {
	if opts.postErr {
		return errors.New("400 bad request")
	}
	return nil
}

func monitoringTestUpdateErr(opts monitoringServerOpts) error {
	if opts.putErr {
		return errors.New("400 bad request")
	}
	return nil
}

func monitoringTestVersions(opts monitoringServerOpts) []extensiontest.Version {
	if opts.noVersions || opts.versionErr {
		return nil
	}
	return extensiontest.Versions("1.2.0")
}

func monitoringTestVersionErr(opts monitoringServerOpts) error {
	if opts.versionErr {
		return errors.New("temporary failure")
	}
	return nil
}

func monitoringTestSchema(opts monitoringServerOpts) json.RawMessage {
	switch {
	case opts.emptyLocations:
		return json.RawMessage(`{"enums":{"FeatureSetsType":{"items":[{"value":"compute_essential"}]}}}`)
	case opts.noEssential:
		return json.RawMessage(`{"enums":{"dynatrace.datasource.azure:location":{"items":[{"value":"eastus"}]},"FeatureSetsType":{"items":[{"value":"compute_premium"}]}}}`)
	default:
		return json.RawMessage(stockDAAzureSchemaJSON)
	}
}

func monitoringTestSchemaErr(opts monitoringServerOpts) error {
	if opts.schemaErr {
		return errors.New("temporary failure")
	}
	return nil
}

func TestSDKCreateMonitoring_HappyPath(t *testing.T) {
	var body map[string]interface{}
	dtc, sdk := newMonitoringTestClient(t, monitoringServerOpts{})

	if err := dtc.createMonitoring("cfg-name", "conn-obj-001", "client-app-001", "sub-abc123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sdk.CreateCalled {
		t.Fatal("expected CreateMonitoringConfiguration to be called")
	}
	if err := extensiontest.DecodeBody(sdk.CreateBody, &body); err != nil {
		t.Fatalf("decode monitoring body: %v", err)
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
	dtc, _ := newMonitoringTestClient(t, monitoringServerOpts{versionErr: true})
	if err := dtc.createMonitoring("cfg", "conn", "client", "sub"); err == nil {
		t.Fatal("expected error when version fetch fails, got nil")
	}
}

func TestSDKCreateMonitoring_NoVersions(t *testing.T) {
	dtc, _ := newMonitoringTestClient(t, monitoringServerOpts{noVersions: true})
	if err := dtc.createMonitoring("cfg", "conn", "client", "sub"); err == nil {
		t.Fatal("expected error for no versions, got nil")
	}
}

func TestSDKCreateMonitoring_SchemaFetchFails(t *testing.T) {
	dtc, _ := newMonitoringTestClient(t, monitoringServerOpts{schemaErr: true})
	if err := dtc.createMonitoring("cfg", "conn", "client", "sub"); err == nil {
		t.Fatal("expected error when schema fetch fails, got nil")
	}
}

func TestSDKCreateMonitoring_NoLocations(t *testing.T) {
	dtc, _ := newMonitoringTestClient(t, monitoringServerOpts{emptyLocations: true})
	err := dtc.createMonitoring("cfg", "conn", "client", "sub")
	if err == nil {
		t.Fatal("expected error for no locations, got nil")
	}
	if !strings.Contains(err.Error(), "no locations") {
		t.Errorf("error %q does not mention no locations", err.Error())
	}
}

func TestSDKCreateMonitoring_NoEssentialFeatureSets(t *testing.T) {
	dtc, _ := newMonitoringTestClient(t, monitoringServerOpts{noEssential: true})
	err := dtc.createMonitoring("cfg", "conn", "client", "sub")
	if err == nil {
		t.Fatal("expected error for no essential feature sets, got nil")
	}
	if !strings.Contains(err.Error(), "_essential") {
		t.Errorf("error %q does not mention _essential feature sets", err.Error())
	}
}

func TestSDKCreateMonitoring_PostFails(t *testing.T) {
	dtc, _ := newMonitoringTestClient(t, monitoringServerOpts{postErr: true})
	err := dtc.createMonitoring("cfg", "conn", "client", "sub")
	if err == nil {
		t.Fatal("expected error when POST fails, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error %q does not mention 400", err.Error())
	}
}

// ─── updateMonitoring ─────────────────────────────────────────────────────────

func TestSDKUpdateMonitoring_HappyPath(t *testing.T) {
	var body map[string]interface{}
	dtc, sdk := newMonitoringTestClient(t, monitoringServerOpts{})

	if err := dtc.updateMonitoring("mon-existing-1", "cfg-name", "conn-obj-001", "client-app-001", "sub-abc123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// PUT must target the existing config ID, not a bare POST collection URL.
	if sdk.UpdateConfigID != "mon-existing-1" {
		t.Errorf("update config ID = %q, want mon-existing-1", sdk.UpdateConfigID)
	}
	if err := extensiontest.DecodeBody(sdk.UpdateBody, &body); err != nil {
		t.Fatalf("decode monitoring body: %v", err)
	}
	// Shares the create body builder: same schema-derived defaults.
	val, _ := body["value"].(map[string]interface{})
	if val == nil {
		t.Fatal("body.value missing")
	}
	if fs, _ := val["featureSets"].([]interface{}); len(fs) != 2 {
		t.Errorf("expected 2 essential featureSets, got %v", val["featureSets"])
	}
}

func TestSDKUpdateMonitoring_PutFails(t *testing.T) {
	dtc, _ := newMonitoringTestClient(t, monitoringServerOpts{putErr: true})
	err := dtc.updateMonitoring("mon-1", "cfg", "conn", "client", "sub")
	if err == nil {
		t.Fatal("expected error when PUT fails, got nil")
	}
}

func TestSDKUpdateMonitoring_EmptyEnumsFailFast(t *testing.T) {
	dtc, _ := newMonitoringTestClient(t, monitoringServerOpts{noEssential: true})
	err := dtc.updateMonitoring("mon-1", "cfg", "conn", "client", "sub")
	if err == nil {
		t.Fatal("expected error for no essential feature sets, got nil")
	}
	if !strings.Contains(err.Error(), "_essential") {
		t.Errorf("error %q does not mention _essential feature sets", err.Error())
	}
}

// ─── findAllMonitoringConfigs ─────────────────────────────────────────────────

func TestSDKFindAllMonitoringConfigs_Found(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{MonitoringConfigs: []extensiontest.MonitoringConfiguration{
		{ObjectID: "mon-001", Value: []byte(`{"description":"my-config"}`)},
		{ObjectID: "mon-002", Value: []byte(`{"description":"other-config"}`)},
	}}

	ids, err := newExtensionTestClient(sdk).findAllMonitoringConfigs("my-config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "mon-001" {
		t.Errorf("ids = %v, want [mon-001]", ids)
	}
}

func TestSDKFindAllMonitoringConfigs_NotFound(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{}

	ids, err := newExtensionTestClient(sdk).findAllMonitoringConfigs("missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected no ids for not-found, got %v", ids)
	}
}

func TestSDKFindAllMonitoringConfigs_ServerError(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{ListErr: errors.New("401 unauthorized")}

	_, err := newExtensionTestClient(sdk).findAllMonitoringConfigs("my-config")
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q does not mention 401", err.Error())
	}
}

// ─── deleteMonitoring ─────────────────────────────────────────────────────────

func TestSDKDeleteMonitoring_HappyPath(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{}

	if err := newExtensionTestClient(sdk).deleteMonitoring("mon-001"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !sdk.DeleteCalled {
		t.Fatal("expected DeleteMonitoringConfiguration to be called")
	}
	if sdk.DeleteConfigID != "mon-001" {
		t.Errorf("delete config ID = %q, want mon-001", sdk.DeleteConfigID)
	}
}

func TestSDKDeleteMonitoring_NotFound(t *testing.T) {
	// 404 means the config is already gone — deleteMonitoring must treat it as success.
	sdk := &extensiontest.FakeExtensionAPI{DeleteErr: errors.New("404 not found")}

	if err := newExtensionTestClient(sdk).deleteMonitoring("mon-001"); err != nil {
		t.Errorf("404 must be treated as already-deleted (success), got: %v", err)
	}
}

func TestSDKDeleteMonitoring_ServerError(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{DeleteErr: errors.New("500 internal server error")}

	if err := newExtensionTestClient(sdk).deleteMonitoring("mon-001"); err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}
