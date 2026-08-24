package gcp

import (
	"encoding/json"
	"errors"
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

// ─── dtServiceAccount ─────────────────────────────────────────────────────────

func TestSDKDTServiceAccount_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("schemaIds") != dtPrincipalSchemaID {
			t.Errorf("schemaIds = %q, want %q", r.URL.Query().Get("schemaIds"), dtPrincipalSchemaID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"objectId":"p-1","value":{"dynatracePrincipal":"dt-monitor@dynatrace-prod.iam.gserviceaccount.com"}}]}`))
	}))
	defer srv.Close()

	email, err := newTestSDKClient(t, srv.URL).dtServiceAccount()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if email != "dt-monitor@dynatrace-prod.iam.gserviceaccount.com" {
		t.Errorf("email = %q", email)
	}
}

func TestSDKDTServiceAccount_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	_, err := newTestSDKClient(t, srv.URL).dtServiceAccount()
	if !errors.Is(err, errNoPrincipal) {
		t.Fatalf("expected errNoPrincipal, got %v", err)
	}
}

// ─── findAllConnections ───────────────────────────────────────────────────────

func TestSDKFindAllConnections_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("schemaIds") != connectionSchemaID {
			t.Errorf("schemaIds query param missing or wrong: %q", r.URL.Query().Get("schemaIds"))
		}
		// Regression guard: filtering by scopes=environment was confirmed live to
		// return zero results for this schema even for objects scoped to "environment" —
		// the request must not send a scopes filter at all.
		if _, present := r.URL.Query()["scopes"]; present {
			t.Errorf("scopes query param must be omitted, got %q", r.URL.Query().Get("scopes"))
		}
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{
			"items": []map[string]any{{
				"objectId": "obj-001",
				"value": map[string]any{
					"name": "my-conn",
					"type": connectionType,
					connectionType: map[string]any{
						"serviceAccountId": "dtwiz-gcp@my-project.iam.gserviceaccount.com",
						"consumers":        []string{"SVC:com.dynatrace.da"},
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
	if conns[0].serviceAccountEmail != "dtwiz-gcp@my-project.iam.gserviceaccount.com" {
		t.Errorf("serviceAccountEmail = %q", conns[0].serviceAccountEmail)
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

func TestSDKDeleteConnection_404IsIdempotent(t *testing.T) {
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

// ─── updateConnection ─────────────────────────────────────────────────────────

func TestSDKUpdateConnection_HappyPath(t *testing.T) {
	var putBody map[string]any
	var putIfMatch string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"schemaVersion":"42","value":{"name":"my-conn","type":"serviceAccountImpersonation"}}`))
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

	err := newTestSDKClient(t, srv.URL).updateConnection("obj-001", "my-conn", "dtwiz-gcp@my-project.iam.gserviceaccount.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if putIfMatch != "42" {
		t.Errorf("If-Match = %q, want %q", putIfMatch, "42")
	}
	val, ok := putBody["value"].(map[string]any)
	if !ok {
		t.Fatalf("put body.value missing or wrong type: %T", putBody["value"])
	}
	impersonation, ok := val[connectionType].(map[string]any)
	if !ok {
		t.Fatalf("%s block missing: %+v", connectionType, val)
	}
	if impersonation["serviceAccountId"] != "dtwiz-gcp@my-project.iam.gserviceaccount.com" {
		t.Errorf("serviceAccountId = %v", impersonation["serviceAccountId"])
	}
}

func TestSDKUpdateConnection_SurfacesConstraintViolationDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"schemaVersion":"42","value":{"name":"my-conn","type":"serviceAccountImpersonation"}}`))
		case http.MethodPut:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":400,"message":"Constraints violated.","constraintViolations":[{"path":"builtin:hyperscaler-authentication.connections.gcp/0/serviceAccountImpersonation/serviceAccount","message":"Unknown property"}]}}`))
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
	}))
	defer srv.Close()

	err := newTestSDKClient(t, srv.URL).updateConnection("obj-001", "my-conn", "dtwiz-gcp@my-project.iam.gserviceaccount.com")
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if !strings.Contains(err.Error(), "Unknown property") {
		t.Errorf("error %q does not surface the constraint violation detail the SDK's generic message discards", err.Error())
	}
}

// cmpSemver and latestExtensionVersion are shared logic covered by
// pkg/installer's own test suite; createMonitoring below still exercises them
// end-to-end through buildMonitoringConfig.

// ─── createMonitoring ─────────────────────────────────────────────────────────

// stockDAGCPSchemaJSON is a minimal da-gcp schema with 3 feature sets
// (2 _essential, 1 _premium that must be excluded by createMonitoring).
const stockDAGCPSchemaJSON = `{"enums":{
	"FeatureSetsType":{"items":[{"value":"compute_engine_essential"},{"value":"cloud_sql_essential"},{"value":"cloud_run_premium"}]}
}}`

type monitoringServerOpts struct {
	noVersions     bool
	schemaErr      bool
	noEssential    bool
	postErr        bool
	putErr         bool
	captureBody    any
	capturePutPath *string
}

func newMonitoringTestServer(t *testing.T, opts monitoringServerOpts) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == extensionAPI+"/environment-configuration":
			_, _ = w.Write([]byte(`{"version":"1.2.0"}`))
		case r.Method == http.MethodGet && r.URL.Path == extensionAPI:
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
			if opts.noEssential {
				_, _ = w.Write([]byte(`{"enums":{"FeatureSetsType":{"items":[{"value":"compute_engine_premium"}]}}}`))
				return
			}
			_, _ = w.Write([]byte(stockDAGCPSchemaJSON))
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
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, monitoringAPI+"/"):
			if opts.putErr {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if opts.capturePutPath != nil {
				*opts.capturePutPath = r.URL.Path
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
	var body map[string]any
	srv := newMonitoringTestServer(t, monitoringServerOpts{captureBody: &body})
	defer srv.Close()

	if err := newTestSDKClient(t, srv.URL).createMonitoring("cfg-name", "conn-obj-001", "dtwiz-gcp@my-project.iam.gserviceaccount.com", "my-project"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body["scope"] != monitoringScope {
		t.Errorf("scope = %v, want %v", body["scope"], monitoringScope)
	}
	val, _ := body["value"].(map[string]any)
	if val == nil {
		t.Fatal("body.value missing")
	}
	if val["enabled"] != true {
		t.Errorf("enabled = %v, want true", val["enabled"])
	}

	// only _essential feature sets included
	fs, _ := val["featureSets"].([]any)
	if len(fs) != 2 {
		t.Errorf("expected 2 essential featureSets, got %d: %v", len(fs), fs)
	}
	for _, f := range fs {
		if strings.HasSuffix(f.(string), "_premium") {
			t.Errorf("featureSets must not include _premium, got: %v", fs)
		}
	}

	// Block key is "googleCloud" (not "gcp") to match the da-gcp schema and dtctl.
	gcpBlock, _ := val["googleCloud"].(map[string]any)
	if gcpBlock == nil {
		t.Fatal("googleCloud block missing")
	}
	if _, present := gcpBlock["projectFilteringMode"]; present {
		t.Errorf("projectFilteringMode must not be sent (not in schema), got: %v", gcpBlock["projectFilteringMode"])
	}
	projects, _ := gcpBlock["projectFiltering"].([]any)
	if len(projects) != 1 || projects[0] != "my-project" {
		t.Errorf("projectFiltering = %v, want [my-project]", projects)
	}
	creds, _ := gcpBlock["credentials"].([]any)
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	cred, _ := creds[0].(map[string]any)
	if cred["connectionId"] != "conn-obj-001" {
		t.Errorf("connectionId = %v", cred["connectionId"])
	}
	// Credential key is "serviceAccount" (not "serviceAccountId"), and no "type" field exists.
	if cred["serviceAccount"] != "dtwiz-gcp@my-project.iam.gserviceaccount.com" {
		t.Errorf("serviceAccount = %v", cred["serviceAccount"])
	}
	if _, present := cred["type"]; present {
		t.Errorf("credential type must not be sent (not in schema), got: %v", cred["type"])
	}
}

func TestSDKCreateMonitoring_NoVersions(t *testing.T) {
	srv := newMonitoringTestServer(t, monitoringServerOpts{noVersions: true})
	defer srv.Close()
	if err := newTestSDKClient(t, srv.URL).createMonitoring("cfg", "conn", "sa", "proj"); err == nil {
		t.Fatal("expected error for no versions, got nil")
	}
}

func TestSDKCreateMonitoring_NoEssentialFeatureSets(t *testing.T) {
	srv := newMonitoringTestServer(t, monitoringServerOpts{noEssential: true})
	defer srv.Close()
	err := newTestSDKClient(t, srv.URL).createMonitoring("cfg", "conn", "sa", "proj")
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
	if err := newTestSDKClient(t, srv.URL).createMonitoring("cfg", "conn", "sa", "proj"); err == nil {
		t.Fatal("expected error when POST fails, got nil")
	}
}

// ─── updateMonitoring ─────────────────────────────────────────────────────────

func TestSDKUpdateMonitoring_HappyPath(t *testing.T) {
	var body map[string]any
	var putPath string
	srv := newMonitoringTestServer(t, monitoringServerOpts{captureBody: &body, capturePutPath: &putPath})
	defer srv.Close()

	if err := newTestSDKClient(t, srv.URL).updateMonitoring("mon-existing-1", "cfg-name", "conn-obj-001", "sa@p.iam.gserviceaccount.com", "my-project"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(putPath, "/mon-existing-1") {
		t.Errorf("PUT path = %q, want suffix /mon-existing-1", putPath)
	}
}

// ─── findAllMonitoringConfigs ─────────────────────────────────────────────────

func TestSDKFindAllMonitoringConfigs_Found(t *testing.T) {
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

	ids, err := newTestSDKClient(t, srv.URL).findAllMonitoringConfigs("my-config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "mon-001" {
		t.Errorf("ids = %v, want [mon-001]", ids)
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

// ─── installExtension ────────────────────────────────────────────────────────

func TestSDKInstallExtension_AlreadyInstalled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == extensionAPI+"/environment-configuration" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"1.2.0"}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == extensionAPI {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"version":"1.2.0"}]}`))
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	fresh, err := newTestSDKClient(t, srv.URL).installExtension()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fresh {
		t.Error("expected fresh=false for already-installed extension")
	}
}

func TestSDKInstallExtension_FreshInstall(t *testing.T) {
	installCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == extensionAPI:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Extension not found"}}`))
		case r.Method == http.MethodPost && r.URL.Path == extensionAPI:
			installCalled = true
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"extensionName":"com.dynatrace.extension.da-gcp","version":"1.2.0"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	fresh, err := newTestSDKClient(t, srv.URL).installExtension()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fresh {
		t.Error("expected fresh=true after hub install")
	}
	if !installCalled {
		t.Error("expected InstallFromHub to be called")
	}
}

// ─── isExtensionActive ───────────────────────────────────────────────────────

func TestSDKIsExtensionActive_Active(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"version":"1.2.0","active":true}]}`))
	}))
	defer srv.Close()

	active, err := newTestSDKClient(t, srv.URL).isExtensionActive()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !active {
		t.Error("expected active=true when extension version has active:true")
	}
}

func TestSDKIsExtensionActive_NotActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"version":"1.2.0"}]}`))
	}))
	defer srv.Close()

	active, err := newTestSDKClient(t, srv.URL).isExtensionActive()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected active=false when active field is absent")
	}
}
