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

// withNeedsSudo overrides needsSudoFn for the duration of the test.
func withNeedsSudo(t *testing.T, val bool) {
	t.Helper()
	orig := needsSudoFn
	needsSudoFn = func() bool { return val }
	t.Cleanup(func() { needsSudoFn = orig })
}

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

func TestInstallOneAgentV2_DryRun_NoDownload(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("OneAgent not supported on macOS")
	}

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

// --- Task 6: BuildInstallCommand ---

func testLinuxCfg(serverURL string) AgentConfig {
	return AgentConfig{MonitoringMode: "fullstack", AppLogContentAccess: true, ServerURL: serverURL}
}

func TestBuildInstallCommand_Linux_NonRoot(t *testing.T) {
	withNeedsSudo(t, true)
	argv, err := BuildInstallCommand(Environment{OS: "linux", Arch: "x86"}, testLinuxCfg("https://env.live.dynatrace.com"), InstallOptions{}, "/tmp/agent.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if argv[0] != "sudo" {
		t.Errorf("argv[0] = %q, want sudo", argv[0])
	}
	if argv[1] != "/bin/sh" {
		t.Errorf("argv[1] = %q, want /bin/sh", argv[1])
	}
	if argv[2] != "/tmp/agent.sh" {
		t.Errorf("argv[2] = %q, want installer path", argv[2])
	}
	assertContains(t, argv, "--set-server=https://env.live.dynatrace.com")
	assertContains(t, argv, "--set-monitoring-mode=fullstack")
	assertContains(t, argv, "--set-app-log-content-access=true")
}

func TestBuildInstallCommand_Linux_Root(t *testing.T) {
	withNeedsSudo(t, false)
	argv, err := BuildInstallCommand(Environment{OS: "linux", Arch: "x86"}, testLinuxCfg("https://env.live.dynatrace.com"), InstallOptions{}, "/tmp/agent.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if argv[0] == "sudo" {
		t.Error("root user should not have sudo prepended")
	}
	if argv[0] != "/bin/sh" {
		t.Errorf("argv[0] = %q, want /bin/sh", argv[0])
	}
}

func TestBuildInstallCommand_Linux_HostGroup(t *testing.T) {
	withNeedsSudo(t, false)
	argv, err := BuildInstallCommand(Environment{OS: "linux", Arch: "x86"}, testLinuxCfg("https://env.live.dynatrace.com"), InstallOptions{HostGroup: "prod-web"}, "/tmp/agent.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, argv, "--set-host-group=prod-web")
}

func TestBuildInstallCommand_Linux_NoHostGroup(t *testing.T) {
	withNeedsSudo(t, false)
	argv, err := BuildInstallCommand(Environment{OS: "linux", Arch: "x86"}, testLinuxCfg("https://env.live.dynatrace.com"), InstallOptions{}, "/tmp/agent.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, a := range argv {
		if strings.HasPrefix(a, "--set-host-group") {
			t.Errorf("unexpected --set-host-group flag: %s", a)
		}
	}
}

func TestBuildInstallCommand_Windows_Basic(t *testing.T) {
	argv, err := BuildInstallCommand(Environment{OS: "windows", Arch: "x86"}, testLinuxCfg("https://env.live.dynatrace.com"), InstallOptions{}, `C:\agent.exe`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if argv[0] != `C:\agent.exe` {
		t.Errorf("argv[0] = %q, want installer path", argv[0])
	}
	assertContains(t, argv, "--set-monitoring-mode=fullstack")
	assertContains(t, argv, "--set-app-log-content-access=true")
	for _, a := range argv {
		if a == "--quiet" {
			t.Error("--quiet should not be present when opts.Quiet is false")
		}
	}
}

func TestBuildInstallCommand_Windows_Quiet_OrderFirst(t *testing.T) {
	argv, err := BuildInstallCommand(Environment{OS: "windows", Arch: "x86"}, testLinuxCfg("https://env.live.dynatrace.com"), InstallOptions{Quiet: true}, `C:\agent.exe`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if argv[1] != "--quiet" {
		t.Errorf("--quiet should be the first flag (argv[1]), got %q", argv[1])
	}
	// All --set-* flags must appear after --quiet
	quietIdx := 1
	for i, a := range argv {
		if strings.HasPrefix(a, "--set-") && i < quietIdx {
			t.Errorf("--set-* flag %q appears before --quiet", a)
		}
	}
}

func TestBuildInstallCommand_Windows_HostGroup(t *testing.T) {
	argv, err := BuildInstallCommand(Environment{OS: "windows", Arch: "x86"}, testLinuxCfg("https://env.live.dynatrace.com"), InstallOptions{HostGroup: "staging"}, `C:\agent.exe`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, argv, "--set-host-group=staging")
}

func TestBuildInstallCommand_MonitoringModeOverride(t *testing.T) {
	withNeedsSudo(t, false)
	cfg := AgentConfig{MonitoringMode: "infra-only", AppLogContentAccess: false, ServerURL: "https://env.live.dynatrace.com"}
	argv, err := BuildInstallCommand(Environment{OS: "linux", Arch: "x86"}, cfg, InstallOptions{}, "/tmp/agent.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, argv, "--set-monitoring-mode=infra-only")
	assertContains(t, argv, "--set-app-log-content-access=false")
}

func TestBuildInstallCommand_UnsupportedOS(t *testing.T) {
	_, err := BuildInstallCommand(Environment{OS: "darwin", Arch: "x86"}, testLinuxCfg("https://env.live.dynatrace.com"), InstallOptions{}, "/tmp/agent.sh")
	if err == nil {
		t.Fatal("expected error for unsupported OS")
	}
	if !strings.Contains(err.Error(), "unsupported OS") {
		t.Errorf("error = %q, want unsupported OS message", err)
	}
}

func TestBuildInstallCommand_DebugLog(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	withNeedsSudo(t, false)
	_, _ = BuildInstallCommand(Environment{OS: "linux", Arch: "x86"}, testLinuxCfg("https://env.live.dynatrace.com"), InstallOptions{}, "/tmp/agent.sh")

	if !strings.Contains(buf.String(), "built install command") {
		t.Errorf("expected debug log 'built install command', got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "argv") {
		t.Errorf("expected 'argv' key in debug log, got: %s", buf.String())
	}
}

// --- Task 6: ExecuteInstallCommand ---

func TestExecuteInstallCommand_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses Unix shell")
	}
	code, err := ExecuteInstallCommand([]string{"sh", "-c", "exit 0"}, true)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestExecuteInstallCommand_NonZeroExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses Unix shell")
	}
	code, err := ExecuteInstallCommand([]string{"sh", "-c", "exit 7"}, true)
	if code != 7 {
		t.Errorf("expected exit code 7, got %d", code)
	}
	if err == nil {
		t.Error("expected non-nil error for non-zero exit")
	}
}

func TestExecuteInstallCommand_StderrCapturedOnFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses Unix shell")
	}
	code, err := ExecuteInstallCommand([]string{"sh", "-c", "echo 'failed to write /opt/dynatrace' >&2; exit 7"}, true)
	if code != 7 {
		t.Errorf("expected exit code 7, got %d", code)
	}
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "failed to write /opt/dynatrace") {
		t.Errorf("error = %q, want captured stderr in message", err)
	}
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("error = %q, want exit code in message", err)
	}
}

func TestExecuteInstallCommand_DebugLog(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses Unix shell")
	}
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	_, _ = ExecuteInstallCommand([]string{"sh", "-c", "exit 0"}, true)

	if !strings.Contains(buf.String(), "executing installer") {
		t.Errorf("expected debug log 'executing installer', got: %s", buf.String())
	}
}

// assertContains fails the test if elem is not in slice.
func assertContains(t *testing.T, slice []string, elem string) {
	t.Helper()
	for _, s := range slice {
		if s == elem {
			return
		}
	}
	t.Errorf("expected %q in %v", elem, slice)
}
