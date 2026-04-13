package installer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectServiceName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/projects/my-api", "my-api"},
		{"/opt/services/backend", "backend"},
		{"", "my-service"},
		{".", "my-service"},
		{"/", "my-service"},
		{"/single", "single"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := projectServiceName(tt.path)
			if got != tt.want {
				t.Errorf("projectServiceName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestGenerateBaseOtelEnvVars(t *testing.T) {
	envVars := generateBaseOtelEnvVars("https://abc123.live.dynatrace.com", "dt0c01.TOKEN", "my-svc")

	wantEndpoint := "https://abc123.live.dynatrace.com/api/v2/otlp"
	if got := envVars["OTEL_EXPORTER_OTLP_ENDPOINT"]; got != wantEndpoint {
		t.Errorf("ENDPOINT = %q, want %q", got, wantEndpoint)
	}

	wantHeaders := "Authorization=Api-Token%20dt0c01.TOKEN"
	if got := envVars["OTEL_EXPORTER_OTLP_HEADERS"]; got != wantHeaders {
		t.Errorf("HEADERS = %q, want %q", got, wantHeaders)
	}

	if got := envVars["OTEL_SERVICE_NAME"]; got != "my-svc" {
		t.Errorf("SERVICE_NAME = %q, want %q", got, "my-svc")
	}

	if got := envVars["OTEL_EXPORTER_OTLP_PROTOCOL"]; got != "http/protobuf" {
		t.Errorf("PROTOCOL = %q, want %q", got, "http/protobuf")
	}

	if got := envVars["OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE"]; got != "delta" {
		t.Errorf("TEMPORALITY = %q, want %q", got, "delta")
	}

	for _, key := range []string{"OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER"} {
		if got := envVars[key]; got != "otlp" {
			t.Errorf("%s = %q, want %q", key, got, "otlp")
		}
	}
}

func TestGenerateBaseOtelEnvVars_TrailingSlash(t *testing.T) {
	envVars := generateBaseOtelEnvVars("https://abc123.live.dynatrace.com/", "tok", "svc")
	want := "https://abc123.live.dynatrace.com/api/v2/otlp"
	if got := envVars["OTEL_EXPORTER_OTLP_ENDPOINT"]; got != want {
		t.Errorf("ENDPOINT = %q, want %q (trailing slash should be stripped)", got, want)
	}
}

func TestFormatEnvVars(t *testing.T) {
	m := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}
	got := formatEnvVars(m)
	want := []string{"BAZ=qux", "FOO=bar"}
	if len(got) != len(want) {
		t.Fatalf("formatEnvVars length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("formatEnvVars[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFormatEnvVars_Empty(t *testing.T) {
	got := formatEnvVars(map[string]string{})
	if len(got) != 0 {
		t.Errorf("formatEnvVars(empty) = %v, want empty", got)
	}
}

func TestGenerateEnvExportScript(t *testing.T) {
	envVars := map[string]string{
		"OTEL_EXPORTER_OTLP_HEADERS":  "Authorization=Api-Token%20dt0c01.secret",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
		"OTEL_SERVICE_NAME":           "my-svc",
	}
	script := GenerateEnvExportScript(envVars)
	lines := strings.Split(strings.TrimSpace(script), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 export lines, got %d in %q", len(lines), script)
	}
	if lines[0] != "export OTEL_EXPORTER_OTLP_HEADERS=\"Authorization=Api-Token%20<redacted>\"" {
		t.Errorf("unexpected first line %q", lines[0])
	}
	if lines[1] != "export OTEL_EXPORTER_OTLP_PROTOCOL=\"http/protobuf\"" {
		t.Errorf("unexpected second line %q", lines[1])
	}
	if lines[2] != "export OTEL_SERVICE_NAME=\"my-svc\"" {
		t.Errorf("unexpected third line %q", lines[2])
	}
	if !strings.Contains(script, "export OTEL_SERVICE_NAME=") {
		t.Errorf("script missing export line, got:\n%s", script)
	}
	if !strings.Contains(script, "my-svc") {
		t.Errorf("script missing service name, got:\n%s", script)
	}
	if strings.Contains(script, "dt0c01.secret") {
		t.Errorf("script leaked token, got:\n%s", script)
	}
}

func TestFormatPrintableEnvVars(t *testing.T) {
	envVars := map[string]string{
		"OTEL_EXPORTER_OTLP_HEADERS":  "Authorization=Api-Token%20dt0c01.secret",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
	}

	got := formatPrintableEnvVars(envVars)
	want := []string{
		"OTEL_EXPORTER_OTLP_HEADERS=Authorization=Api-Token%20<redacted>",
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf",
	}

	if len(got) != len(want) {
		t.Fatalf("formatPrintableEnvVars length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("formatPrintableEnvVars[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWaitForServices_LinkContainsDieter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(dqlResponse{
			Result: struct {
				Records []map[string]interface{} `json:"records"`
			}{
				Records: []map[string]interface{}{
					{"name": "my-svc"},
				},
			},
		})
	}))
	defer server.Close()

	output := captureStdout(t, func() {
		waitForServices(server.URL, "dt0s16.token", []string{"my-svc"})
	})

	const wantSubstr = "my.getting.started.dieter"
	if !strings.Contains(output, wantSubstr) {
		t.Errorf("output does not contain %q:\n%s", wantSubstr, output)
	}
}

func TestFetchSmartscapeServiceNames(t *testing.T) {
	var receivedAuthorization string
	var receivedContentType string
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthorization = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")

		var payload struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		receivedQuery = payload.Query

		_ = json.NewEncoder(w).Encode(dqlResponse{
			Result: struct {
				Records []map[string]interface{} `json:"records"`
			}{
				Records: []map[string]interface{}{
					{"name": "orders-api"},
					{"name": "checkout-api"},
					{"ignored": true},
				},
			},
		})
	}))
	defer server.Close()

	serviceNames := fetchSmartscapeServiceNames(server.URL, "dt0s16.platform-token", "smartscapeNodes SERVICE | limit 2")

	if receivedAuthorization != "Bearer dt0s16.platform-token" {
		t.Fatalf("unexpected authorization header %q", receivedAuthorization)
	}
	if receivedContentType != "application/json" {
		t.Fatalf("unexpected content type %q", receivedContentType)
	}
	if receivedQuery != "smartscapeNodes SERVICE | limit 2" {
		t.Fatalf("unexpected DQL query %q", receivedQuery)
	}
	if len(serviceNames) != 2 || serviceNames[0] != "orders-api" || serviceNames[1] != "checkout-api" {
		t.Fatalf("unexpected service names %v", serviceNames)
	}
}
