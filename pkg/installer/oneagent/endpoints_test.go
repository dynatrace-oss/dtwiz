package oneagent

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
)

func newEndpointServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/deployment/installer/agent/connectioninfo/endpoints",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, body)
		})
	return httptest.NewServer(mux)
}

func TestResolveEndpoints_HappyPath(t *testing.T) {
	srv := newEndpointServer(t, http.StatusOK, "endpoint-1.example.com:443;endpoint-2.example.com:443")
	defer srv.Close()
	c := newTestClassicClient(t, srv.URL)

	endpoints, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}
	if endpoints[0].Host != "endpoint-1.example.com" || endpoints[0].Port != 443 {
		t.Errorf("endpoints[0] = %+v, want {endpoint-1.example.com 443}", endpoints[0])
	}
	if endpoints[1].Host != "endpoint-2.example.com" || endpoints[1].Port != 443 {
		t.Errorf("endpoints[1] = %+v, want {endpoint-2.example.com 443}", endpoints[1])
	}
}

func TestResolveEndpoints_DefaultPort(t *testing.T) {
	srv := newEndpointServer(t, http.StatusOK, "endpoint-1.example.com")
	defer srv.Close()
	c := newTestClassicClient(t, srv.URL)

	endpoints, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0].Port != 443 {
		t.Errorf("expected default port 443, got %d", endpoints[0].Port)
	}
}

func TestResolveEndpoints_IPLiteral(t *testing.T) {
	srv := newEndpointServer(t, http.StatusOK, "54.88.45.104:443")
	defer srv.Close()
	c := newTestClassicClient(t, srv.URL)

	endpoints, err := ResolveEndpoints(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0].Host != "54.88.45.104" || endpoints[0].Port != 443 {
		t.Errorf("endpoints[0] = %+v, want {54.88.45.104 443}", endpoints[0])
	}
}

func TestResolveEndpoints_Unauthorized(t *testing.T) {
	srv := newEndpointServer(t, http.StatusUnauthorized, "Invalid token")
	defer srv.Close()
	c := newTestClassicClient(t, srv.URL)

	_, err := ResolveEndpoints(c)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want 401 in message", err)
	}
}

func TestResolveEndpoints_AuthFailuresAreAuthErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantReason string
	}{
		{"401 unauthorized", http.StatusUnauthorized, "authentication_failed"},
		{"403 forbidden", http.StatusForbidden, "insufficient_permissions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newEndpointServer(t, tt.status, "denied")
			defer srv.Close()

			_, err := ResolveEndpoints(newTestClassicClient(t, srv.URL))

			var authErr *installer.AuthError
			if !errors.As(err, &authErr) {
				t.Fatalf("error %v is not an AuthError", err)
			}
			if authErr.Reason != tt.wantReason {
				t.Errorf("Reason = %q, want %q", authErr.Reason, tt.wantReason)
			}
			if !strings.Contains(err.Error(), "InstallerDownload") {
				t.Errorf("error %q does not name the InstallerDownload scope", err)
			}
		})
	}
}

// Reproduces mqa14477 (Oct 16): a valid token without the installer permission gets 403 on the
// connection-info call. dtwiz reported install_failed; it must report auth_error with the
// insufficient_permissions reason so the failure is actionable.
func TestResolveEndpoints_403_ReportedAsAuthError(t *testing.T) {
	srv := newEndpointServer(t, http.StatusForbidden, "")
	defer srv.Close()

	_, err := ResolveEndpoints(newTestClassicClient(t, srv.URL))

	errType, attrs := selfmonitoring.ClassifyError(err)
	if errType != installer.ErrTypeAuthError {
		t.Fatalf("reported as %q, want %q (error: %v)", errType, installer.ErrTypeAuthError, err)
	}
	if attrs["auth.failure_reason"] != "insufficient_permissions" {
		t.Errorf("attrs = %v, want auth.failure_reason=insufficient_permissions", attrs)
	}
}

func TestResolveEndpoints_ServerErrorIsNotAuthError(t *testing.T) {
	srv := newEndpointServer(t, http.StatusServiceUnavailable, "")
	defer srv.Close()

	_, err := ResolveEndpoints(newTestClassicClient(t, srv.URL))

	var authErr *installer.AuthError
	if errors.As(err, &authErr) {
		t.Errorf("503 classified as AuthError: %v", err)
	}
}

func TestResolveEndpoints_NetworkErrorIsNetworkError(t *testing.T) {
	srv := newEndpointServer(t, http.StatusOK, "")
	c := newTestClassicClient(t, srv.URL)
	srv.Close()

	_, err := ResolveEndpoints(c)

	var netErr *installer.NetworkError
	if !errors.As(err, &netErr) {
		t.Fatalf("error %v is not a NetworkError", err)
	}
	if netErr.Reason != "environment_not_reachable" || !strings.Contains(netErr.URL, endpointsAPIPath) {
		t.Errorf("NetworkError = %+v, want environment_not_reachable for %s", netErr, endpointsAPIPath)
	}
}

func TestResolveEndpoints_ServerError(t *testing.T) {
	srv := newEndpointServer(t, http.StatusServiceUnavailable, "")
	defer srv.Close()
	c := newTestClassicClient(t, srv.URL)

	_, err := ResolveEndpoints(c)
	if err == nil {
		t.Fatal("expected error for 503")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want 503 in message", err)
	}
}

func TestResolveEndpoints_EmptyBody(t *testing.T) {
	srv := newEndpointServer(t, http.StatusOK, "")
	defer srv.Close()
	c := newTestClassicClient(t, srv.URL)

	_, err := ResolveEndpoints(c)
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "no endpoints") {
		t.Errorf("error = %q, want 'no endpoints' message", err)
	}
}

func TestParseEndpoint_BareHost(t *testing.T) {
	ep, err := parseEndpoint("host.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.Host != "host.example.com" || ep.Port != 443 {
		t.Errorf("got %+v, want {host.example.com 443}", ep)
	}
}

func TestParseEndpoint_HostWithPort(t *testing.T) {
	ep, err := parseEndpoint("host.example.com:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.Host != "host.example.com" || ep.Port != 8080 {
		t.Errorf("got %+v, want {host.example.com 8080}", ep)
	}
}

func TestEndpointString(t *testing.T) {
	ep := Endpoint{Host: "host.example.com", Port: 443}
	if ep.String() != "host.example.com:443" {
		t.Errorf("String() = %q, want %q", ep.String(), "host.example.com:443")
	}
}
