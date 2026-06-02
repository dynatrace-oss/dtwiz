package oneagent

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
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
		if got := display.HumanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
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
