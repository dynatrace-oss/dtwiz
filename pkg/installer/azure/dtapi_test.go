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
