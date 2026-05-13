package installer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

func newMockTenantServer(t *testing.T, path string, status int, body string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	return httptest.NewServer(mux)
}

func newMockClient(t *testing.T, serverURL string) *client.Client {
	t.Helper()
	c, err := client.New(serverURL, "dt0c01.testtoken", serverURL, "dt0s16.testtoken", 0)
	if err != nil {
		t.Fatalf("newMockClient: %v", err)
	}
	return c
}

func TestDefaultAgentConfig(t *testing.T) {
	cfg := DefaultAgentConfig()
	if cfg.MonitoringMode != "fullstack" {
		t.Errorf("expected MonitoringMode fullstack, got %q", cfg.MonitoringMode)
	}
}

func TestResolveAgentConfig_Default(t *testing.T) {
	cfg := ResolveAgentConfig(InstallOptions{MonitoringMode: "fullstack"})
	if cfg.MonitoringMode != "fullstack" {
		t.Errorf("expected MonitoringMode fullstack, got %q", cfg.MonitoringMode)
	}
}

func TestResolveAgentConfig_Override(t *testing.T) {
	cfg := ResolveAgentConfig(InstallOptions{MonitoringMode: "infra-only"})
	if cfg.MonitoringMode != "infra-only" {
		t.Errorf("expected MonitoringMode infra-only, got %q", cfg.MonitoringMode)
	}
}

func TestInstallOneAgentV2_ExitsCleanly(t *testing.T) {
	srv := newMockTenantServer(t, "/", http.StatusOK, `{}`)
	defer srv.Close()

	c := newMockClient(t, srv.URL)
	if err := InstallOneAgentV2(c, InstallOptions{MonitoringMode: "fullstack"}); err != nil {
		t.Errorf("expected clean exit, got error: %v", err)
	}
}
