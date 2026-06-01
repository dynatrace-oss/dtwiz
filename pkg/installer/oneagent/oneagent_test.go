package oneagent

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

// newTestClassicClient creates a ClassicClient pointing at the given test server URL.
func newTestClassicClient(t *testing.T, serverURL string) *client.ClassicClient {
	t.Helper()
	c, err := client.New(serverURL, serverURL, "dt0s16.test", "dt0s16.test", 0)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	return c.Classic
}

// createStubFile writes content to path (creating parent directories as needed).
func createStubFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

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
	c, err := client.New(serverURL, serverURL, "dt0s16.testtoken", "dt0s16.testtoken", 0)
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

func TestResolveAgentConfig_DebugLog(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	ResolveAgentConfig(InstallOptions{})
	ResolveAgentConfig(InstallOptions{MonitoringMode: "infra-only"})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 debug lines, got %d:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "resolved agent config") || !strings.Contains(lines[0], "monitoring-mode=fullstack") || !strings.Contains(lines[0], "override_set=false") {
		t.Errorf("default path: unexpected log line: %s", lines[0])
	}
	if !strings.Contains(lines[1], "resolved agent config") || !strings.Contains(lines[1], "monitoring-mode=infra-only") || !strings.Contains(lines[1], "override_set=true") {
		t.Errorf("override path: unexpected log line: %s", lines[1])
	}
}

func TestInstallOneAgentV2_UnsupportedPlatformReturnsError(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific error path test")
	}
	srv := newMockTenantServer(t, "/", http.StatusOK, `{}`)
	defer srv.Close()

	c := newMockClient(t, srv.URL)
	err := InstallOneAgentV2(c, InstallOptions{MonitoringMode: "fullstack"})
	if err == nil {
		t.Fatal("expected error on macOS, got nil")
	}
	if !strings.Contains(err.Error(), "macOS") {
		t.Errorf("error = %q, want macOS message", err)
	}
}

func TestDetectRuntimeEnvironment(t *testing.T) {
	env, err := detectRuntimeEnvironment()
	switch runtime.GOOS {
	case "linux", "windows":
		if err != nil {
			if !strings.Contains(err.Error(), "unsupported architecture") {
				t.Errorf("unexpected error on %s: %v", runtime.GOOS, err)
			}
			return
		}
		if env.OS != runtime.GOOS {
			t.Errorf("env.OS = %q, want %q", env.OS, runtime.GOOS)
		}
		if env.Arch == "" {
			t.Error("expected non-empty Arch")
		}
	case "darwin":
		if err == nil {
			t.Fatal("expected error on macOS")
		}
		if !strings.Contains(err.Error(), "macOS") {
			t.Errorf("error = %q, want macOS message", err)
		}
	default:
		if err == nil {
			t.Fatalf("expected error on unsupported OS %s", runtime.GOOS)
		}
		if !strings.Contains(err.Error(), "unsupported") {
			t.Errorf("error = %q, want unsupported OS message", err)
		}
	}
}
