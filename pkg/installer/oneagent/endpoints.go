package oneagent

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const endpointsAPIPath = "/api/v1/deployment/installer/agent/connectioninfo/endpoints"

// defaultProbeTimeout is the per-endpoint TCP dial timeout. A variable so tests
// can override it without waiting 5 seconds per unreachable probe.
var defaultProbeTimeout = 5 * time.Second

// Endpoint is a OneAgent communication endpoint resolved from the tenant API.
type Endpoint struct {
	Host string
	Port int
}

// ConnectivityResult holds the TCP probe outcome for a single endpoint.
type ConnectivityResult struct {
	Endpoint  Endpoint
	Reachable bool
	Latency   time.Duration
	Error     string
}

// ConnectivityReport aggregates per-endpoint probe results.
type ConnectivityReport struct {
	Results     []ConnectivityResult
	AllPassed   bool
	FailedCount int
}

// ResolveEndpoints calls GET /api/v1/deployment/installer/agent/connectioninfo/endpoints
// and returns the parsed list. The API response is a newline- or semicolon-separated
// list of entries: "host:port", bare "host" (defaults to port 443), or full URL
// "https://host:port/path" (scheme and path are stripped). All three forms
// tolerate both DNS hostnames and IP literals (including IPv6 bracket notation).
func ResolveEndpoints(c *client.ClassicClient) ([]Endpoint, error) {
	reqURL := strings.TrimRight(c.BaseURL(), "/") + endpointsAPIPath
	logger.Debug("resolving tenant endpoints", "url", reqURL)

	resp, err := c.HTTP().R().Get(endpointsAPIPath)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", reqURL, err)
	}

	if resp.StatusCode() >= 400 {
		body := strings.TrimSpace(resp.String())
		return nil, fmt.Errorf("GET %s returned HTTP %d: %s", reqURL, resp.StatusCode(), body)
	}

	body := strings.TrimSpace(resp.String())
	if body == "" {
		return nil, fmt.Errorf("tenant returned no endpoints")
	}

	var endpoints []Endpoint
	// Split on semicolons and newlines; \r\n line endings are handled by the
	// '\r' case so we never get stray carriage-returns in host tokens.
	for _, token := range strings.FieldsFunc(body, func(r rune) bool {
		return r == ';' || r == '\n' || r == '\r'
	}) {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		ep, parseErr := parseEndpoint(token)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing endpoint %q: %w", token, parseErr)
		}
		logger.Debug("tenant endpoint", "host", ep.Host, "port", ep.Port)
		endpoints = append(endpoints, ep)
	}

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("tenant returned no endpoints")
	}

	logger.Verbose("resolved tenant endpoints", "count", len(endpoints))
	return endpoints, nil
}

// parseEndpoint parses a single endpoint token into an Endpoint.
// Accepted forms (all tolerate DNS hostnames and IP literals):
//   - "host:port"           → {host, port}
//   - "host"                → {host, 443}
//   - "https://host:port/x" → scheme + path stripped → {host, port}
func parseEndpoint(s string) (Endpoint, error) {
	// Strip optional scheme (https://host:port/path → host:port/path).
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	// Strip optional path component (host:port/path → host:port).
	// net.SplitHostPort handles brackets for IPv6, so we must not split on
	// the first '/' blindly when the host is an IPv6 literal — but IPv6
	// addresses in URLs are always bracketed, so the first '/' after the
	// closing ']' is safe to trim.
	if slash := strings.Index(s, "/"); slash >= 0 {
		s = s[:slash]
	}

	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		// No port present — use default 443.
		return Endpoint{Host: s, Port: 443}, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Endpoint{}, fmt.Errorf("invalid port %q: %w", portStr, err)
	}
	return Endpoint{Host: host, Port: port}, nil
}

// CheckAllEndpoints probes each endpoint concurrently via net.DialTimeout and
// returns a ConnectivityReport with per-endpoint reachability, latency, and errors.
func CheckAllEndpoints(endpoints []Endpoint, timeout time.Duration) ConnectivityReport {
	results := make([]ConnectivityResult, len(endpoints))
	var wg sync.WaitGroup
	for i, ep := range endpoints {
		wg.Add(1)
		go func(i int, ep Endpoint) {
			defer wg.Done()
			addr := net.JoinHostPort(ep.Host, strconv.Itoa(ep.Port))
			start := time.Now()
			conn, dialErr := net.DialTimeout("tcp", addr, timeout)
			latency := time.Since(start)
			r := ConnectivityResult{Endpoint: ep, Latency: latency}
			if dialErr != nil {
				r.Error = dialErr.Error()
			} else {
				conn.Close()
				r.Reachable = true
			}
			logger.Debug("endpoint probe result",
				"host", ep.Host,
				"port", ep.Port,
				"reachable", r.Reachable,
				"latency_ms", latency.Milliseconds(),
				"error", r.Error,
			)
			results[i] = r
		}(i, ep)
	}
	wg.Wait()

	failed := 0
	for _, r := range results {
		if !r.Reachable {
			failed++
		}
	}
	report := ConnectivityReport{
		Results:     results,
		AllPassed:   failed == 0,
		FailedCount: failed,
	}
	logger.Verbose("connectivity probe complete", "total", len(results), "failed", failed)
	return report
}

// printConnectivityReport outputs a full probe table — used by --connectivity-check-only.
func printConnectivityReport(report ConnectivityReport) {
	display.Header("Checking network connectivity...")
	for _, r := range report.Results {
		label := fmt.Sprintf("%s:%d", r.Endpoint.Host, r.Endpoint.Port)
		if r.Reachable {
			display.PrintStatusLine(label, fmt.Sprintf("✓ %s", r.Latency.Round(time.Millisecond)), display.ColorOK)
		} else {
			display.PrintStatusLine(label, fmt.Sprintf("✗ %s", r.Error), display.ColorError)
		}
	}
}

// printConnectivityWarning outputs a warning block with only the failed endpoints,
// used in the normal install path when some endpoints are unreachable.
func printConnectivityWarning(report ConnectivityReport) {
	display.Header("Warning: some endpoints could not be reached")
	for _, r := range report.Results {
		if !r.Reachable {
			label := fmt.Sprintf("%s:%d", r.Endpoint.Host, r.Endpoint.Port)
			display.PrintStatusLine(label, r.Error, display.ColorError)
		}
	}
	display.PrintStatusLine("tip", "set HTTP_PROXY / HTTPS_PROXY if a proxy is required", display.ColorWarning)
}
