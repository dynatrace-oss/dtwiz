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

// --- DownloadInstaller ---

func TestDownloadInstaller_LinuxX86_StreamsAndStores(t *testing.T) {
	content := []byte("fake-installer-binary-content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/deployment/installer/agent/unix/default/latest" {
			t.Errorf("path = %q, want unix segment", r.URL.Path)
		}
		if got := r.URL.Query().Get("arch"); got != "x86" {
			t.Errorf("arch = %q, want x86", got)
		}
		if auth := r.Header.Get("Authorization"); auth == "" {
			t.Error("expected Authorization header to be present")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	path, err := DownloadInstaller(c, Environment{OS: "linux", Arch: "x86"})
	if err != nil {
		t.Fatalf("DownloadInstaller: %v", err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read downloaded: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
	if !strings.HasSuffix(path, ".sh") {
		t.Errorf("filename %q should end in .sh on Unix env", path)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Errorf("perms = %o, want 0700", info.Mode().Perm())
		}
	}
}

func TestDownloadInstaller_LinuxArm_URL(t *testing.T) {
	var gotArch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotArch = r.URL.Query().Get("arch")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	path, err := DownloadInstaller(newTestClassicClient(t, srv.URL), Environment{OS: "linux", Arch: "arm"})
	if err != nil {
		t.Fatalf("DownloadInstaller: %v", err)
	}
	defer os.Remove(path)
	if gotArch != "arm" {
		t.Errorf("arch query = %q, want arm", gotArch)
	}
}

func TestDownloadInstaller_Windows_URL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	path, err := DownloadInstaller(newTestClassicClient(t, srv.URL), Environment{OS: "windows", Arch: "x86"})
	if err != nil {
		t.Fatalf("DownloadInstaller: %v", err)
	}
	defer os.Remove(path)
	if gotPath != "/api/v1/deployment/installer/agent/windows/default/latest" {
		t.Errorf("path = %q, want windows segment", gotPath)
	}
	if !strings.HasSuffix(path, ".exe") {
		t.Errorf("filename %q should end in .exe for Windows env", path)
	}
}

func TestDownloadInstaller_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := DownloadInstaller(newTestClassicClient(t, srv.URL), Environment{OS: "linux", Arch: "x86"})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q does not mention 401", err)
	}
}

func TestDownloadInstaller_UnsupportedOS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := DownloadInstaller(newTestClassicClient(t, srv.URL), Environment{OS: "freebsd", Arch: "x86"})
	if err == nil {
		t.Fatal("expected error for unsupported OS")
	}
	if !strings.Contains(err.Error(), "unsupported installer OS") {
		t.Errorf("error = %q, want unsupported OS message", err)
	}
}

func TestDownloadInstaller_DebugLogLineRedactsToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	path, err := DownloadInstaller(newTestClassicClient(t, srv.URL), Environment{OS: "linux", Arch: "x86"})
	if err != nil {
		t.Fatalf("DownloadInstaller: %v", err)
	}
	defer os.Remove(path)

	logs := buf.String()
	if !strings.Contains(logs, "downloading installer") {
		t.Errorf("logs missing 'downloading installer' line:\n%s", logs)
	}
	if strings.Contains(logs, "dt0s16.test") {
		t.Errorf("logs contain raw token:\n%s", logs)
	}
}

// --- VerifyInstallerSignature ---

func TestVerifyInstallerSignature_SkipFlag(t *testing.T) {
	if err := VerifyInstallerSignature(Environment{OS: "linux"}, "/nonexistent", true); err != nil {
		t.Errorf("skip=true should return nil, got %v", err)
	}
}

func TestVerifyInstallerSignature_NonLinux(t *testing.T) {
	cases := []string{"darwin", "windows", ""}
	for _, os := range cases {
		if err := VerifyInstallerSignature(Environment{OS: os}, "/nonexistent", false); err != nil {
			t.Errorf("os=%q should return nil, got %v", os, err)
		}
	}
}

func TestVerifyInstallerSignature_MissingOpenSSL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH-based openssl shadowing test relies on Unix semantics")
	}
	t.Setenv("PATH", t.TempDir())

	err := VerifyInstallerSignature(Environment{OS: "linux", Arch: "x86"}, "/nonexistent", false)
	if err == nil {
		t.Fatal("expected missing-openssl error")
	}
	if !strings.Contains(err.Error(), "openssl is required") {
		t.Errorf("error = %q, want missing-openssl message", err)
	}
	if !strings.Contains(err.Error(), "--no-verify-signature") {
		t.Errorf("error = %q, want --no-verify-signature hint", err)
	}
}

func TestVerifyInstallerSignature_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh fake openssl")
	}
	binDir := t.TempDir()
	createStubFile(t, filepath.Join(binDir, "openssl"), "#!/bin/sh\ncat >/dev/null\nexit 0\n", 0o755)
	t.Setenv("PATH", binDir)

	installer := filepath.Join(t.TempDir(), "fake-installer.sh")
	if err := os.WriteFile(installer, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	caSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PEM"))
	}))
	defer caSrv.Close()
	withRootCertURL(t, caSrv.URL)

	if err := VerifyInstallerSignature(Environment{OS: "linux", Arch: "x86"}, installer, false); err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestVerifyInstallerSignature_FailureWrapsStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh fake openssl")
	}
	binDir := t.TempDir()
	createStubFile(t, filepath.Join(binDir, "openssl"),
		"#!/bin/sh\ncat >/dev/null\necho 'Verification failure' >&2\nexit 4\n", 0o755)
	t.Setenv("PATH", binDir)

	installer := filepath.Join(t.TempDir(), "fake-installer.sh")
	if err := os.WriteFile(installer, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	caSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PEM"))
	}))
	defer caSrv.Close()
	withRootCertURL(t, caSrv.URL)

	err := VerifyInstallerSignature(Environment{OS: "linux", Arch: "x86"}, installer, false)
	if err == nil {
		t.Fatal("expected verification failure")
	}
	if !strings.Contains(err.Error(), "Verification failure") {
		t.Errorf("error = %q, missing openssl stderr", err)
	}
	if !strings.Contains(err.Error(), "exit 4") {
		t.Errorf("error = %q, missing exit code", err)
	}
}

func TestVerifyInstallerSignature_CADownloadFailsAborts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh fake openssl")
	}
	binDir := t.TempDir()
	// openssl should never be invoked because CA fetch fails first.
	createStubFile(t, filepath.Join(binDir, "openssl"),
		"#!/bin/sh\necho 'should not run' >&2\nexit 99\n", 0o755)
	t.Setenv("PATH", binDir)

	caSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer caSrv.Close()
	withRootCertURL(t, caSrv.URL)

	installer := filepath.Join(t.TempDir(), "fake-installer.sh")
	if err := os.WriteFile(installer, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := VerifyInstallerSignature(Environment{OS: "linux", Arch: "x86"}, installer, false)
	if err == nil {
		t.Fatal("expected CA download failure")
	}
	if !strings.Contains(err.Error(), "Dynatrace root CA") {
		t.Errorf("error = %q, want CA download error", err)
	}
}

// withRootCertURL overrides the package-level CA URL for the duration of the
// test, restoring it on cleanup.
func withRootCertURL(t *testing.T, url string) {
	t.Helper()
	orig := dtRootCertURL
	dtRootCertURL = url
	t.Cleanup(func() { dtRootCertURL = orig })
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1KB"},
		{2 * 1024 * 1024, "2MB"},
		{3 * 1024 * 1024 * 1024, "3GB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
