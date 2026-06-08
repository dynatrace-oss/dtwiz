package oneagent

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startTCPListener opens a localhost TCP listener on an ephemeral port and
// returns it plus its "host:port" address. The caller is responsible for
// closing the listener.
func startTCPListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("startTCPListener: %v", err)
	}
	return ln, ln.Addr().String()
}

// acceptLoop drains accept calls so the listener doesn't block probes.
func acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}
}

// newMockEndpointServer returns an httptest.Server that serves the given
// semicolon-separated endpoint string at the connectioninfo/endpoints path.
func newMockEndpointServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == endpointsAPIPath {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
			return
		}
		// Allow download API calls to go through without error in integration
		// tests that don't care about the download stage.
		http.NotFound(w, r)
	}))
}

// --- ResolveEndpoints unit tests ---

func TestResolveEndpoints_HappyPath(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK,
		"endpoint-1.example.com:443;endpoint-2.example.com:443")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	eps, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("ResolveEndpoints: %v", err)
	}
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}
	if eps[0].Host != "endpoint-1.example.com" || eps[0].Port != 443 {
		t.Errorf("eps[0] = %+v, want {endpoint-1.example.com 443}", eps[0])
	}
	if eps[1].Host != "endpoint-2.example.com" || eps[1].Port != 443 {
		t.Errorf("eps[1] = %+v, want {endpoint-2.example.com 443}", eps[1])
	}
}

func TestResolveEndpoints_NoPort_Defaults443(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK, "endpoint-1.example.com")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	eps, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("ResolveEndpoints: %v", err)
	}
	if len(eps) != 1 || eps[0].Port != 443 {
		t.Errorf("expected port 443, got %+v", eps)
	}
}

func TestResolveEndpoints_IPLiteral(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK, "54.88.45.104:443")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	eps, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("ResolveEndpoints: %v", err)
	}
	if len(eps) != 1 || eps[0].Host != "54.88.45.104" || eps[0].Port != 443 {
		t.Errorf("expected {54.88.45.104 443}, got %+v", eps)
	}
}

func TestResolveEndpoints_NewlineSeparated(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK,
		"endpoint-1.example.com:443\nendpoint-2.example.com:443\nendpoint-3.example.com:443")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	eps, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("ResolveEndpoints: %v", err)
	}
	if len(eps) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(eps))
	}
}

func TestResolveEndpoints_CRLFSeparated(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK,
		"ep1.example.com:443\r\nep2.example.com:443\r\nep3.example.com:443")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	eps, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("ResolveEndpoints: %v", err)
	}
	if len(eps) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(eps))
	}
	for _, ep := range eps {
		if strings.Contains(ep.Host, "\r") {
			t.Errorf("host %q contains stray carriage return", ep.Host)
		}
	}
}

func TestResolveEndpoints_FullHTTPSURL(t *testing.T) {
	// Some Dynatrace deployments return full URLs in the form
	// https://host:port/communication rather than bare host:port.
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK,
		"https://endpoint-1.example.com:9999/communication;https://endpoint-2.example.com:8443/communication")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	eps, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("ResolveEndpoints: %v", err)
	}
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}
	if eps[0].Host != "endpoint-1.example.com" || eps[0].Port != 9999 {
		t.Errorf("eps[0] = %+v, want {endpoint-1.example.com 9999}", eps[0])
	}
	if eps[1].Host != "endpoint-2.example.com" || eps[1].Port != 8443 {
		t.Errorf("eps[1] = %+v, want {endpoint-2.example.com 8443}", eps[1])
	}
}

func TestResolveEndpoints_MixedSeparators(t *testing.T) {
	// Real tenants may mix formats — be robust.
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK,
		"ep1.example.com:443;ep2.example.com\nep3.example.com:9999")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	eps, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("ResolveEndpoints: %v", err)
	}
	if len(eps) != 3 {
		t.Fatalf("expected 3 endpoints from mixed separators, got %d", len(eps))
	}
	if eps[1].Port != 443 {
		t.Errorf("bare host ep2 should default to port 443, got %d", eps[1].Port)
	}
}

func TestResolveEndpoints_EmptyResponse(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK, "")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	_, err := ResolveEndpoints(c)
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
	if !strings.Contains(err.Error(), "no endpoints") {
		t.Errorf("error = %q, want 'no endpoints'", err)
	}
}

func TestResolveEndpoints_401(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusUnauthorized, "Invalid token")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	_, err := ResolveEndpoints(c)
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want status 401", err)
	}
	if !strings.Contains(err.Error(), "Invalid token") {
		t.Errorf("error = %q, want body 'Invalid token'", err)
	}
}

func TestResolveEndpoints_5xx(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusServiceUnavailable, "server error")
	defer srv.Close()

	c := newTestClassicClient(t, srv.URL)
	// Disable retries to keep the test fast — the error-handling path is the
	// same regardless of how many times the server returns 5xx.
	c.HTTP().SetRetryCount(0)

	_, err := ResolveEndpoints(c)
	if err == nil {
		t.Fatal("expected error for 5xx, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want status 503", err)
	}
}

func TestResolveEndpoints_DebugLogs(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK,
		"ep1.example.com:443;ep2.example.com:443;ep3.example.com:443")
	defer srv.Close()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	c := newTestClassicClient(t, srv.URL)
	_, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("ResolveEndpoints: %v", err)
	}

	logs := buf.String()
	if !strings.Contains(logs, "resolving tenant endpoints") {
		t.Errorf("missing 'resolving tenant endpoints' debug line:\n%s", logs)
	}
	// Three per-endpoint debug lines expected (exact msg key, not substring of "tenant endpoints").
	count := strings.Count(logs, `msg="tenant endpoint"`)
	if count != 3 {
		t.Errorf("expected 3 'tenant endpoint' debug lines, got %d:\n%s", count, logs)
	}
}

func TestResolveEndpoints_VerboseLog(t *testing.T) {
	srv := newMockTenantServer(t, endpointsAPIPath, http.StatusOK,
		"ep1.example.com:443;ep2.example.com:443;ep3.example.com:443")
	defer srv.Close()

	var buf bytes.Buffer
	// Info level only — Verbose lines appear, Debug lines do not.
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	c := newTestClassicClient(t, srv.URL)
	_, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("ResolveEndpoints: %v", err)
	}

	logs := buf.String()
	if !strings.Contains(logs, "resolved tenant endpoints") {
		t.Errorf("missing verbose summary line:\n%s", logs)
	}
	// "tenant endpoint" (per-endpoint debug) must NOT appear; the verbose summary
	// "resolved tenant endpoints" is fine and already checked above.
	if strings.Contains(logs, `msg="tenant endpoint"`) {
		t.Errorf("per-endpoint debug lines should not appear at Info level:\n%s", logs)
	}
}

// --- CheckAllEndpoints unit tests ---

func addrToEndpoint(t *testing.T, addr string) Endpoint {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("addrToEndpoint(%q): %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("addrToEndpoint(%q): bad port: %v", addr, err)
	}
	return Endpoint{Host: host, Port: port}
}

func TestCheckAllEndpoints_AllReachable(t *testing.T) {
	ln1, addr1 := startTCPListener(t)
	ln2, addr2 := startTCPListener(t)
	defer ln1.Close()
	defer ln2.Close()
	go acceptLoop(ln1)
	go acceptLoop(ln2)

	report := CheckAllEndpoints([]Endpoint{addrToEndpoint(t, addr1), addrToEndpoint(t, addr2)}, time.Second)
	if !report.AllPassed {
		t.Error("AllPassed should be true when all endpoints are reachable")
	}
	if report.FailedCount != 0 {
		t.Errorf("FailedCount = %d, want 0", report.FailedCount)
	}
	for _, r := range report.Results {
		if !r.Reachable {
			t.Errorf("endpoint %s:%d reported unreachable", r.Endpoint.Host, r.Endpoint.Port)
		}
		if r.Latency <= 0 {
			t.Errorf("endpoint %s:%d has zero latency", r.Endpoint.Host, r.Endpoint.Port)
		}
	}
}

func TestCheckAllEndpoints_SomeBlocked(t *testing.T) {
	ln, addr := startTCPListener(t)
	defer ln.Close()
	go acceptLoop(ln)

	endpoints := []Endpoint{
		addrToEndpoint(t, addr),          // reachable
		{Host: "192.0.2.1", Port: 12345}, // RFC 5737 TEST-NET — never routed
	}

	old := defaultProbeTimeout
	defaultProbeTimeout = 50 * time.Millisecond
	defer func() { defaultProbeTimeout = old }()

	report := CheckAllEndpoints(endpoints, defaultProbeTimeout)
	if report.AllPassed {
		t.Error("AllPassed should be false when some endpoints are unreachable")
	}
	if report.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", report.FailedCount)
	}
}

func TestCheckAllEndpoints_AllBlocked(t *testing.T) {
	old := defaultProbeTimeout
	defaultProbeTimeout = 50 * time.Millisecond
	defer func() { defaultProbeTimeout = old }()

	endpoints := []Endpoint{
		{Host: "192.0.2.1", Port: 12345},
		{Host: "192.0.2.2", Port: 12346},
	}
	report := CheckAllEndpoints(endpoints, defaultProbeTimeout)
	if report.AllPassed {
		t.Error("AllPassed should be false when all endpoints are unreachable")
	}
	if report.FailedCount != 2 {
		t.Errorf("FailedCount = %d, want 2", report.FailedCount)
	}
}

func TestCheckAllEndpoints_RunsConcurrently(t *testing.T) {
	old := defaultProbeTimeout
	timeout := 100 * time.Millisecond
	defaultProbeTimeout = timeout
	defer func() { defaultProbeTimeout = old }()

	// 5 unreachable endpoints; if sequential they'd take ≥ 5 * timeout.
	// If concurrent, total time should be roughly one timeout.
	endpoints := make([]Endpoint, 5)
	for i := range endpoints {
		endpoints[i] = Endpoint{Host: "192.0.2.1", Port: 12340 + i}
	}

	start := time.Now()
	CheckAllEndpoints(endpoints, timeout)
	elapsed := time.Since(start)

	// Allow 3× the single timeout for scheduling overhead.
	if elapsed > 3*timeout {
		t.Errorf("CheckAllEndpoints took %v for 5 endpoints with %v timeout — expected concurrent execution", elapsed, timeout)
	}
}

func TestCheckAllEndpoints_DebugLogs(t *testing.T) {
	ln, addr := startTCPListener(t)
	defer ln.Close()
	go acceptLoop(ln)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	endpoints := []Endpoint{addrToEndpoint(t, addr)}
	CheckAllEndpoints(endpoints, time.Second)

	logs := buf.String()
	if !strings.Contains(logs, "endpoint probe result") {
		t.Errorf("missing 'endpoint probe result' debug line:\n%s", logs)
	}
}

// --- friendlyDialError ---

func TestFriendlyDialError(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"dial tcp 1.2.3.4:443: i/o timeout", "timed out"},
		{"context deadline exceeded (Client.Timeout exceeded while awaiting headers)", "timed out"},
		{"dial tcp 127.0.0.1:443: connect: connection refused", "connection refused"},
		{"dial tcp 1.2.3.4:443: connect: no route to host", "no route to host"},
		{"dial tcp 1.2.3.4:443: network is unreachable", "network unreachable"},
		{"read tcp: connection reset by peer", "connection reset"},
		{"some other unknown error from the OS", "unreachable"},
		{"", ""},
	}
	for _, c := range cases {
		if got := friendlyDialError(c.input); got != c.want {
			t.Errorf("friendlyDialError(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// --- InstallOneAgentV2 integration tests for Task 3 paths ---

func TestInstallOneAgentV2_PrintEndpoints(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("OneAgent not supported on macOS")
	}

	srv := newMockEndpointServer(t, "ep1.example.com:443;ep2.example.com:8080")
	defer srv.Close()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	c := newMockClient(t, srv.URL)
	err := InstallOneAgentV2(c, InstallOptions{
		MonitoringMode: "fullstack",
		PrintEndpoints: true,
	})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ep1.example.com:443") {
		t.Errorf("output missing ep1: %q", out)
	}
	if !strings.Contains(out, "ep2.example.com:8080") {
		t.Errorf("output missing ep2: %q", out)
	}
}

func TestInstallOneAgentV2_ConnectivityCheckOnly_NoDownload(t *testing.T) {
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
		downloadCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newMockClient(t, srv.URL)
	err := InstallOneAgentV2(c, InstallOptions{
		MonitoringMode:        "fullstack",
		ConnectivityCheckOnly: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if downloadCalled {
		t.Error("download API was called but should not be under --connectivity-check-only")
	}
}

func TestInstallOneAgentV2_SkipConnectivityCheck(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("OneAgent not supported on macOS")
	}

	srv := newMockEndpointServer(t, "192.0.2.1:12345")
	defer srv.Close()

	var debugBuf bytes.Buffer
	handler := slog.NewTextHandler(&debugBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	c := newMockClient(t, srv.URL)
	// ConnectivityCheckOnly ensures we exit before download so the test is self-contained.
	err := InstallOneAgentV2(c, InstallOptions{
		MonitoringMode:        "fullstack",
		SkipConnectivityCheck: true,
		ConnectivityCheckOnly: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(debugBuf.String(), "skipping connectivity probe") {
		t.Errorf("expected 'skipping connectivity probe' debug line:\n%s", debugBuf.String())
	}
}
