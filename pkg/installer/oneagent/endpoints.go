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

// A variable so tests can override it without waiting 5 seconds per unreachable probe.
var defaultProbeTimeout = 5 * time.Second

type Endpoint struct {
	Host string
	Port int
}

type ConnectivityResult struct {
	Endpoint  Endpoint
	Reachable bool
	Latency   time.Duration
	Error     string
}

type ConnectivityReport struct {
	Results     []ConnectivityResult
	AllPassed   bool
	FailedCount int
}

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

// Accepted forms: "host:port", "host" (→ port 443), "https://host:port/path" (scheme+path stripped).
func parseEndpoint(s string) (Endpoint, error) {
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	// IPv6 literals are always bracketed in URLs, so the first '/' after ']' is safe to trim.
	if slash := strings.Index(s, "/"); slash >= 0 {
		s = s[:slash]
	}

	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		logger.Debug("endpoint token has no port, defaulting to 443", "host", s)
		return Endpoint{Host: s, Port: 443}, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Endpoint{}, fmt.Errorf("invalid port %q: %w", portStr, err)
	}
	return Endpoint{Host: host, Port: port}, nil
}

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

// Caller must print the section header before calling this so it appears before the dial window.
func printConnectivityResults(report ConnectivityReport) {
	for _, r := range report.Results {
		label := fmt.Sprintf("%s:%d", r.Endpoint.Host, r.Endpoint.Port)
		if r.Reachable {
			display.PrintStatusLine(label, fmt.Sprintf("✓ %s", r.Latency.Round(time.Millisecond)), display.ColorOK)
		} else {
			display.PrintStatusLine(label, fmt.Sprintf("✗ %s", friendlyDialError(r.Error)), display.ColorError)
		}
	}
}

func printConnectivityWarning(report ConnectivityReport) {
	display.Header("Warning: connectivity check failed")
	display.PrintStatusLine("action", "allow outbound TCP to the following addresses", display.ColorWarning)
	display.PrintSectionDivider()
	for _, r := range report.Results {
		if !r.Reachable {
			label := fmt.Sprintf("%s:%d", r.Endpoint.Host, r.Endpoint.Port)
			display.PrintStatusLine(label, fmt.Sprintf("✗ %s", friendlyDialError(r.Error)), display.ColorError)
		}
	}
	display.PrintSectionDivider()
	display.PrintStatusLine("tip", "if a proxy is required, set HTTP_PROXY / HTTPS_PROXY", display.ColorWarning)
}

func friendlyDialError(errStr string) string {
	switch {
	case strings.Contains(errStr, "i/o timeout"),
		strings.Contains(errStr, "timed out"),
		strings.Contains(errStr, "deadline exceeded"):
		return "timed out"
	case strings.Contains(errStr, "connection refused"):
		return "connection refused"
	case strings.Contains(errStr, "no route to host"):
		return "no route to host"
	case strings.Contains(errStr, "network is unreachable"):
		return "network unreachable"
	case strings.Contains(errStr, "connection reset"):
		return "connection reset"
	case errStr == "":
		return ""
	default:
		return "unreachable"
	}
}
