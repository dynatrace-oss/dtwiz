package installer

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/internal/extensiontest"
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

func newTestExtensionClientWithSDK(sdk ExtensionAPI) *ExtensionClient {
	return &ExtensionClient{Extension: sdk}
}

// ─── cmpSemver ─────────────────────────────────────────────────────────────

func TestCmpSemver(t *testing.T) {
	t.Parallel()

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
		tc := tc
		t.Run(tc.a+"/"+tc.b, func(t *testing.T) {
			t.Parallel()

			if got := cmpSemver(tc.a, tc.b); got != tc.want {
				t.Errorf("cmpSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestParseConstraintViolations(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"error": {
			"message": "Constraints violated.",
			"constraintViolations": [
				{"path":"value.enabled","message":"must not be null"},
				{"path":"value.aws.credentials[0].accountId","message":"invalid account"}
			]
		}
	}`)

	got := ParseConstraintViolations(body)
	if len(got) != 2 {
		t.Fatalf("ParseConstraintViolations() returned %d violations, want 2", len(got))
	}
	if got[0].Path != "value.enabled" || got[0].Message != "must not be null" {
		t.Fatalf("first violation = %#v, want value.enabled message", got[0])
	}
	if got[1].Path != "value.aws.credentials[0].accountId" || got[1].Message != "invalid account" {
		t.Fatalf("second violation = %#v, want account message", got[1])
	}
}

func TestParseConstraintViolations_InvalidBody(t *testing.T) {
	t.Parallel()

	if got := ParseConstraintViolations([]byte(`not-json`)); got != nil {
		t.Fatalf("ParseConstraintViolations(invalid) = %#v, want nil", got)
	}
}

func TestFormatConstraintViolations(t *testing.T) {
	t.Parallel()

	got := FormatConstraintViolations([]ConstraintViolation{
		{Path: "value.enabled", Message: "must not be null"},
		{Path: "value.aws.credentials[0].accountId", Message: "invalid account"},
	})
	want := "value.enabled: must not be null; value.aws.credentials[0].accountId: invalid account"
	if got != want {
		t.Fatalf("FormatConstraintViolations() = %q, want %q", got, want)
	}
}

func TestWithScopeHint(t *testing.T) {
	t.Parallel()

	if err := WithScopeHint(nil, "settings:objects:read"); err != nil {
		t.Fatalf("WithScopeHint(nil) = %v, want nil", err)
	}

	orig := errors.New("boom")
	if got := WithScopeHint(orig, "settings:objects:read"); !errors.Is(got, orig) || strings.Contains(got.Error(), "platform token") {
		t.Fatalf("WithScopeHint(non-auth) = %v, want original without scope hint", got)
	}

	for _, err := range []error{httpclient.ErrForbidden, httpclient.ErrUnauthorized} {
		got := WithScopeHint(err, "settings:objects:write")
		if !errors.Is(got, err) || !strings.Contains(got.Error(), "settings:objects:write") {
			t.Fatalf("WithScopeHint(%v) = %v, want wrapped scope hint", err, got)
		}
	}
}

// ─── ActivateExtension ──────────────────────────────────────────────────────

func TestExtensionClientActivateExtension_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		wantPath := "/platform/extensions/v2/extensions/" + testExtensionName + "/environment-configuration"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"1.2.3"}`))
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).ActivateExtension(testExtensionName, "1.2.3"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtensionClientActivateExtension_ConflictIsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"message":"version already active"}}`))
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).ActivateExtension(testExtensionName, "1.2.3"); err != nil {
		t.Errorf("409 Conflict must be treated as success, got: %v", err)
	}
}

func TestExtensionClientActivateExtension_APIErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"insufficient permissions"}}`))
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).ActivateExtension(testExtensionName, "1.2.3"); err == nil {
		t.Fatal("expected error for 403, got nil")
	}
}

// ─── DeactivateExtension ─────────────────────────────────────────────────────

func TestExtensionClientDeactivateExtension_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		wantPath := "/platform/extensions/v2/extensions/" + testExtensionName + "/environment-configuration"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).DeactivateExtension(testExtensionName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtensionClientDeactivateExtension_404IsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"no environment configuration"}}`))
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).DeactivateExtension(testExtensionName); err != nil {
		t.Errorf("404 must be treated as already inactive, got: %v", err)
	}
}

func TestExtensionClientDeactivateExtension_APIErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"insufficient permissions"}}`))
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).DeactivateExtension(testExtensionName); err == nil {
		t.Fatal("expected error for 403, got nil")
	}
}

// ─── DeleteExtensionVersion ──────────────────────────────────────────────────

func TestExtensionClientDeleteExtensionVersion_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		wantPath := "/platform/extensions/v2/extensions/" + testExtensionName + "/1.2.3"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"extensionName":"` + testExtensionName + `","version":"1.2.3"}`))
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).DeleteExtensionVersion(testExtensionName, "1.2.3"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtensionClientDeleteExtensionVersion_404IsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"extension version not found"}}`))
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).DeleteExtensionVersion(testExtensionName, "1.2.3"); err != nil {
		t.Errorf("404 must be treated as already deleted, got: %v", err)
	}
}

func TestExtensionClientDeleteExtensionVersion_APIErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"insufficient permissions"}}`))
	}))
	defer srv.Close()

	if err := newTestExtensionClient(t, srv.URL).DeleteExtensionVersion(testExtensionName, "1.2.3"); err == nil {
		t.Fatal("expected error for 403, got nil")
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
	sdk := &extensiontest.FakeExtensionAPI{}

	if err := newTestExtensionClientWithSDK(sdk).InstallExtension(testExtensionName, "1.2.3"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sdk.InstallCalled {
		t.Fatal("expected InstallFromHub to be called")
	}
	if sdk.InstallVersion != "1.2.3" {
		t.Errorf("install version = %q, want 1.2.3", sdk.InstallVersion)
	}
}

func TestExtensionClientInstallExtension_AlreadyInstalledIsSuccess(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusConflict} {
		sdk := &extensiontest.FakeExtensionAPI{InstallErr: extensiontest.APIError(status, "extension already installed")}
		if err := newTestExtensionClientWithSDK(sdk).InstallExtension(testExtensionName, "1.2.3"); err != nil {
			t.Errorf("status %d should be idempotent success, got: %v", status, err)
		}
	}
}

// ─── EnsureInstalled ────────────────────────────────────────────────────────

func TestExtensionClientEnsureInstalled_AlreadyInstalled(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{Versions: extensiontest.Versions("1.2.0")}

	fresh, err := newTestExtensionClientWithSDK(sdk).EnsureInstalled(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fresh {
		t.Error("expected fresh=false for already-installed extension")
	}
}

func TestExtensionClientEnsureInstalled_NotFoundInstalls(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{GetErr: extensiontest.NotFound(testExtensionName)}

	fresh, err := newTestExtensionClientWithSDK(sdk).EnsureInstalled(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fresh {
		t.Error("expected fresh=true after hub install")
	}
	if !sdk.InstallCalled {
		t.Error("expected InstallFromHub to be called")
	}
}

func TestExtensionClientEnsureInstalled_PropagatesVersionCheckError(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{GetErr: errors.New("temporary failure")}

	if _, err := newTestExtensionClientWithSDK(sdk).EnsureInstalled(testExtensionName); err == nil {
		t.Fatal("expected error for version-check failure")
	}
	if sdk.InstallCalled {
		t.Error("InstallFromHub must not be called when the version check fails")
	}
}

// ─── LatestExtensionVersion ──────────────────────────────────────────────────

func TestExtensionClientLatestExtensionVersion_HappyPath(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{Versions: extensiontest.Versions("1.2.0", "1.0.0", "1.1.3")}

	v, err := newTestExtensionClientWithSDK(sdk).LatestExtensionVersion(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "1.2.0" {
		t.Errorf("version = %q, want %q", v, "1.2.0")
	}
}

func TestExtensionClientLatestExtensionVersion_Empty(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{}

	_, err := newTestExtensionClientWithSDK(sdk).LatestExtensionVersion(testExtensionName)
	if err == nil {
		t.Fatal("expected error for empty versions, got nil")
	}
}

// TestExtensionClientLatestExtensionVersion_AllBlankVersions guards against a
// regression where items are present but every version string is empty: the
// filtered slice is empty and indexing versions[0] would panic.
func TestExtensionClientLatestExtensionVersion_AllBlankVersions(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{Versions: extensiontest.Versions("", "")}

	_, err := newTestExtensionClientWithSDK(sdk).LatestExtensionVersion(testExtensionName)
	if err == nil {
		t.Fatal("expected error for all-blank versions, got nil")
	}
}

func TestExtensionClientLatestExtensionVersion_ServerError(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{GetErr: errors.New("temporary failure")}

	_, err := newTestExtensionClientWithSDK(sdk).LatestExtensionVersion(testExtensionName)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

// ─── IsExtensionActive ───────────────────────────────────────────────────────

func TestExtensionClientIsExtensionActive_Active(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{ActiveVersion: "1.2.0"}

	active, err := newTestExtensionClientWithSDK(sdk).IsExtensionActive(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !active {
		t.Error("expected active=true when environment configuration exists")
	}
}

func TestExtensionClientIsExtensionActive_NotActive(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{}

	active, err := newTestExtensionClientWithSDK(sdk).IsExtensionActive(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected active=false when environment configuration is absent")
	}
}

func TestExtensionClientIsExtensionActive_ActiveLookupFailureIsInactive(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{ActiveErr: errors.New("temporary failure")}

	active, err := newTestExtensionClientWithSDK(sdk).IsExtensionActive(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected active=false when environment configuration lookup fails")
	}
}

// ─── GetStatus ────────────────────────────────────────────────────────────────

func TestExtensionClientGetStatus_NotInstalled(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{GetErr: extensiontest.NotFound(testExtensionName)}

	status, err := newTestExtensionClientWithSDK(sdk).GetStatus(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != ExtensionNotInstalled {
		t.Errorf("status = %v, want ExtensionNotInstalled", status)
	}
}

func TestExtensionClientGetStatus_InstalledInactive(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{Versions: extensiontest.Versions("1.2.0")}

	status, err := newTestExtensionClientWithSDK(sdk).GetStatus(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != ExtensionInstalledInactive {
		t.Errorf("status = %v, want ExtensionInstalledInactive", status)
	}
}

func TestExtensionClientGetStatus_InstalledActive(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{Versions: extensiontest.Versions("1.2.0"), ActiveVersion: "1.2.0"}

	status, err := newTestExtensionClientWithSDK(sdk).GetStatus(testExtensionName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != ExtensionInstalledActive {
		t.Errorf("status = %v, want ExtensionInstalledActive", status)
	}
}

func TestExtensionClientGetStatus_ServerError(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{GetErr: errors.New("temporary failure")}

	if _, err := newTestExtensionClientWithSDK(sdk).GetStatus(testExtensionName); err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

// ─── FetchExtensionSchema / EnumValues ───────────────────────────────────────

func TestExtensionClientFetchExtensionSchema_HappyPath(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{Schema: []byte(`{"enums":{
			"someLocationEnum":{"items":[{"value":"eastus"},{"value":"westeurope"}]},
			"FeatureSetsType":{"items":[{"value":"essential_one"},{"value":""}]}
		}}`)}

	schema, err := newTestExtensionClientWithSDK(sdk).FetchExtensionSchema(testExtensionName, "1.2.0")
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
	sdk := &extensiontest.FakeExtensionAPI{SchemaErr: errors.New("schema not found")}

	_, err := newTestExtensionClientWithSDK(sdk).FetchExtensionSchema(testExtensionName, "1.2.0")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

// ─── FindAllMonitoringConfigs / DeleteMonitoringConfiguration ────────────────

func TestExtensionClientFindAllMonitoringConfigs_Found(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{MonitoringConfigs: []extensiontest.MonitoringConfiguration{
		{ObjectID: "mon-001", Value: []byte(`{"description":"my-config"}`)},
		{ObjectID: "mon-002", Value: []byte(`{"description":"other-config"}`)},
	}}

	ids, err := newTestExtensionClientWithSDK(sdk).FindAllMonitoringConfigs(testExtensionName, "my-config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "mon-001" {
		t.Errorf("ids = %v, want [mon-001]", ids)
	}
}

func TestExtensionClientDeleteMonitoringConfiguration_HappyPath(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{}

	if err := newTestExtensionClientWithSDK(sdk).DeleteMonitoringConfiguration(testExtensionName, "mon-001"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !sdk.DeleteCalled {
		t.Fatal("expected DeleteMonitoringConfiguration to be called")
	}
	if sdk.DeleteConfigID != "mon-001" {
		t.Errorf("delete config ID = %q, want mon-001", sdk.DeleteConfigID)
	}
}

func TestExtensionClientDeleteMonitoringConfiguration_ServerError(t *testing.T) {
	sdk := &extensiontest.FakeExtensionAPI{DeleteErr: errors.New("delete failed")}

	err := newTestExtensionClientWithSDK(sdk).DeleteMonitoringConfiguration(testExtensionName, "mon-001")
	if err == nil {
		t.Fatal("expected error for delete failure, got nil")
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
