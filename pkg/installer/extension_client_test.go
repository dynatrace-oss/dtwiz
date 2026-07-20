package installer

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testExtensionName = "com.dynatrace.extension.test-ext"

func newTestExtensionClient(t *testing.T, serverURL string) *ExtensionClient {
	t.Helper()
	e, err := NewExtensionClient(serverURL, "dt0s16.test")
	if err != nil {
		t.Fatalf("create test extension client: %v", err)
	}
	e.C.HTTP().SetRetryCount(0)
	return e
}

// ─── cmpSemver ─────────────────────────────────────────────────────────────

func TestCmpSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.2.3", "1.2.4", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.10.0", "1.9.0", 1}, // numeric, not lexicographic
		{"1.0.11", "1.0.9", 1}, // numeric: 11 > 9
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

// ─── DeleteConnection ────────────────────────────────────────────────────────

func TestExtensionClientDeleteConnection_HappyPath(t *testing.T) {
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

	if err := newTestExtensionClient(t, srv.URL).DeleteConnection("obj-001"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExtensionClientDeleteConnection_404IsIdempotent(t *testing.T) {
	deleteCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).DeleteConnection("obj-001"); err != nil {
		t.Errorf("404 on GET must be treated as already deleted, got: %v", err)
	}
	if deleteCalled {
		t.Error("DELETE must not be issued when GET returns 404")
	}
}

// ─── InstallExtension ───────────────────────────────────────────────────────

func TestExtensionClientInstallExtension_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		wantPath := "/platform/extensions/v2/extensions/" + testExtensionName
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		if got := r.URL.Query().Get("version"); got != "1.2.3" {
			t.Errorf("version query = %q, want 1.2.3", got)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"extensionName":"` + testExtensionName + `","version":"1.2.3"}`))
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).InstallExtension(testExtensionName, "1.2.3"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtensionClientInstallExtension_AlreadyInstalledIsSuccess(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusConflict} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"extension already installed"}}`))
		}))
		if err := newTestExtensionClient(t, srv.URL).InstallExtension(testExtensionName, "1.2.3"); err != nil {
			t.Errorf("status %d should be idempotent success, got: %v", status, err)
		}
		srv.Close()
	}
}

// ─── LatestExtensionVersion ──────────────────────────────────────────────────

func TestExtensionClientLatestExtensionVersion_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"version":"1.2.0"},{"version":"1.0.0"},{"version":"1.1.3"}]}`))
	}))
	defer srv.Close()

	v, err := newTestExtensionClient(t, srv.URL).LatestExtensionVersion(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "1.2.0" {
		t.Errorf("version = %q, want %q", v, "1.2.0")
	}
}

func TestExtensionClientLatestExtensionVersion_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	_, err := newTestExtensionClient(t, srv.URL).LatestExtensionVersion(testExtensionName)
	if err == nil {
		t.Fatal("expected error for empty versions, got nil")
	}
}

// TestExtensionClientLatestExtensionVersion_AllBlankVersions guards against a
// regression where items are present but every version string is empty: the
// filtered slice is empty and indexing versions[0] would panic.
func TestExtensionClientLatestExtensionVersion_AllBlankVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"version":""},{"version":""}]}`))
	}))
	defer srv.Close()

	_, err := newTestExtensionClient(t, srv.URL).LatestExtensionVersion(testExtensionName)
	if err == nil {
		t.Fatal("expected error for all-blank versions, got nil")
	}
}

func TestExtensionClientLatestExtensionVersion_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newTestExtensionClient(t, srv.URL).LatestExtensionVersion(testExtensionName)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

// ─── FetchExtensionSchema / EnumValues ───────────────────────────────────────

func TestExtensionClientFetchExtensionSchema_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/platform/extensions/v2/extensions/" + testExtensionName + "/1.2.0/schema"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"enums":{
			"someLocationEnum":{"items":[{"value":"eastus"},{"value":"westeurope"}]},
			"FeatureSetsType":{"items":[{"value":"essential_one"},{"value":""}]}
		}}`))
	}))
	defer srv.Close()

	schema, err := newTestExtensionClient(t, srv.URL).FetchExtensionSchema(testExtensionName, "1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	locs := schema.EnumValues("someLocationEnum")
	if len(locs) != 2 {
		t.Errorf("expected 2 locations, got %d: %v", len(locs), locs)
	}
	// blank value must be filtered out
	fs := schema.EnumValues("FeatureSetsType")
	if len(fs) != 1 || fs[0] != "essential_one" {
		t.Errorf("expected [essential_one], got: %v", fs)
	}
	// missing key returns nil
	if schema.EnumValues("nonexistent-key") != nil {
		t.Error("expected nil for missing enum key")
	}
}

func TestExtensionClientFetchExtensionSchema_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := newTestExtensionClient(t, srv.URL).FetchExtensionSchema(testExtensionName, "1.2.0")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

// ─── FindAllMonitoringConfigs / DeleteMonitoringConfiguration ────────────────

func TestExtensionClientFindAllMonitoringConfigs_Found(t *testing.T) {
	wantPath := "/platform/extensions/v2/extensions/" + testExtensionName + "/monitoring-configurations"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"objectId":"mon-001","value":{"description":"my-config"}},
			{"objectId":"mon-002","value":{"description":"other-config"}}
		]}`))
	}))
	defer srv.Close()

	ids, err := newTestExtensionClient(t, srv.URL).FindAllMonitoringConfigs(testExtensionName, "my-config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "mon-001" {
		t.Errorf("ids = %v, want [mon-001]", ids)
	}
}

func TestExtensionClientDeleteMonitoringConfiguration_HappyPath(t *testing.T) {
	wantPath := "/platform/extensions/v2/extensions/" + testExtensionName + "/monitoring-configurations/mon-001"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).DeleteMonitoringConfiguration(testExtensionName, "mon-001"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// The extension API's DeleteMonitoringConfiguration does not surface 404s as
// httpclient.ErrNotFound (unlike the settings API used by DeleteConnection), so
// this currently returns an error rather than treating it as already-deleted.
func TestExtensionClientDeleteMonitoringConfiguration_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := newTestExtensionClient(t, srv.URL).DeleteMonitoringConfiguration(testExtensionName, "mon-001")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

// ─── EssentialFeatureSets ────────────────────────────────────────────────────

func TestExtensionSchema_EssentialFeatureSets_HappyPath(t *testing.T) {
	schema := &ExtensionSchema{
		Enums: map[string]struct {
			Items []struct {
				Value string `json:"value"`
			} `json:"items"`
		}{
			"FSTypes": {Items: []struct {
				Value string `json:"value"`
			}{
				{Value: "compute_essential"},
				{Value: "storage_essential"},
				{Value: "compute_premium"},
			}},
		},
	}

	fs, err := schema.EssentialFeatureSets("FSTypes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("expected 2 essential feature sets, got %d: %v", len(fs), fs)
	}
	for _, f := range fs {
		if !strings.HasSuffix(f, "_essential") {
			t.Errorf("non-essential value returned: %q", f)
		}
	}
}

func TestExtensionSchema_EssentialFeatureSets_NoEssential(t *testing.T) {
	schema := &ExtensionSchema{
		Enums: map[string]struct {
			Items []struct {
				Value string `json:"value"`
			} `json:"items"`
		}{
			"FSTypes": {Items: []struct {
				Value string `json:"value"`
			}{
				{Value: "compute_premium"},
				{Value: "storage_premium"},
			}},
		},
	}

	_, err := schema.EssentialFeatureSets("FSTypes")
	if err == nil {
		t.Fatal("expected error when no _essential feature sets exist, got nil")
	}
	if !strings.Contains(err.Error(), "_essential") {
		t.Errorf("error should mention _essential, got: %v", err)
	}
}

func TestExtensionSchema_EssentialFeatureSets_KeyNotFound(t *testing.T) {
	schema := &ExtensionSchema{
		Enums: map[string]struct {
			Items []struct {
				Value string `json:"value"`
			} `json:"items"`
		}{},
	}

	_, err := schema.EssentialFeatureSets("NonExistent")
	if err == nil {
		t.Fatal("expected error for missing enum key, got nil")
	}
}
