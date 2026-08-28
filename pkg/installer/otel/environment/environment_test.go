package environment

import (
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
			got := ProjectServiceName(tt.path)
			if got != tt.want {
				t.Errorf("projectServiceName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeServiceName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "my-service", "my-service"},
		{"forward slash", "ui/web", "ui-web"},
		{"back slash", "ui\\web", "ui-web"},
		{"colon", "group:artifact", "group-artifact"},
		{"gradle colon-prefixed", ":api:web", "-api-web"},
		{"mixed separators", "a/b\\c:d", "a-b-c-d"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeServiceName(tt.in)
			if got != tt.want {
				t.Errorf("NormalizeServiceName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGenerateBaseOtelEnvVars(t *testing.T) {
	envVars := GenerateBaseOtelEnvVars("http://127.0.0.1:4318", "my-svc")

	wantEndpoint := "http://127.0.0.1:4318"
	if got := envVars["OTEL_EXPORTER_OTLP_ENDPOINT"]; got != wantEndpoint {
		t.Errorf("ENDPOINT = %q, want %q", got, wantEndpoint)
	}

	if _, ok := envVars["OTEL_EXPORTER_OTLP_HEADERS"]; ok {
		t.Error("OTEL_EXPORTER_OTLP_HEADERS must not be present — credentials belong to the collector, not the app")
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

func TestGenerateBaseOtelEnvVars_NonDefaultPort(t *testing.T) {
	envVars := GenerateBaseOtelEnvVars("http://127.0.0.1:4320", "svc")
	want := "http://127.0.0.1:4320"
	if got := envVars["OTEL_EXPORTER_OTLP_ENDPOINT"]; got != want {
		t.Errorf("ENDPOINT = %q, want %q (non-default port should be preserved)", got, want)
	}
}

func TestFormatEnvVars(t *testing.T) {
	m := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}
	got := FormatEnvVars(m)
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
	got := FormatEnvVars(map[string]string{})
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

	got := FormatPrintableEnvVars(envVars)
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
