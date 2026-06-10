package extensions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

func newTestPlatformClient(t *testing.T, serverURL string) *client.PlatformClient {
	t.Helper()
	c, err := client.New(serverURL, serverURL, "dt0s16.test", "dt0s16.test", 0)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	c.Platform.HTTP().SetRetryCount(0)
	return c.Platform
}

// InstallExtension

func TestInstallExtension_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := fmt.Sprintf(extensionPath, "com.example.ext")
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		if body["extensionName"] != "com.example.ext" {
			t.Errorf("extensionName = %q", body["extensionName"])
		}
		if body["version"] != "1.0.0" {
			t.Errorf("version = %q", body["version"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	if err := InstallExtension(newTestPlatformClient(t, srv.URL), "com.example.ext", "1.0.0", false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallExtension_SilentIgnores400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	if err := InstallExtension(newTestPlatformClient(t, srv.URL), "com.example.ext", "1.0.0", true); err != nil {
		t.Errorf("silent=true: expected no error on 400, got: %v", err)
	}
}

func TestInstallExtension_SilentIgnores409(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	if err := InstallExtension(newTestPlatformClient(t, srv.URL), "com.example.ext", "1.0.0", true); err != nil {
		t.Errorf("silent=true: expected no error on 409, got: %v", err)
	}
}

func TestInstallExtension_ErrorOnServerFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := InstallExtension(newTestPlatformClient(t, srv.URL), "com.example.ext", "1.0.0", true)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if want := "500"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

func TestInstallExtension_AcceptsHTTP202(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	if err := InstallExtension(newTestPlatformClient(t, srv.URL), "com.example.ext", "1.0.0", false); err != nil {
		t.Errorf("expected nil error for HTTP 202, got: %v", err)
	}
}

// ListMonitoringConfigs

func TestListMonitoringConfigs_ReturnsSinglePage(t *testing.T) {
	items := []MonitoringConfigItem{
		{ObjectID: "id-1", Value: json.RawMessage(`{"key":"a"}`)},
		{ObjectID: "id-2", Value: json.RawMessage(`{"key":"b"}`)},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := fmt.Sprintf(monitoringConfigsPath, "com.example.ext")
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(monitoringConfigPage{Items: items}) //nolint:errcheck
	}))
	defer srv.Close()

	got, err := ListMonitoringConfigs(newTestPlatformClient(t, srv.URL), "com.example.ext")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}
	if got[0].ObjectID != "id-1" || got[1].ObjectID != "id-2" {
		t.Errorf("unexpected items: %+v", got)
	}
}

func TestListMonitoringConfigs_FollowsPagination(t *testing.T) {
	page1 := monitoringConfigPage{
		Items:       []MonitoringConfigItem{{ObjectID: "id-1"}},
		NextPageKey: "page2key",
	}
	page2 := monitoringConfigPage{
		Items: []MonitoringConfigItem{{ObjectID: "id-2"}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("nextPageKey") == "page2key" {
			json.NewEncoder(w).Encode(page2) //nolint:errcheck
		} else {
			json.NewEncoder(w).Encode(page1) //nolint:errcheck
		}
	}))
	defer srv.Close()

	got, err := ListMonitoringConfigs(newTestPlatformClient(t, srv.URL), "com.example.ext")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}
}

func TestListMonitoringConfigs_ErrorOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := ListMonitoringConfigs(newTestPlatformClient(t, srv.URL), "com.example.ext")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if want := "403"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

// CreateMonitoringConfig

func TestCreateMonitoringConfig_ReturnsObjectID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"objectId": "new-uuid"}) //nolint:errcheck
	}))
	defer srv.Close()

	id, err := CreateMonitoringConfig(newTestPlatformClient(t, srv.URL), "com.example.ext", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "new-uuid" {
		t.Errorf("objectId = %q, want %q", id, "new-uuid")
	}
}

func TestCreateMonitoringConfig_ErrorOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := CreateMonitoringConfig(newTestPlatformClient(t, srv.URL), "com.example.ext", nil)
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if want := "401"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

// DeleteMonitoringConfig

func TestDeleteMonitoringConfig_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := fmt.Sprintf(monitoringConfigsPath+"/obj-123", "com.example.ext")
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := DeleteMonitoringConfig(newTestPlatformClient(t, srv.URL), "com.example.ext", "obj-123"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteMonitoringConfig_ErrorOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := DeleteMonitoringConfig(newTestPlatformClient(t, srv.URL), "com.example.ext", "obj-123")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if want := "404"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

// ListInstalledVersions / GetLatestInstalledVersion

func TestListInstalledVersions_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := fmt.Sprintf(extensionPath, "com.example.ext")
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"version":"1.0.11"},{"version":"1.2.0"}]}`))
	}))
	defer srv.Close()

	items, err := ListInstalledVersions(newTestPlatformClient(t, srv.URL), "com.example.ext")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(items), 2; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
	if items[0].Version != "1.0.11" || items[1].Version != "1.2.0" {
		t.Errorf("versions = %+v", items)
	}
}

func TestListInstalledVersions_ErrorOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := ListInstalledVersions(newTestPlatformClient(t, srv.URL), "com.example.ext")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if want := "404"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

func TestGetLatestInstalledVersion_PicksHighestNumeric(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Deliberately unordered; "1.2.0" should win over "1.0.11" because the
		// second segment dominates and "1.0.11" parses segment-wise (1,0,11).
		_, _ = w.Write([]byte(`{"items":[{"version":"1.0.10"},{"version":"1.2.0"},{"version":"1.0.11"}]}`))
	}))
	defer srv.Close()

	v, err := GetLatestInstalledVersion(newTestPlatformClient(t, srv.URL), "com.example.ext")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "1.2.0"; v != want {
		t.Errorf("version = %q, want %q", v, want)
	}
}

func TestGetLatestInstalledVersion_NumericSortBeatsLexicographic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Lexicographically "1.0.9" > "1.0.11"; numerically 11 > 9.
		_, _ = w.Write([]byte(`{"items":[{"version":"1.0.9"},{"version":"1.0.11"}]}`))
	}))
	defer srv.Close()

	v, err := GetLatestInstalledVersion(newTestPlatformClient(t, srv.URL), "com.example.ext")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "1.0.11"; v != want {
		t.Errorf("version = %q, want %q", v, want)
	}
}

func TestGetLatestInstalledVersion_ErrorWhenNoVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	_, err := GetLatestInstalledVersion(newTestPlatformClient(t, srv.URL), "com.example.ext")
	if err == nil {
		t.Fatal("expected error for empty version list, got nil")
	}
}

// GetMonitoringConfig / UpdateMonitoringConfig

func TestGetMonitoringConfig_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := fmt.Sprintf(monitoringConfigPath, "com.example.ext", "obj-123")
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"scope":"integration-aws","value":{"enabled":false,"aws":{"credentials":[{"enabled":false}]}}}`))
	}))
	defer srv.Close()

	cfg, err := GetMonitoringConfig(newTestPlatformClient(t, srv.URL), "com.example.ext", "obj-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scope != "integration-aws" {
		t.Errorf("scope = %q", cfg.Scope)
	}
	if cfg.Value == nil {
		t.Fatal("value is nil")
	}
	if v, ok := cfg.Value["enabled"].(bool); !ok || v {
		t.Errorf("enabled = %v (ok=%v)", v, ok)
	}
}

func TestGetMonitoringConfig_ErrorOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	_, err := GetMonitoringConfig(newTestPlatformClient(t, srv.URL), "com.example.ext", "obj-123")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if want := "404"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

func TestGetMonitoringConfig_ErrorOnEmptyValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"scope":"integration-aws"}`))
	}))
	defer srv.Close()

	_, err := GetMonitoringConfig(newTestPlatformClient(t, srv.URL), "com.example.ext", "obj-123")
	if err == nil {
		t.Fatal("expected error for empty value, got nil")
	}
}

func TestUpdateMonitoringConfig_SendsScopeAndValue(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := fmt.Sprintf(monitoringConfigPath, "com.example.ext", "obj-123")
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &MonitoringConfig{
		Scope: "integration-aws",
		Value: map[string]interface{}{"enabled": true, "tag": "x"},
	}
	if err := UpdateMonitoringConfig(newTestPlatformClient(t, srv.URL), "com.example.ext", "obj-123", cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received["scope"] != "integration-aws" {
		t.Errorf("scope sent = %v", received["scope"])
	}
	v, ok := received["value"].(map[string]interface{})
	if !ok {
		t.Fatalf("value is %T, want map", received["value"])
	}
	if enabled, _ := v["enabled"].(bool); !enabled {
		t.Errorf("value.enabled = %v, want true", v["enabled"])
	}
}

func TestUpdateMonitoringConfig_ErrorOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	cfg := &MonitoringConfig{Scope: "x", Value: map[string]interface{}{}}
	err := UpdateMonitoringConfig(newTestPlatformClient(t, srv.URL), "com.example.ext", "obj-123", cfg)
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if want := "403"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

// compareDottedVersions

func TestCompareDottedVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.11", "1.0.9", 1},
		{"1.0.9", "1.0.11", -1},
		{"1.2.0", "1.0.11", 1},
		{"1.0", "1.0.0", 0},
		{"2", "1.99.99", 1},
		{"1.0.abc", "1.0.0", 0}, // non-numeric segment treated as 0
	}
	for _, tc := range cases {
		if got := compareDottedVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("compareDottedVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
