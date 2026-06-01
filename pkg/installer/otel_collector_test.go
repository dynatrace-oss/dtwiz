package installer

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestOtelCollectorInstallDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() failed: %v", err)
	}

	got, err := otelCollectorInstallDir()
	if err != nil {
		t.Fatalf("otelCollectorInstallDir() returned error: %v", err)
	}

	want := filepath.Join(home, "opentelemetry")
	if got != want {
		t.Errorf("otelCollectorInstallDir() = %q, want %q", got, want)
	}
}

func TestFindFreePort_ReturnsFreePort(t *testing.T) {
	port := findFreePort(8888)
	if port < 8888 {
		t.Fatalf("expected port >= 8888, got %d", port)
	}
	// Verify the returned port is bindable on all interfaces, matching collector behaviour.
	l, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("port %d returned by findFreePort is not free on 0.0.0.0: %v", port, err)
	}
	l.Close()
}

func TestFindFreePort_SkipsOccupied(t *testing.T) {
	// Occupy 8888 on all interfaces so findFreePort's 0.0.0.0 probe sees it as taken.
	l, err := net.Listen("tcp", "0.0.0.0:8888")
	if err != nil {
		t.Skip("cannot bind to 0.0.0.0:8888 — skipping")
	}
	defer l.Close()

	port := findFreePort(8888)
	if port == 8888 {
		t.Fatal("expected findFreePort to skip the occupied 8888 port")
	}
}

func TestDetectConfigFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args string
		want string
	}{
		{"unquoted path", "otelcol --config /etc/otel/config.yaml", "/etc/otel/config.yaml"},
		{"double-quoted path with spaces", `otelcol.exe --config "C:\Program Files\otelcol\config.yaml"`, `C:\Program Files\otelcol\config.yaml`},
		{"single-quoted path with spaces", "otelcol --config '/etc/otel/my config.yaml'", "/etc/otel/my config.yaml"},
		{"inline = form", "otelcol --config=/etc/otel/config.yaml", "/etc/otel/config.yaml"},
		{"inline = with double quotes", `otelcol --config="C:\Program Files\config.yaml"`, `C:\Program Files\config.yaml`},
		{"short flag -c", "otelcol -c /etc/otel/config.yaml", "/etc/otel/config.yaml"},
		{"no config flag", "otelcol --other-flag value", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectConfigFromArgs(tc.args)
			if got != tc.want {
				t.Errorf("detectConfigFromArgs(%q) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestGenerateOtelConfig_ContainsMetricsPort(t *testing.T) {
	cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	if !strings.Contains(cfg, "port:") {
		t.Errorf("generated config missing metrics port:\n%s", cfg)
	}
	if !strings.Contains(cfg, "readers:") {
		t.Errorf("generated config missing readers section:\n%s", cfg)
	}
}
