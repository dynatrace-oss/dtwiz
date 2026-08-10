package otel

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
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
	// The returned port must actually be bindable on localhost (matching the collector's Prometheus bind address).
	l, err := net.Listen("tcp", "localhost:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("port %d returned by findFreePort is not free: %v", port, err)
	}
	l.Close()
}

func TestFindFreePort_SkipsOccupied(t *testing.T) {
	// Occupy 8888 on localhost; findFreePort should hand back the next free port.
	l, err := net.Listen("tcp", "localhost:8888")
	if err != nil {
		t.Skip("cannot bind to localhost:8888 — skipping")
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
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")

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

// otelConfigYAML is a minimal map type for inspecting the rendered config.
type otelConfigYAML struct {
	Extensions map[string]any `yaml:"extensions"`
	Receivers  map[string]any `yaml:"receivers"`
	Processors map[string]any `yaml:"processors"`
	Service    struct {
		Extensions []string `yaml:"extensions"`
		Pipelines  map[string]struct {
			Receivers  []string `yaml:"receivers"`
			Processors []string `yaml:"processors"`
			Exporters  []string `yaml:"exporters"`
		} `yaml:"pipelines"`
	} `yaml:"service"`
}

func parseOtelConfig(t *testing.T, cfg string) otelConfigYAML {
	t.Helper()
	var parsed otelConfigYAML
	if err := yaml.Unmarshal([]byte(cfg), &parsed); err != nil {
		t.Fatalf("rendered config is not valid YAML: %v\n---\n%s", err, cfg)
	}
	return parsed
}

// TestGenerateOtelConfig_AppOnly_Default asserts that without the experimental flag
// the config is app-only: no hostmetrics/journald/health_check, pipeline named "metrics".
func TestGenerateOtelConfig_AppOnly_Default(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")

	cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	parsed := parseOtelConfig(t, cfg)

	if _, ok := parsed.Receivers["hostmetrics/10s"]; ok {
		t.Error("app-only config must not contain hostmetrics/10s receiver")
	}
	if _, ok := parsed.Receivers["journald"]; ok {
		t.Error("app-only config must not contain journald receiver")
	}
	if _, ok := parsed.Extensions["health_check"]; ok {
		t.Error("app-only config must not contain health_check extension")
	}
	if _, ok := parsed.Service.Pipelines["metrics/host"]; ok {
		t.Error("app-only config must not contain metrics/host pipeline")
	}
	if _, ok := parsed.Service.Pipelines["logs/host"]; ok {
		t.Error("app-only config must not contain logs/host pipeline")
	}
	if _, ok := parsed.Service.Pipelines["metrics"]; !ok {
		t.Error("app-only config must contain metrics pipeline")
	}
}

// TestGenerateOtelConfig_Combined_ExperimentalEnabled asserts that with the experimental flag
// the combined config includes host receivers, all five processors in order,
// metrics/host and logs/host pipelines, and the app pipelines.
func TestGenerateOtelConfig_Combined_ExperimentalEnabled(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	parsed := parseOtelConfig(t, cfg)

	// Host receivers present.
	for _, recv := range []string{"host_metrics/10s", "host_metrics/5m", "host_metrics/1h"} {
		if _, ok := parsed.Receivers[recv]; !ok {
			t.Errorf("combined config missing receiver %q", recv)
		}
	}

	// health_check extension present.
	if _, ok := parsed.Extensions["health_check"]; !ok {
		t.Error("combined config missing health_check extension")
	}

	// metrics/host pipeline with processors in correct order.
	hostPipeline, ok := parsed.Service.Pipelines["metrics/host"]
	if !ok {
		t.Fatal("combined config missing metrics/host pipeline")
	}
	wantProcessors := []string{"filter", "resource_detection", "transform", "filter/delete-metrics", "cumulative_to_delta"}
	if len(hostPipeline.Processors) != len(wantProcessors) {
		t.Errorf("metrics/host processors: got %v, want %v", hostPipeline.Processors, wantProcessors)
	} else {
		for i, p := range wantProcessors {
			if hostPipeline.Processors[i] != p {
				t.Errorf("metrics/host processors[%d]: got %q, want %q", i, hostPipeline.Processors[i], p)
			}
		}
	}

	// logs/host pipeline is only emitted on Linux (journald required).
	if runtime.GOOS == "linux" {
		if _, ok := parsed.Service.Pipelines["logs/host"]; !ok {
			t.Error("combined config missing logs/host pipeline on Linux")
		}
	} else {
		if _, ok := parsed.Service.Pipelines["logs/host"]; ok {
			t.Error("combined config must not contain logs/host pipeline on non-Linux (no journald)")
		}
	}

	// app pipelines still present.
	if _, ok := parsed.Service.Pipelines["metrics/apps"]; !ok {
		t.Error("combined config missing metrics/apps pipeline")
	}
	if _, ok := parsed.Service.Pipelines["traces"]; !ok {
		t.Error("combined config missing traces pipeline")
	}
	if _, ok := parsed.Service.Pipelines["logs"]; !ok {
		t.Error("combined config missing logs pipeline")
	}
}

// TestGenerateOtelConfig_Combined_EnvVar asserts experimental can be enabled via env var.
func TestGenerateOtelConfig_Combined_EnvVar(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "true")

	cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	parsed := parseOtelConfig(t, cfg)

	if _, ok := parsed.Service.Pipelines["metrics/host"]; !ok {
		t.Error("expected metrics/host pipeline when DTWIZ_EXPERIMENTAL=true")
	}
}

// TestGenerateOtelConfig_JournaldConsistency asserts the journald receiver and
// its reference in logs/host are both present or both absent — never just one.
// It runs on the current platform; journald will be absent on non-Linux, and the
// test verifies the pipeline reference is absent too (the guards stay in sync).
func TestGenerateOtelConfig_JournaldConsistency(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	parsed := parseOtelConfig(t, cfg)

	_, receiverDefined := parsed.Receivers["journald"]
	logsHost, logsHostExists := parsed.Service.Pipelines["logs/host"]
	var pipelineReferences bool
	if logsHostExists {
		for _, r := range logsHost.Receivers {
			if r == "journald" {
				pipelineReferences = true
				break
			}
		}
	}

	// Both present or both absent — never a mismatch.
	if receiverDefined != pipelineReferences {
		t.Errorf("journald receiver defined=%v but pipeline reference=%v — guards must stay in sync",
			receiverDefined, pipelineReferences)
	}
}

// TestGenerateOtelConfig_ValidYAML asserts the rendered config parses as valid YAML.
func TestGenerateOtelConfig_ValidYAML(t *testing.T) {
	for _, experimental := range []bool{false, true} {
		experimental := experimental
		name := "app-only"
		if experimental {
			name = "combined"
		}
		t.Run(name, func(t *testing.T) {
			if experimental {
				featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)
			} else {
				featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
				t.Setenv("DTWIZ_EXPERIMENTAL", "")
			}
			cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
			if err != nil {
				t.Fatalf("generateOtelConfig: %v", err)
			}
			var parsed any
			if err := yaml.Unmarshal([]byte(cfg), &parsed); err != nil {
				t.Errorf("rendered config is not valid YAML: %v\n---\n%s", err, cfg)
			}
		})
	}
}

// TestGenerateOtelConfig_PreviewTruncation asserts that the generated config is long
// enough to be truncated in the default (non-verbose) preview for the experimental path.
func TestGenerateOtelConfig_PreviewTruncation(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	lines := strings.Split(strings.TrimRight(cfg, "\n"), "\n")
	if len(lines) <= 20 {
		t.Errorf("experimental config has %d lines, expected more than 20 so truncation is exercised", len(lines))
	}
}

// TestGenerateOtelConfig_TokenMaskedInPreview asserts the token is masked in the preview.
func TestGenerateOtelConfig_TokenMaskedInPreview(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")

	const token = "dt0s16.supersecrettoken"
	cfg, err := generateOtelConfig("https://env.example.com", token)
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}

	preview := installer.MaskSecret(cfg, token)
	if strings.Contains(preview, token) {
		t.Error("token must be masked in the config preview")
	}
	if !strings.Contains(preview, "***") {
		t.Error("masked preview must contain '***' placeholder")
	}
}

func TestConfigHeadEnd(t *testing.T) {
	tests := []struct {
		name      string
		lines     []string
		headLines int
		want      int
	}{
		{
			name:      "cuts before hostmetrics line",
			lines:     []string{"receivers:", "  otlp: {}", "  hostmetrics/10s:", "    scrapers: {}"},
			headLines: 20,
			want:      2,
		},
		{
			name:      "cuts before host_metrics line",
			lines:     []string{"receivers:", "  otlp: {}", "  host_metrics/10s:", "    scrapers: {}"},
			headLines: 20,
			want:      2,
		},
		{
			name:      "no hostmetrics falls back to headLines",
			lines:     []string{"receivers:", "  otlp: {}", "exporters:", "  otlp_http: {}"},
			headLines: 2,
			want:      2,
		},
		{
			name:      "lines shorter than headLines returns len(lines)",
			lines:     []string{"receivers:", "  otlp: {}"},
			headLines: 20,
			want:      2,
		},
		{
			name:      "hostmetrics at index 0 returns 0 (empty head)",
			lines:     []string{"  hostmetrics/10s:", "    scrapers: {}"},
			headLines: 20,
			want:      0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := configHeadEnd(tc.lines, tc.headLines); got != tc.want {
				t.Errorf("configHeadEnd() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestPipelinesSectionStart(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  int
	}{
		{
			name:  "finds pipelines at correct indent",
			lines: []string{"service:", "  telemetry: {}", "  pipelines:", "    traces: {}"},
			want:  2,
		},
		{
			name:  "returns -1 when not found",
			lines: []string{"service:", "  telemetry: {}"},
			want:  -1,
		},
		{
			name:  "ignores pipelines at wrong indent",
			lines: []string{"service:", "    pipelines:", "  pipelines:"},
			want:  2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pipelinesSectionStart(tc.lines); got != tc.want {
				t.Errorf("pipelinesSectionStart() = %d, want %d", got, tc.want)
			}
		})
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// buildPreviewConfig builds a synthetic config.yaml content with the given
// number of head lines, middle lines (between head and pipelines), and an
// optional "  pipelines:" section at the end.
func buildPreviewConfig(headLines, middleLines int, withPipelines bool) string {
	var b strings.Builder
	for i := range headLines {
		fmt.Fprintf(&b, "head_%02d: value\n", i)
	}
	for i := range middleLines {
		fmt.Fprintf(&b, "middle_%02d: value\n", i)
	}
	if withPipelines {
		b.WriteString("  pipelines:\n")
		b.WriteString("    traces:\n")
		b.WriteString("      receivers: [otlp]\n")
	}
	return b.String()
}

func TestPrintConfigPreview_Truncation(t *testing.T) {
	const sep = "────"

	tests := []struct {
		name          string
		headLines     int
		middleLines   int
		withPipelines bool
		wantEllipsis  bool
		wantMsg       string // substring expected in ellipsis line (if wantEllipsis)
	}{
		{
			name:          "short middle with pipelines — no truncation",
			headLines:     20,
			middleLines:   9, // 9 ≤ 30: show everything
			withPipelines: true,
			wantEllipsis:  false,
		},
		{
			name:          "long middle with pipelines — truncate, show pipelines after",
			headLines:     20,
			middleLines:   31, // 31 > 30: hide middle
			withPipelines: true,
			wantEllipsis:  true,
			wantMsg:       "31 lines",
		},
		{
			name:          "short tail without pipelines — no truncation",
			headLines:     20,
			middleLines:   9, // 9 ≤ 30: show everything
			withPipelines: false,
			wantEllipsis:  false,
		},
		{
			name:          "long tail without pipelines — truncate",
			headLines:     20,
			middleLines:   31, // 31 > 30: hide tail
			withPipelines: false,
			wantEllipsis:  true,
			wantMsg:       "31 more lines",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cp := &collectorPlan{
				configPath:    "/some/path/config.yaml",
				configPreview: buildPreviewConfig(tc.headLines, tc.middleLines, tc.withPipelines),
			}

			out := captureStdout(t, func() { cp.printConfigPreview(sep) })

			hasEllipsis := strings.Contains(out, "# ...")
			if hasEllipsis != tc.wantEllipsis {
				t.Errorf("ellipsis present = %v, want %v\noutput:\n%s", hasEllipsis, tc.wantEllipsis, out)
			}
			if tc.wantEllipsis && !strings.Contains(out, tc.wantMsg) {
				t.Errorf("ellipsis line missing %q\noutput:\n%s", tc.wantMsg, out)
			}
			// When no truncation, all middle/pipelines lines must appear.
			if !tc.wantEllipsis {
				for i := range tc.middleLines {
					marker := fmt.Sprintf("middle_%02d", i)
					if !strings.Contains(out, marker) {
						t.Errorf("expected %q in full output\noutput:\n%s", marker, out)
					}
				}
				if tc.withPipelines && !strings.Contains(out, "pipelines:") {
					t.Errorf("expected pipelines section in full output\noutput:\n%s", out)
				}
			}
			// When truncating with pipelines, the pipelines section must still appear.
			if tc.wantEllipsis && tc.withPipelines && !strings.Contains(out, "pipelines:") {
				t.Errorf("pipelines section missing after ellipsis\noutput:\n%s", out)
			}
		})
	}
}
