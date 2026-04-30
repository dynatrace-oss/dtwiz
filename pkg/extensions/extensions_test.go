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
	c, err := client.New(serverURL, "dt0c01.test", serverURL, "dt0s16.test", 0)
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
