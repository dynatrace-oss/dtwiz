package oneagent

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// ConnectivityResult holds the outcome of a single TCP probe.
type ConnectivityResult struct {
	Endpoint  Endpoint
	Reachable bool
	Latency   time.Duration
	Error     string
}

// ConnectivityReport aggregates all probe results.
type ConnectivityReport struct {
	Results     []ConnectivityResult
	AllPassed   bool
	FailedCount int
}

// CheckAllEndpoints probes each endpoint concurrently via TCP and returns a
// consolidated report. Probes run in parallel; total time is bounded by the
// slowest single probe.
func CheckAllEndpoints(endpoints []Endpoint, timeout time.Duration) ConnectivityReport {
	results := make([]ConnectivityResult, len(endpoints))
	var wg sync.WaitGroup

	for i, ep := range endpoints {
		wg.Add(1)
		go func(idx int, e Endpoint) {
			defer wg.Done()
			addr := fmt.Sprintf("%s:%d", e.Host, e.Port)
			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, timeout)
			latency := time.Since(start)

			var result ConnectivityResult
			result.Endpoint = e
			result.Latency = latency
			if err != nil {
				result.Reachable = false
				result.Error = err.Error()
			} else {
				conn.Close()
				result.Reachable = true
			}
			results[idx] = result
		}(i, ep)
	}
	wg.Wait()

	failed := 0
	for _, r := range results {
		if !r.Reachable {
			failed++
		}
		logger.Debug("endpoint probe result",
			"host", r.Endpoint.Host,
			"port", r.Endpoint.Port,
			"reachable", r.Reachable,
			"latency_ms", r.Latency.Milliseconds(),
			"error", r.Error,
		)
	}
	logger.Verbose("connectivity probe complete", "total", len(results), "failed", failed)

	return ConnectivityReport{
		Results:     results,
		AllPassed:   failed == 0,
		FailedCount: failed,
	}
}

// printConnectivityReport prints a status-line per endpoint to stdout.
func printConnectivityReport(report ConnectivityReport) {
	display.Header("Checking network connectivity...")
	for _, r := range report.Results {
		label := r.Endpoint.String()
		if r.Reachable {
			detail := fmt.Sprintf("%dms", r.Latency.Milliseconds())
			display.PrintStatusLine(label, "✓ "+detail, display.ColorOK)
		} else {
			display.PrintStatusLine(label, "✗ "+r.Error, display.ColorError)
		}
	}
}

// printConnectivityWarning prints a warning block for failed endpoints during a
// normal install run (failures are non-blocking).
func printConnectivityWarning(report ConnectivityReport) {
	display.Header("Warning: some endpoints are unreachable")
	for _, r := range report.Results {
		if !r.Reachable {
			display.PrintStatusLine(r.Endpoint.String(), "✗ "+r.Error, display.ColorError)
		}
	}
	display.PrintStatusLine("hint", "set HTTP_PROXY / HTTPS_PROXY if a proxy is required", display.ColorMuted)
}
