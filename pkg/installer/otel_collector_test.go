package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOtelCollectorBinaryName(t *testing.T) {
	name := otelCollectorBinaryName()
	if name == "" {
		t.Fatal("binary name should not be empty")
	}
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(name, ".exe") {
			t.Errorf("Windows binary should end with .exe, got %q", name)
		}
	} else {
		if strings.HasSuffix(name, ".exe") {
			t.Errorf("non-Windows binary should not end with .exe, got %q", name)
		}
	}
}

func TestOtelPlatformAssetName_CurrentPlatform(t *testing.T) {
	name, err := otelPlatformAssetName("v0.44.0")
	if err != nil {
		t.Fatalf("unexpected error for current platform: %v", err)
	}
	if !strings.HasPrefix(name, "dynatrace-otel-collector_0.44.0_") {
		t.Errorf("unexpected asset name prefix: %s", name)
	}
}

func TestOtelPlatformAssetName_StripLeadingV(t *testing.T) {
	a, err := otelPlatformAssetName("v0.44.0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := otelPlatformAssetName("0.44.0")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("leading 'v' should be stripped: %q vs %q", a, b)
	}
}

func TestOtelPlatformAssetName_WindowsUsesZip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}
	name, err := otelPlatformAssetName("v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(name, ".zip") {
		t.Errorf("Windows asset should be a .zip, got %q", name)
	}
}

func TestOtelPlatformAssetName_NonWindowsUsesTarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-Windows test")
	}
	name, err := otelPlatformAssetName("v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(name, ".tar.gz") {
		t.Errorf("non-Windows asset should be a .tar.gz, got %q", name)
	}
}

func TestOtelReleaseURL(t *testing.T) {
	got := otelReleaseURL("v0.44.0", "dynatrace-otel-collector_0.44.0_Darwin_arm64.tar.gz")
	want := "https://github.com/Dynatrace/dynatrace-otel-collector/releases/download/v0.44.0/dynatrace-otel-collector_0.44.0_Darwin_arm64.tar.gz"
	if got != want {
		t.Errorf("otelReleaseURL() = %q, want %q", got, want)
	}
}

func TestToAppsURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			"https://fxz0998d.dev.dynatracelabs.com",
			"https://fxz0998d.dev.apps.dynatracelabs.com",
		},
		{
			"https://abc123.live.dynatrace.com",
			"https://abc123.live.apps.dynatrace.com",
		},
		{
			// Already contains .apps. — return unchanged.
			"https://fxz0998d.dev.apps.dynatracelabs.com",
			"https://fxz0998d.dev.apps.dynatracelabs.com",
		},
		{
			// Unknown domain — return unchanged.
			"https://custom.example.com",
			"https://custom.example.com",
		},
	}
	for _, tc := range tests {
		got := toAppsURL(tc.input)
		if got != tc.want {
			t.Errorf("toAppsURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestBuildOtelLogsUIURL(t *testing.T) {
	got := buildOtelLogsUIURL("https://abc123.live.dynatrace.com", "dtwiz-host-1234")
	if !strings.HasPrefix(got, "https://abc123.live.apps.dynatrace.com") {
		t.Errorf("expected apps URL prefix, got: %s", got)
	}
	if !strings.Contains(got, "/ui/apps/dynatrace.logs/intent/view_query#") {
		t.Errorf("URL missing expected path: %s", got)
	}
	if !strings.Contains(got, "dtwiz-host-1234") {
		t.Errorf("URL missing search term: %s", got)
	}
}

func TestFormatPIDs(t *testing.T) {
	tests := []struct {
		procs []runningCollector
		want  string
	}{
		{nil, ""},
		{[]runningCollector{{pid: 42}}, "42"},
		{[]runningCollector{{pid: 1}, {pid: 2}, {pid: 3}}, "1, 2, 3"},
	}
	for _, tc := range tests {
		got := formatPIDs(tc.procs)
		if got != tc.want {
			t.Errorf("formatPIDs(%v) = %q, want %q", tc.procs, got, tc.want)
		}
	}
}

func TestGenerateOtelConfig(t *testing.T) {
	config, err := generateOtelConfig("https://env.live.dynatrace.com", "mytoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(config) == 0 {
		t.Error("config should not be empty")
	}
	if !strings.Contains(config, "https://env.live.dynatrace.com") {
		t.Errorf("config missing endpoint: %s", config)
	}
}

func TestGenerateOtelConfig_TrailingSlashStripped(t *testing.T) {
	a, err := generateOtelConfig("https://env.live.dynatrace.com/", "tok")
	if err != nil {
		t.Fatal(err)
	}
	b, err := generateOtelConfig("https://env.live.dynatrace.com", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("trailing slash on apiURL should produce identical config\ngot:  %s\nwant: %s", a, b)
	}
}

func TestExtractFromTarGz(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	binaryName := "dynatrace-otel-collector"
	content := []byte("binary content here")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: binaryName, Size: int64(len(content)), Mode: 0o755})
	_, _ = tw.Write(content)
	tw.Close()
	gz.Close()
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(dir, "extracted")
	if err := extractFromTarGz(archivePath, binaryName, destPath); err != nil {
		t.Fatalf("extractFromTarGz error: %v", err)
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestExtractFromTarGz_FileInSubdir(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	binaryName := "dynatrace-otel-collector"
	content := []byte("nested binary")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	entryName := "subdir/" + binaryName
	_ = tw.WriteHeader(&tar.Header{Name: entryName, Size: int64(len(content)), Mode: 0o755})
	_, _ = tw.Write(content)
	tw.Close()
	gz.Close()
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(dir, "extracted")
	if err := extractFromTarGz(archivePath, binaryName, destPath); err != nil {
		t.Fatalf("extractFromTarGz error: %v", err)
	}
	got, _ := os.ReadFile(destPath)
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestExtractFromTarGz_NotFound(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "other-file", Size: 5, Mode: 0o644})
	_, _ = tw.Write([]byte("hello"))
	tw.Close()
	gz.Close()
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	err := extractFromTarGz(archivePath, "missing-binary", filepath.Join(dir, "out"))
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "not found in archive") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExtractFromZip(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.zip")
	binaryName := "dynatrace-otel-collector"
	content := []byte("zip binary content")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(binaryName)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write(content)
	zw.Close()
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(dir, "extracted")
	if err := extractFromZip(archivePath, binaryName, destPath); err != nil {
		t.Fatalf("extractFromZip error: %v", err)
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestExtractFromZip_FileInSubdir(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.zip")
	binaryName := "dynatrace-otel-collector"
	content := []byte("nested zip binary")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("subdir/" + binaryName)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write(content)
	zw.Close()
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(dir, "extracted")
	if err := extractFromZip(archivePath, binaryName, destPath); err != nil {
		t.Fatalf("extractFromZip error: %v", err)
	}
	got, _ := os.ReadFile(destPath)
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestExtractFromZip_NotFound(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.zip")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("other-file")
	_, _ = f.Write([]byte("hello"))
	zw.Close()
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	err := extractFromZip(archivePath, "missing-binary", filepath.Join(dir, "out"))
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "not found in zip archive") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWaitForLogInDynatrace_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"records":[{"content":"dtwiz-host-1234"}]}}`)
	}))
	defer ts.Close()

	err := waitForLogInDynatrace(ts.URL, "testtoken", "dtwiz-host-1234", 5*time.Second)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestWaitForLogInDynatrace_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorized"}`)
	}))
	defer ts.Close()

	err := waitForLogInDynatrace(ts.URL, "badtoken", "anything", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "storage:logs:read") {
		t.Errorf("expected scope hint in error, got: %v", err)
	}
}

func TestWaitForLogInDynatrace_Forbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"forbidden"}`)
	}))
	defer ts.Close()

	err := waitForLogInDynatrace(ts.URL, "badtoken", "anything", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got: %v", err)
	}
}

func TestWaitForLogInDynatrace_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"records":[]}}`)
	}))
	defer ts.Close()

	// Negative timeout guarantees the deadline is already expired before the
	// first request, so we hit the timeout check without sleeping 5 seconds.
	err := waitForLogInDynatrace(ts.URL, "tok", "missing-term", -time.Second)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected 'timed out' in error, got: %v", err)
	}
}

func TestWaitForLogInDynatrace_TokenTruncatedInError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	longToken := strings.Repeat("x", 50)
	err := waitForLogInDynatrace(ts.URL, longToken, "term", 5*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	// Token should be truncated to first 20 chars + "...".
	if strings.Contains(err.Error(), longToken) {
		t.Errorf("full token should not appear in error: %v", err)
	}
	if !strings.Contains(err.Error(), longToken[:20]+"...") {
		t.Errorf("expected truncated token hint in error: %v", err)
	}
}

func TestWaitForOtelCollectorReady_CrashedBeforeReady(t *testing.T) {
	// This test requires port 4318 to be idle; skip if something is already listening.
	if c, err := net.Dial("tcp", "127.0.0.1:4318"); err == nil {
		c.Close()
		t.Skip("port 4318 is in use — crash detection requires no listener on :4318")
	}

	crashed := make(chan error, 1)
	crashed <- fmt.Errorf("process died")

	err := waitForOtelCollectorReady(5*time.Second, crashed)
	if err == nil {
		t.Fatal("expected error when collector crashes")
	}
	if !strings.Contains(err.Error(), "exited unexpectedly") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWaitForOtelCollectorReady_ConnectsSuccessfully(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:4318")
	if err != nil {
		t.Skip("port 4318 not available:", err)
	}
	defer ln.Close()

	crashed := make(chan error, 1)
	if err := waitForOtelCollectorReady(5*time.Second, crashed); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestTermSupportsOSC8(t *testing.T) {
	tests := []struct {
		env  map[string]string
		want bool
	}{
		{map[string]string{"TERM_PROGRAM": "vscode"}, true},
		{map[string]string{"TERM_PROGRAM": "iTerm.app"}, true},
		{map[string]string{"TERM_PROGRAM": "WezTerm"}, true},
		{map[string]string{"TERM_PROGRAM": "Hyper"}, true},
		{map[string]string{"WT_SESSION": "some-session-id"}, true},
		{map[string]string{"VTE_VERSION": "6500"}, true},
		{map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, false},
		{map[string]string{}, false},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%v", tc.env), func(t *testing.T) {
			t.Setenv("TERM_PROGRAM", "")
			t.Setenv("WT_SESSION", "")
			t.Setenv("VTE_VERSION", "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got := termSupportsOSC8()
			if got != tc.want {
				t.Errorf("termSupportsOSC8() = %v, want %v (env: %v)", got, tc.want, tc.env)
			}
		})
	}
}

func TestTermLink_PlainText(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("WT_SESSION", "")
	t.Setenv("VTE_VERSION", "")

	got := termLink("Click here", "https://example.com")
	if strings.Contains(got, "\x1b]8;;") {
		t.Errorf("expected plain text link, got OSC8 sequence: %q", got)
	}
	if !strings.Contains(got, "Click here") {
		t.Errorf("label missing from output: %q", got)
	}
	if !strings.Contains(got, "https://example.com") {
		t.Errorf("URL missing from output: %q", got)
	}
}

func TestTermLink_OSC8(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "iTerm.app")

	got := termLink("Click here", "https://example.com")
	if !strings.Contains(got, "\x1b]8;;https://example.com") {
		t.Errorf("expected OSC8 sequence, got: %q", got)
	}
	if !strings.Contains(got, "Click here") {
		t.Errorf("label missing from output: %q", got)
	}
}
