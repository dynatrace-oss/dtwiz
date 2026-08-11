package otel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPortsFromConfig_ReturnsDefaults_OnMissingFile(t *testing.T) {
	grpc, http, metrics, hc := portsFromConfig("/nonexistent/path/config.yaml")
	if grpc != 4317 || http != 4318 || metrics != 8888 || hc != 13133 {
		t.Errorf("expected canonical defaults, got grpc=%d http=%d metrics=%d hc=%d", grpc, http, metrics, hc)
	}
}

func TestPortsFromConfig_ParsesAllPorts(t *testing.T) {
	cfg := `
extensions:
  health_check:
    endpoint: 0.0.0.0:13200
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:5317
      http:
        endpoint: 0.0.0.0:5318
service:
  telemetry:
    metrics:
      readers:
        - pull:
            exporter:
              prometheus:
                host: localhost
                port: 9999
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp_http]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	grpc, http, metrics, hc := portsFromConfig(path)
	if grpc != 5317 {
		t.Errorf("grpc: want 5317, got %d", grpc)
	}
	if http != 5318 {
		t.Errorf("http: want 5318, got %d", http)
	}
	if metrics != 9999 {
		t.Errorf("metrics: want 9999, got %d", metrics)
	}
	if hc != 13200 {
		t.Errorf("health_check: want 13200, got %d", hc)
	}
}
