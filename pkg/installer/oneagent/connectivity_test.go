package oneagent

import (
	"net"
	"testing"
	"time"
)

// startTCPListener opens a TCP listener on a random local port and returns
// it alongside the host:port address. The caller is responsible for closing it.
func startTCPListener(t *testing.T) (*net.TCPListener, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return l.(*net.TCPListener), l.Addr().String()
}

func parseTestEndpoint(t *testing.T, addr string) Endpoint {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", addr, err)
	}
	ep, err := parseEndpoint(host + ":" + portStr)
	if err != nil {
		t.Fatalf("parseEndpoint: %v", err)
	}
	return ep
}

func TestCheckAllEndpoints_AllReachable(t *testing.T) {
	l1, addr1 := startTCPListener(t)
	defer l1.Close()
	l2, addr2 := startTCPListener(t)
	defer l2.Close()

	ep1 := parseTestEndpoint(t, addr1)
	ep2 := parseTestEndpoint(t, addr2)

	report := CheckAllEndpoints([]Endpoint{ep1, ep2}, 5*time.Second)

	if !report.AllPassed {
		t.Errorf("expected AllPassed=true, got false")
	}
	if report.FailedCount != 0 {
		t.Errorf("expected FailedCount=0, got %d", report.FailedCount)
	}
	for _, r := range report.Results {
		if !r.Reachable {
			t.Errorf("endpoint %s not reachable: %s", r.Endpoint, r.Error)
		}
		if r.Latency <= 0 {
			t.Errorf("expected positive latency, got %v", r.Latency)
		}
	}
}

func TestCheckAllEndpoints_Unreachable(t *testing.T) {
	// Port 1 is typically closed / refused on loopback.
	ep := Endpoint{Host: "127.0.0.1", Port: 1}

	report := CheckAllEndpoints([]Endpoint{ep}, 500*time.Millisecond)

	if report.AllPassed {
		t.Error("expected AllPassed=false for unreachable endpoint")
	}
	if report.FailedCount != 1 {
		t.Errorf("expected FailedCount=1, got %d", report.FailedCount)
	}
	if report.Results[0].Reachable {
		t.Error("expected Reachable=false")
	}
	if report.Results[0].Error == "" {
		t.Error("expected non-empty Error for unreachable endpoint")
	}
}

func TestCheckAllEndpoints_Parallel(t *testing.T) {
	// Start 5 listeners; probing them all should take ≈ max(latencies), not sum.
	const n = 5
	listeners := make([]*net.TCPListener, n)
	endpoints := make([]Endpoint, n)
	for i := range listeners {
		l, addr := startTCPListener(t)
		listeners[i] = l
		endpoints[i] = parseTestEndpoint(t, addr)
		defer l.Close()
	}

	start := time.Now()
	report := CheckAllEndpoints(endpoints, 5*time.Second)
	elapsed := time.Since(start)

	if !report.AllPassed {
		t.Errorf("expected all endpoints reachable, failed: %d", report.FailedCount)
	}
	// Parallel probes should complete well within 1s even with 5 endpoints.
	if elapsed > 2*time.Second {
		t.Errorf("probes took %v, expected parallel execution to finish faster", elapsed)
	}
}

func TestCheckAllEndpoints_Mixed(t *testing.T) {
	l, addr := startTCPListener(t)
	defer l.Close()
	reachable := parseTestEndpoint(t, addr)
	unreachable := Endpoint{Host: "127.0.0.1", Port: 1}

	report := CheckAllEndpoints([]Endpoint{reachable, unreachable}, 500*time.Millisecond)

	if report.AllPassed {
		t.Error("expected AllPassed=false when one endpoint is unreachable")
	}
	if report.FailedCount != 1 {
		t.Errorf("expected FailedCount=1, got %d", report.FailedCount)
	}
}
