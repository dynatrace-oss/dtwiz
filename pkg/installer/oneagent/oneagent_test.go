package oneagent

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
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
	if !cfg.AppLogContentAccess {
		t.Error("expected AppLogContentAccess to be true by default")
	}
}

func TestResolveAgentConfig_Default(t *testing.T) {
	// Empty MonitoringMode → default-fallback branch (the if-override is not entered).
	cfg := ResolveAgentConfig(InstallOptions{})
	if cfg.MonitoringMode != "fullstack" {
		t.Errorf("expected MonitoringMode fullstack, got %q", cfg.MonitoringMode)
	}
	if !cfg.AppLogContentAccess {
		t.Error("expected AppLogContentAccess to be true")
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
	if !strings.Contains(lines[0], "resolved agent config") || !strings.Contains(lines[0], "monitoring_mode=fullstack") || !strings.Contains(lines[0], "override_set=false") {
		t.Errorf("default path: unexpected log line: %s", lines[0])
	}
	if !strings.Contains(lines[1], "resolved agent config") || !strings.Contains(lines[1], "monitoring_mode=infra-only") || !strings.Contains(lines[1], "override_set=true") {
		t.Errorf("override path: unexpected log line: %s", lines[1])
	}
}

func TestInstallOneAgentV2_DryRun_NoDownload(t *testing.T) {
	skipNonLinux(t)
	withInstallDir(t, filepath.Join(t.TempDir(), "nonexistent"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("dry-run made a real HTTP request: %s %s", r.Method, r.URL.Path)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newMockClient(t, srv.URL)
	if err := InstallOneAgentV2(c, InstallOptions{DryRun: true, MonitoringMode: "fullstack"}); err != nil {
		t.Fatalf("dry-run returned unexpected error: %v", err)
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

func TestInstallOneAgentV2_ConnectivityFail_AbortsBeforeDownload(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("OneAgent not supported on macOS")
	}

	var downloadCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == endpointsAPIPath {
			w.WriteHeader(http.StatusOK)
			// RFC 5737 TEST-NET: never routable, probe will time out.
			_, _ = w.Write([]byte("192.0.2.1:12345"))
			return
		}
		downloadCalled = true
		http.NotFound(w, r)
	}))
	defer srv.Close()

	old := defaultProbeTimeout
	defaultProbeTimeout = 50 * time.Millisecond
	defer func() { defaultProbeTimeout = old }()

	c := newMockClient(t, srv.URL)
	err := InstallOneAgentV2(c, InstallOptions{MonitoringMode: "fullstack"})
	if err == nil {
		t.Fatal("expected error when connectivity check fails, got nil")
	}
	if !strings.Contains(err.Error(), "connectivity check failed") {
		t.Errorf("error = %q, want 'connectivity check failed'", err.Error())
	}
	if downloadCalled {
		t.Error("download API was called but must not be when connectivity check fails")
	}
}

func TestInstallOneAgentV2_ConnectivityPass_ContinuesToDownload(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("OneAgent not supported on macOS")
	}

	ln, addr := startTCPListener(t)
	defer ln.Close()
	go acceptLoop(ln)

	var downloadCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == endpointsAPIPath {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(addr))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/deployment/installer/agent/") {
			downloadCalled = true
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newMockClient(t, srv.URL)
	err := InstallOneAgentV2(c, InstallOptions{MonitoringMode: "fullstack"})
	// Connectivity passed, so the error must come from the download stage, not connectivity.
	if err != nil && strings.Contains(err.Error(), "connectivity check failed") {
		t.Errorf("connectivity should have passed but got: %v", err)
	}
	if !downloadCalled {
		t.Error("download API was not called — install should have proceeded past the connectivity check")
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

// captureStdout replaces os.Stdout with a pipe for the duration of the test
// and returns a function that restores it and returns the captured output.
// Only fmt.Printf / fmt.Print calls are captured; color.Output is not.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })
	return func() string {
		w.Close()
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		r.Close()
		return buf.String()
	}
}

// withStdin replaces os.Stdin with a pipe pre-loaded with the given text.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("withStdin: %v", err)
	}
	_, _ = w.WriteString(input)
	w.Close()
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		r.Close()
	})
}

// --- printDryRun ---

func TestPrintDryRun_InstallHeader(t *testing.T) {
	withNeedsSudo(t, false)
	flush := captureStdout(t)
	printDryRun(
		Environment{OS: "linux", Arch: "x86"},
		AgentConfig{MonitoringMode: "fullstack", AppLogContentAccess: true, ServerURL: "https://abc.live.dynatrace.com"},
		InstallOptions{NoVerifySignature: true},
		false, // not updating
	)
	out := flush()
	if !strings.Contains(out, "Would install") {
		t.Errorf("expected 'Would install', got:\n%s", out)
	}
	if strings.Contains(out, "Would update") {
		t.Errorf("unexpected 'Would update':\n%s", out)
	}
}

func TestPrintDryRun_UpdateHeader(t *testing.T) {
	withNeedsSudo(t, false)
	flush := captureStdout(t)
	printDryRun(
		Environment{OS: "linux", Arch: "x86"},
		AgentConfig{MonitoringMode: "infra-only", AppLogContentAccess: true, ServerURL: "https://abc.live.dynatrace.com"},
		InstallOptions{NoVerifySignature: true},
		true, // updating
	)
	out := flush()
	if !strings.Contains(out, "Would update") {
		t.Errorf("expected 'Would update', got:\n%s", out)
	}
	if !strings.Contains(out, "infra-only") {
		t.Errorf("expected monitoring mode in output, got:\n%s", out)
	}
}

func TestPrintDryRun_SignatureLines(t *testing.T) {
	withNeedsSudo(t, false)
	cfg := AgentConfig{MonitoringMode: "fullstack", AppLogContentAccess: true, ServerURL: "https://abc.live.dynatrace.com"}

	// Linux without --no-verify-signature → should mention verification CA URL.
	flush := captureStdout(t)
	printDryRun(Environment{OS: "linux", Arch: "x86"}, cfg, InstallOptions{}, false)
	if out := flush(); !strings.Contains(out, "would verify") {
		t.Errorf("linux without flag: expected 'would verify', got:\n%s", out)
	}

	// Linux with --no-verify-signature → "skipped".
	flush = captureStdout(t)
	printDryRun(Environment{OS: "linux", Arch: "x86"}, cfg, InstallOptions{NoVerifySignature: true}, false)
	if out := flush(); !strings.Contains(out, "skipped") {
		t.Errorf("--no-verify-signature: expected 'skipped', got:\n%s", out)
	}

	// Non-Linux → "skipped".
	flush = captureStdout(t)
	printDryRun(Environment{OS: "windows", Arch: "x86"}, cfg, InstallOptions{}, false)
	if out := flush(); !strings.Contains(out, "skipped") {
		t.Errorf("windows: expected 'skipped', got:\n%s", out)
	}
}

func TestPrintDryRun_HostGroup(t *testing.T) {
	withNeedsSudo(t, false)
	flush := captureStdout(t)
	printDryRun(
		Environment{OS: "linux", Arch: "x86"},
		AgentConfig{MonitoringMode: "fullstack", AppLogContentAccess: true, ServerURL: "https://abc.live.dynatrace.com"},
		InstallOptions{HostGroup: "prod-eu", NoVerifySignature: true},
		false,
	)
	out := flush()
	if !strings.Contains(out, "prod-eu") {
		t.Errorf("expected host group in output, got:\n%s", out)
	}
}

// skipNonLinux skips the test on platforms where InstallOneAgentV2 cannot
// reach the detection logic (macOS returns an error from detectRuntimeEnvironment).
func skipNonLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("update-flow tests require Linux; skipping on %s", runtime.GOOS)
	}
}

// TestInstallOneAgentV2_DryRun_HeaderInstall verifies that the dry-run plan
// header reads "Would install" when no existing agent is detected.
func TestInstallOneAgentV2_DryRun_HeaderInstall(t *testing.T) {
	skipNonLinux(t)
	withInstallDir(t, filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("PATH", "")

	flush := captureStdout(t)
	srv := newMockTenantServer(t, "/", http.StatusOK, "")
	defer srv.Close()
	c := newMockClient(t, srv.URL)

	if err := InstallOneAgentV2(c, InstallOptions{DryRun: true, MonitoringMode: "fullstack"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := flush()
	if !strings.Contains(out, "Would install") {
		t.Errorf("expected 'Would install' in output, got:\n%s", out)
	}
	if strings.Contains(out, "Would update") {
		t.Errorf("unexpected 'Would update' in output:\n%s", out)
	}
}

// TestInstallOneAgentV2_DryRun_HeaderUpdate verifies that the dry-run plan
// header reads "Would update" when an existing installation is detected.
func TestInstallOneAgentV2_DryRun_HeaderUpdate(t *testing.T) {
	skipNonLinux(t)
	withInstallDir(t, t.TempDir())

	flush := captureStdout(t)
	srv := newMockTenantServer(t, "/", http.StatusOK, "")
	defer srv.Close()
	c := newMockClient(t, srv.URL)

	if err := InstallOneAgentV2(c, InstallOptions{DryRun: true, MonitoringMode: "fullstack"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := flush()
	if !strings.Contains(out, "Would update") {
		t.Errorf("expected 'Would update' in output, got:\n%s", out)
	}
}

// TestInstallOneAgentV2_UpdatePrompt_Declined verifies that answering "n"
// exits cleanly without making any network calls.
func TestInstallOneAgentV2_UpdatePrompt_Declined(t *testing.T) {
	skipNonLinux(t)
	withInstallDir(t, t.TempDir())
	withStdin(t, "n\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("declined update made HTTP request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	c := newMockClient(t, srv.URL)

	err := InstallOneAgentV2(c, InstallOptions{MonitoringMode: "fullstack"})
	if !errors.Is(err, installer.ErrInstallCancelled) {
		t.Fatalf("expected ErrInstallCancelled on decline, got: %v", err)
	}
}

// TestInstallOneAgentV2_UpdatePrompt_EOFCancels verifies that a closed stdin
// (CI pipeline, < /dev/null) is treated as "no" and does not proceed to download.
func TestInstallOneAgentV2_UpdatePrompt_EOFCancels(t *testing.T) {
	skipNonLinux(t)
	withInstallDir(t, t.TempDir())
	withStdin(t, "") // EOF immediately

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("EOF should cancel: unexpected HTTP request %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	c := newMockClient(t, srv.URL)

	err := InstallOneAgentV2(c, InstallOptions{MonitoringMode: "fullstack"})
	if !errors.Is(err, installer.ErrInstallCancelled) {
		t.Fatalf("expected ErrInstallCancelled on EOF cancel, got: %v", err)
	}
}

// TestInstallOneAgentV2_UpdatePrompt_AutoConfirm verifies that AutoConfirm
// bypasses the prompt and proceeds to download when an agent is detected.
func TestInstallOneAgentV2_UpdatePrompt_AutoConfirm(t *testing.T) {
	skipNonLinux(t)
	withInstallDir(t, t.TempDir())

	orig := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = orig })

	// Server must serve a valid (minimal) installer body so DownloadInstaller
	// succeeds; the install is expected to fail at execution, not at download.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("#!/bin/sh\n"))
	}))
	defer srv.Close()
	c := newMockClient(t, srv.URL)

	// The install will fail after download (no real installer), but the point
	// is that the prompt was skipped and the download was attempted.
	_ = InstallOneAgentV2(c, InstallOptions{MonitoringMode: "fullstack", NoVerifySignature: true})
}

// TestInstallOneAgentV2_UpdateQuiet verifies that quiet mode skips the prompt
// entirely and proceeds to download when an agent is detected.
func TestInstallOneAgentV2_UpdateQuiet(t *testing.T) {
	skipNonLinux(t)
	withInstallDir(t, t.TempDir())

	downloaded := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "installer/agent") {
			downloaded = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("#!/bin/sh\n"))
		}
	}))
	defer srv.Close()
	c := newMockClient(t, srv.URL)

	_ = InstallOneAgentV2(c, InstallOptions{MonitoringMode: "fullstack", Quiet: true, NoVerifySignature: true})
	if !downloaded {
		t.Error("expected download to be attempted in quiet mode with existing agent")
	}
}
