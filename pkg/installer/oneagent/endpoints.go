package oneagent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// Endpoint represents a single OneAgent communication endpoint.
type Endpoint struct {
	Host string
	Port int
}

func (e Endpoint) String() string {
	return fmt.Sprintf("%s:%d", e.Host, e.Port)
}

// ResolveEndpoints calls the Dynatrace tenant API to discover OneAgent
// communication endpoints. It returns an error if the API is unreachable,
// returns a non-2xx status, or returns an empty endpoint list.
func ResolveEndpoints(c *client.ClassicClient) ([]Endpoint, error) {
	const path = "/api/v1/deployment/installer/agent/connectioninfo/endpoints"
	reqURL := strings.TrimRight(c.BaseURL(), "/") + path
	logger.Debug("resolving tenant endpoints", "url", reqURL)

	resp, err := c.HTTP().R().Get(path)
	if err != nil {
		return nil, fmt.Errorf("endpoint resolution network error: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("endpoint resolution failed (status %d, url %s): %s",
			resp.StatusCode(), reqURL, strings.TrimSpace(resp.String()))
	}

	body := strings.TrimSpace(resp.String())
	if body == "" {
		return nil, fmt.Errorf("tenant returned no endpoints (empty response from %s)", reqURL)
	}

	var endpoints []Endpoint
	for _, part := range strings.Split(body, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ep, err := parseEndpoint(part)
		if err != nil {
			return nil, fmt.Errorf("malformed endpoint %q: %w", part, err)
		}
		endpoints = append(endpoints, ep)
	}

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("tenant returned no endpoints (empty response from %s)", reqURL)
	}

	for _, e := range endpoints {
		logger.Debug("tenant endpoint", "host", e.Host, "port", e.Port)
	}
	logger.Verbose("resolved tenant endpoints", "count", len(endpoints))

	return endpoints, nil
}

func parseEndpoint(s string) (Endpoint, error) {
	// Strip any leading scheme (https:// etc.) that might appear in some responses.
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	// Strip any trailing path.
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}

	// host:port or bare host
	lastColon := strings.LastIndex(s, ":")
	if lastColon < 0 {
		return Endpoint{Host: s, Port: 443}, nil
	}
	// Distinguish IPv6 addresses (which also contain colons) by checking for brackets.
	if strings.HasPrefix(s, "[") {
		// IPv6 with port: [::1]:443
		host := s[1:strings.Index(s, "]")]
		portStr := s[strings.Index(s, "]")+2:]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return Endpoint{}, fmt.Errorf("invalid port %q", portStr)
		}
		return Endpoint{Host: host, Port: port}, nil
	}

	host := s[:lastColon]
	portStr := s[lastColon+1:]
	if portStr == "" {
		return Endpoint{Host: host, Port: 443}, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Endpoint{}, fmt.Errorf("invalid port %q", portStr)
	}
	return Endpoint{Host: host, Port: port}, nil
}

// logTenantID emits a debug line for the extracted tenant ID. Only the first
// DNS label is logged — never the full URL which may contain credentials.
func logTenantID(environmentURL string) {
	id := installer.ExtractTenantID(environmentURL)
	logger.Debug("extracted tenant id", "tenant_id", id)
}
