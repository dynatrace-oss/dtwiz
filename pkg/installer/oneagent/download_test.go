package oneagent

import (
	"bytes"
	"compress/gzip"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestInstallerOSSegment(t *testing.T) {
	cases := []struct {
		os      string
		want    string
		wantErr bool
	}{
		{"linux", "unix", false},
		{"windows", "windows", false},
		{"freebsd", "", true},
		{"darwin", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := installerOSSegment(c.os)
		if (err != nil) != c.wantErr {
			t.Errorf("installerOSSegment(%q) error = %v, wantErr %v", c.os, err, c.wantErr)
		}
		if got != c.want {
			t.Errorf("installerOSSegment(%q) = %q, want %q", c.os, got, c.want)
		}
	}
}

func TestInstallerExtension(t *testing.T) {
	cases := []struct {
		os   string
		want string
	}{
		{"windows", ".exe"},
		{"linux", ".sh"},
		{"darwin", ".sh"},
		{"", ".sh"},
	}
	for _, c := range cases {
		got := installerExtension(c.os)
		if got != c.want {
			t.Errorf("installerExtension(%q) = %q, want %q", c.os, got, c.want)
		}
	}
}

func TestDownloadInstaller_NetworkError(t *testing.T) {
	// Close the server before the request so the client gets a connection-refused error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	c := newTestClassicClient(t, srv.URL)
	srv.Close()

	_, err := DownloadInstaller(c, Environment{OS: "linux", Arch: "x86"})
	if err == nil {
		t.Fatal("expected error on network failure")
	}
	if !strings.Contains(err.Error(), "downloading OneAgent installer") {
		t.Errorf("error = %q, want 'downloading OneAgent installer'", err)
	}
}

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

func TestInstallerDownloadURL(t *testing.T) {
	cases := []struct {
		baseURL string
		env     Environment
		want    string
		wantErr bool
	}{
		{
			"https://abc123.live.dynatrace.com",
			Environment{OS: "linux", Arch: "x86"},
			"https://abc123.live.dynatrace.com/api/v1/deployment/installer/agent/unix/default/latest?arch=x86",
			false,
		},
		{
			// trailing slash on base URL should be stripped
			"https://abc123.live.dynatrace.com/",
			Environment{OS: "linux", Arch: "arm"},
			"https://abc123.live.dynatrace.com/api/v1/deployment/installer/agent/unix/default/latest?arch=arm",
			false,
		},
		{
			"https://abc123.live.dynatrace.com",
			Environment{OS: "windows", Arch: "x86"},
			"https://abc123.live.dynatrace.com/api/v1/deployment/installer/agent/windows/default/latest?arch=x86",
			false,
		},
		{
			"https://abc123.live.dynatrace.com",
			Environment{OS: "freebsd", Arch: "x86"},
			"",
			true,
		},
	}
	for _, c := range cases {
		got, err := InstallerDownloadURL(c.baseURL, c.env)
		if (err != nil) != c.wantErr {
			t.Errorf("InstallerDownloadURL(%q, %+v) error = %v, wantErr %v", c.baseURL, c.env, err, c.wantErr)
		}
		if got != c.want {
			t.Errorf("InstallerDownloadURL(%q, %+v) = %q, want %q", c.baseURL, c.env, got, c.want)
		}
	}
}

func TestDownloadInstaller_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := DownloadInstaller(newTestClassicClient(t, srv.URL), Environment{OS: "linux", Arch: "x86"})
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error %q does not mention 403", err)
	}
}

func TestDownloadInstaller_NonOK_GzipBody(t *testing.T) {
	// Build a gzip-encoded error body so readErrorBody's decompression path runs.
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, _ = gz.Write([]byte("token scope missing"))
	_ = gz.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(compressed.Bytes())
	}))
	defer srv.Close()

	_, err := DownloadInstaller(newTestClassicClient(t, srv.URL), Environment{OS: "linux", Arch: "x86"})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "token scope missing") {
		t.Errorf("error %q does not contain decoded gzip body", err)
	}
}
