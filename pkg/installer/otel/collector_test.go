package otel

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

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

func TestOtelPlatformAssetName_CurrentPlatform(t *testing.T) {
	t.Parallel()

	got, err := otelPlatformAssetName("v1.2.3")
	if err != nil {
		t.Fatalf("otelPlatformAssetName() returned error: %v", err)
	}
	if !strings.Contains(got, "dynatrace-otel-collector_1.2.3_") {
		t.Fatalf("asset name = %q, want versioned collector asset", got)
	}
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(got, ".zip") {
			t.Fatalf("Windows asset = %q, want .zip", got)
		}
	} else if !strings.HasSuffix(got, ".tar.gz") {
		t.Fatalf("non-Windows asset = %q, want .tar.gz", got)
	}
}

func TestLooksLikeOtelCollector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want bool
	}{
		{name: "dynatrace-otel-collector", want: true},
		{name: "otelcorecol", want: true},
		{name: "otelcol-contrib", want: true},
		{name: "opentelemetry-collector", want: true},
		{name: "not-a-collector", want: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := looksLikeOtelCollector(tt.name); got != tt.want {
				t.Fatalf("looksLikeOtelCollector(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsDynatraceOtelCollector(t *testing.T) {
	t.Parallel()

	if !isDynatraceOtelCollector(filepath.Join("/opt", "dynatrace-otel-collector")) {
		t.Fatal("expected Dynatrace collector binary to match")
	}
	if isDynatraceOtelCollector(filepath.Join("/opt", "otelcol")) {
		t.Fatal("expected upstream collector binary not to match Dynatrace collector")
	}
}

func TestSelectCollector(t *testing.T) {
	instances := []collectorInstance{
		{pid: 111, binaryPath: "/usr/bin/otelcol", configPath: "/etc/otel/config.yaml"},
		{containerRuntime: "docker", containerName: "otel-container", containerConfigPath: "/etc/otelcol/config.yaml"},
	}
	setTestStdin(t, "2\n")

	selected, err := selectCollector(instances)
	if err != nil {
		t.Fatalf("selectCollector() returned error: %v", err)
	}
	if selected == nil || selected.containerName != "otel-container" {
		t.Fatalf("selected = %#v, want second collector", selected)
	}
}

func TestSelectCollector_NoInstances(t *testing.T) {
	selected, err := selectCollector(nil)
	if err != nil || selected != nil {
		t.Fatalf("selectCollector(nil) = (%#v, %v), want nil, nil", selected, err)
	}
}

func TestSelectCollector_InvalidInputCancels(t *testing.T) {
	setTestStdin(t, "not-a-number\n")

	selected, err := selectCollector([]collectorInstance{{pid: 111, binaryPath: "/usr/bin/otelcol"}})
	if selected != nil || err == nil {
		t.Fatalf("selectCollector() = (%#v, %v), want cancel error", selected, err)
	}
}

func TestSelectCollectorForUninstall_AutoConfirmSelectsAll(t *testing.T) {
	orig := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = orig })

	instances := []collectorInstance{{pid: 111}, {pid: 222}}
	selected, err := selectCollectorForUninstall(instances)
	if err != nil {
		t.Fatalf("selectCollectorForUninstall() returned error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected = %#v, want all collectors", selected)
	}
}

func TestSelectCollectorForUninstall_UserChoices(t *testing.T) {
	instances := []collectorInstance{{pid: 111}, {pid: 222}}

	tests := []struct {
		name      string
		input     string
		wantPIDs  []int
		wantError bool
	}{
		{name: "single collector", input: "2\n", wantPIDs: []int{222}},
		{name: "all collectors", input: "3\n", wantPIDs: []int{111, 222}},
		{name: "cancel", input: "0\n", wantError: true},
		{name: "invalid", input: "wat\n", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setTestStdin(t, tt.input)
			selected, err := selectCollectorForUninstall(instances)
			if tt.wantError {
				if err == nil {
					t.Fatalf("selectCollectorForUninstall() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("selectCollectorForUninstall() returned error: %v", err)
			}
			if len(selected) != len(tt.wantPIDs) {
				t.Fatalf("selected = %#v, want PIDs %v", selected, tt.wantPIDs)
			}
			for i, wantPID := range tt.wantPIDs {
				if selected[i].pid != wantPID {
					t.Fatalf("selected[%d].pid = %d, want %d", i, selected[i].pid, wantPID)
				}
			}
		})
	}
}

func TestOtelReleaseURL(t *testing.T) {
	t.Parallel()

	got := otelReleaseURL("v1.2.3", "collector.tar.gz")
	want := "https://github.com/Dynatrace/dynatrace-otel-collector/releases/download/v1.2.3/collector.tar.gz"
	if got != want {
		t.Fatalf("otelReleaseURL() = %q, want %q", got, want)
	}
}

func TestFindFreePort_ReturnsFreePort(t *testing.T) {
	port := findFreePort(8888)
	if port < 8888 {
		t.Fatalf("expected port >= 8888, got %d", port)
	}
	for _, host := range []string{"0.0.0.0", "localhost"} {
		l, err := net.Listen("tcp", host+":"+strconv.Itoa(port))
		if err != nil {
			t.Fatalf("port %d returned by findFreePort is not free on %s: %v", port, host, err)
		}
		l.Close()
	}
}

func TestFindFreePort_SkipsOccupiedWildcard(t *testing.T) {
	// Regression: probing only "localhost" missed conflicts on 0.0.0.0.
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

func TestFindFreePort_SkipsOccupiedLoopback(t *testing.T) {
	// Probe literal "localhost" because it may resolve to 127.0.0.1 or ::1.
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

	generatedConfig, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	cfg := generatedConfig.content
	if !strings.Contains(cfg, "port:") {
		t.Errorf("generated config missing metrics port:\n%s", cfg)
	}
	if !strings.Contains(cfg, "readers:") {
		t.Errorf("generated config missing readers section:\n%s", cfg)
	}
}

func TestGenerateOtelConfig_ReturnsRenderedHTTPPort(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")

	l, err := net.Listen("tcp", "0.0.0.0:4318")
	if err != nil {
		t.Skipf("cannot occupy 0.0.0.0:4318: %v", err)
	}
	defer l.Close()

	generatedConfig, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	renderedPort, portFound := extractOtlpHTTPPort([]byte(generatedConfig.content))
	if !portFound {
		t.Fatal("generated config missing OTLP HTTP port")
	}
	if generatedConfig.httpPort != renderedPort {
		t.Fatalf("returned httpPort = %d, rendered port = %d", generatedConfig.httpPort, renderedPort)
	}
	if generatedConfig.httpPort == 4318 {
		t.Fatal("expected generated config to avoid occupied default OTLP HTTP port")
	}
}

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

func TestGenerateOtelConfig_AppOnly_Default(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")

	generatedConfig, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	cfg := generatedConfig.content
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
	if _, ok := parsed.Service.Pipelines["metrics"]; !ok {
		t.Error("app-only config must contain metrics pipeline")
	}
	tracesPipeline, tracesOk := parsed.Service.Pipelines["traces"]
	if !tracesOk {
		t.Fatal("app-only config missing traces pipeline")
	}
	for _, p := range tracesPipeline.Processors {
		if p == "resource_detection/system" {
			t.Errorf("app-only config traces pipeline must not contain resource_detection processor, got %v", tracesPipeline.Processors)
			break
		}
	}
}

func TestGenerateOtelConfig_Combined_ExperimentalEnabled(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	generatedConfig, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	cfg := generatedConfig.content
	parsed := parseOtelConfig(t, cfg)

	for _, recv := range []string{"host_metrics/10s", "host_metrics/5m", "host_metrics/1h"} {
		if _, ok := parsed.Receivers[recv]; !ok {
			t.Errorf("combined config missing receiver %q", recv)
		}
	}

	if _, ok := parsed.Extensions["health_check"]; !ok {
		t.Error("combined config missing health_check extension")
	}

	hostPipeline, ok := parsed.Service.Pipelines["metrics/host"]
	if !ok {
		t.Fatal("combined config missing metrics/host pipeline")
	}
	wantProcessors := []string{"filter/idle-cpu-usage", "resource/add-host-group-id", "resource_detection/system", "transform", "filter/delete-metrics", "cumulative_to_delta"}
	if len(hostPipeline.Processors) != len(wantProcessors) {
		t.Errorf("metrics/host processors: got %v, want %v", hostPipeline.Processors, wantProcessors)
	} else {
		for i, p := range wantProcessors {
			if hostPipeline.Processors[i] != p {
				t.Errorf("metrics/host processors[%d]: got %q, want %q", i, hostPipeline.Processors[i], p)
			}
		}
	}

	logsPipeline, logsOk := parsed.Service.Pipelines["logs"]
	if !logsOk {
		t.Fatal("combined config missing logs pipeline")
	}
	hasResourceDetection := false
	for _, p := range logsPipeline.Processors {
		if p == "resource_detection/system" {
			hasResourceDetection = true
			break
		}
	}
	if !hasResourceDetection {
		t.Errorf("combined config logs pipeline missing resource_detection processor, got %v", logsPipeline.Processors)
	}
	hasJournald := false
	for _, r := range logsPipeline.Receivers {
		if r == "journald" {
			hasJournald = true
			break
		}
	}
	if runtime.GOOS == "linux" {
		if !hasJournald {
			t.Error("combined config logs pipeline missing journald receiver on Linux")
		}
	} else {
		if hasJournald {
			t.Error("combined config logs pipeline must not contain journald receiver on non-Linux")
		}
	}

	if _, ok := parsed.Service.Pipelines["metrics/apps"]; !ok {
		t.Error("combined config missing metrics/apps pipeline")
	}
	tracesPipeline, tracesOk := parsed.Service.Pipelines["traces"]
	if !tracesOk {
		t.Fatal("combined config missing traces pipeline")
	}
	hasResourceDetectionInTraces := false
	for _, p := range tracesPipeline.Processors {
		if p == "resource_detection/system" {
			hasResourceDetectionInTraces = true
			break
		}
	}
	if !hasResourceDetectionInTraces {
		t.Errorf("combined config traces pipeline missing resource_detection processor, got %v", tracesPipeline.Processors)
	}
	if _, ok := parsed.Service.Pipelines["logs"]; !ok {
		t.Error("combined config missing logs pipeline")
	}
}

func TestGenerateOtelConfig_Combined_EnvVar(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "true")

	generatedConfig, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	cfg := generatedConfig.content
	parsed := parseOtelConfig(t, cfg)

	if _, ok := parsed.Service.Pipelines["metrics/host"]; !ok {
		t.Error("expected metrics/host pipeline when DTWIZ_EXPERIMENTAL=true")
	}
}

func TestGenerateOtelConfig_JournaldConsistency(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	generatedConfig, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	cfg := generatedConfig.content
	parsed := parseOtelConfig(t, cfg)

	_, receiverDefined := parsed.Receivers["journald"]
	logsPipeline, logsExists := parsed.Service.Pipelines["logs"]
	var pipelineReferences bool
	if logsExists {
		for _, r := range logsPipeline.Receivers {
			if r == "journald" {
				pipelineReferences = true
				break
			}
		}
	}

	if receiverDefined != pipelineReferences {
		t.Errorf("journald receiver defined=%v but logs pipeline reference=%v — guards must stay in sync",
			receiverDefined, pipelineReferences)
	}
}

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
			generatedConfig, err := generateOtelConfig("https://env.example.com", "mytoken")
			if err != nil {
				t.Fatalf("generateOtelConfig: %v", err)
			}
			cfg := generatedConfig.content
			var parsed any
			if err := yaml.Unmarshal([]byte(cfg), &parsed); err != nil {
				t.Errorf("rendered config is not valid YAML: %v\n---\n%s", err, cfg)
			}
		})
	}
}

func TestGenerateOtelConfig_PreviewTruncation(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	generatedConfig, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	cfg := generatedConfig.content
	lines := strings.Split(strings.TrimRight(cfg, "\n"), "\n")
	if len(lines) <= 20 {
		t.Errorf("experimental config has %d lines, expected more than 20 so truncation is exercised", len(lines))
	}
}

func TestGenerateOtelConfig_TokenMaskedInPreview(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")

	const token = "dt0s16.supersecrettoken"
	generatedConfig, err := generateOtelConfig("https://env.example.com", token)
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	cfg := generatedConfig.content

	preview := installer.MaskSecret(cfg, token)
	if strings.Contains(preview, token) {
		t.Error("token must be masked in the config preview")
	}
	if !strings.Contains(preview, "***") {
		t.Error("masked preview must contain '***' placeholder")
	}
}

func TestGenerateOtelConfig_HostGroupID_ProcessorPresent(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	parsed := parseOtelConfig(t, cfg.content)

	proc, ok := parsed.Processors["resource/add-host-group-id"]
	if !ok {
		t.Fatal("host monitoring config missing resource/add-host-group-id processor")
	}

	hostname, _ := os.Hostname()
	procStr := fmt.Sprintf("%v", proc)
	if !strings.Contains(procStr, hostname) {
		t.Errorf("resource/add-host-group-id processor does not contain hostname %q: %v", hostname, proc)
	}
}

func TestGenerateOtelConfig_HostGroupID_AppOnly_Absent(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")

	cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	parsed := parseOtelConfig(t, cfg.content)

	if _, ok := parsed.Processors["resource/add-host-group-id"]; ok {
		t.Error("standard-mode config must not contain resource/add-host-group-id processor")
	}
	for _, name := range []string{"traces", "metrics", "logs"} {
		pipeline, ok := parsed.Service.Pipelines[name]
		if !ok {
			continue
		}
		for _, p := range pipeline.Processors {
			if p == "resource/add-host-group-id" {
				t.Errorf("pipeline %q must not reference resource/add-host-group-id in standard mode", name)
			}
		}
	}
}

func TestGenerateOtelConfig_HostGroupID_HostMonitoring_Pipelines(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	cfg, err := generateOtelConfig("https://env.example.com", "mytoken")
	if err != nil {
		t.Fatalf("generateOtelConfig: %v", err)
	}
	parsed := parseOtelConfig(t, cfg.content)

	pipelines := []string{"traces", "metrics/apps", "metrics/host", "logs"}
	for _, name := range pipelines {
		pipeline, ok := parsed.Service.Pipelines[name]
		if !ok {
			t.Errorf("pipeline %q not found", name)
			continue
		}
		found := false
		for _, p := range pipeline.Processors {
			if p == "resource/add-host-group-id" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("pipeline %q missing resource/add-host-group-id processor: %v", name, pipeline.Processors)
		}
	}
}

func TestGenerateOtelConfig_HostGroupID_EmptyHostname(t *testing.T) {
	featureflags.ClearCLIOverrideForTest(t, featureflags.Experimental)
	t.Setenv("DTWIZ_EXPERIMENTAL", "")

	// Render the template directly with an empty HostGroupID to simulate hostname failure.
	tmpl, err := template.New("otel").Parse(otelConfigTemplateText)
	if err != nil {
		t.Fatalf("parsing template: %v", err)
	}
	data := otelConfigData{
		Endpoint:    "https://env.example.com",
		AuthHeader:  "Bearer mytoken",
		HostGroupID: "",
		MetricsPort: 8888,
		GRPCPort:    4317,
		HTTPPort:    4318,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("template execution failed with empty HostGroupID: %v", err)
	}
	var parsed any
	if err := yaml.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("rendered config with empty HostGroupID is not valid YAML: %v", err)
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

func TestWarnIfCollectorUnreachable_NoListener(t *testing.T) {
	// Pick a port that nothing is listening on by binding and immediately closing.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	out := captureStdout(t, func() { warnIfCollectorUnreachable(endpoint) })
	if !strings.Contains(out, "Warning") {
		t.Errorf("expected warning in output when collector not reachable, got: %q", out)
	}
	if !strings.Contains(out, fmt.Sprintf("127.0.0.1:%d", port)) {
		t.Errorf("expected port in warning output, got: %q", out)
	}
}

func TestWarnIfCollectorUnreachable_Listening(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	out := captureStdout(t, func() { warnIfCollectorUnreachable(endpoint) })
	if strings.Contains(out, "Warning") {
		t.Errorf("expected no warning when collector is reachable, got: %q", out)
	}
}

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

func TestFormatPIDs(t *testing.T) {
	t.Parallel()

	got := formatPIDs([]runningCollector{{pid: 123}, {pid: 456}, {pid: 789}})
	if got != "123, 456, 789" {
		t.Fatalf("formatPIDs() = %q, want comma-separated PIDs", got)
	}
}

func TestCollectorPlanPrintDryRun(t *testing.T) {
	cp := &collectorPlan{
		installDir:    "/tmp/opentelemetry",
		binaryPath:    "/tmp/opentelemetry/dynatrace-otel-collector",
		configPath:    "/tmp/opentelemetry/config.yaml",
		configPreview: "receivers:\n  otlp:\nexporters:\n  otlphttp:\n    headers:\n      Authorization: Api-Token ***\n",
	}

	out := captureStdout(t, cp.printDryRun)
	for _, want := range []string{
		"[dry-run] Would install Dynatrace OpenTelemetry Collector",
		"Install dir:  /tmp/opentelemetry",
		"Binary:       /tmp/opentelemetry/dynatrace-otel-collector",
		"Config:       /tmp/opentelemetry/config.yaml",
		"Ingest token: (configured)",
		"Authorization: Api-Token ***",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("printDryRun() output missing %q:\n%s", want, out)
		}
	}
}

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
			if tc.wantEllipsis && tc.withPipelines && !strings.Contains(out, "pipelines:") {
				t.Errorf("pipelines section missing after ellipsis\noutput:\n%s", out)
			}
		})
	}
}

func TestExtractOtlpHTTPPort(t *testing.T) {
	tests := []struct {
		name     string
		yamlDoc  string
		wantPort int
		wantOK   bool
	}{
		{
			name: "standard IPv4 endpoint",
			yamlDoc: `
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4320
`,
			wantPort: 4320,
			wantOK:   true,
		},
		{
			name: "bracketed IPv6 endpoint",
			yamlDoc: `
receivers:
  otlp:
    protocols:
      http:
        endpoint: "[::]:4318"
`,
			wantPort: 4318,
			wantOK:   true,
		},
		{
			name: "localhost endpoint",
			yamlDoc: `
receivers:
  otlp:
    protocols:
      http:
        endpoint: localhost:9999
`,
			wantPort: 9999,
			wantOK:   true,
		},
		{
			name: "http protocol absent (grpc only)",
			yamlDoc: `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
`,
			wantOK: false,
		},
		{
			name: "endpoint has no port",
			yamlDoc: `
receivers:
  otlp:
    protocols:
      http:
        endpoint: "0.0.0.0"
`,
			wantOK: false,
		},
		{
			name: "non-numeric port",
			yamlDoc: `
receivers:
  otlp:
    protocols:
      http:
        endpoint: "0.0.0.0:notaport"
`,
			wantOK: false,
		},
		{
			name:    "empty document",
			yamlDoc: ``,
			wantOK:  false,
		},
		{
			name:    "invalid YAML",
			yamlDoc: "not: [valid: yaml",
			wantOK:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			port, ok := extractOtlpHTTPPort([]byte(tc.yamlDoc))
			if ok != tc.wantOK {
				t.Fatalf("extractOtlpHTTPPort() ok = %v, want %v", ok, tc.wantOK)
			}
			if tc.wantOK && port != tc.wantPort {
				t.Errorf("extractOtlpHTTPPort() port = %d, want %d", port, tc.wantPort)
			}
		})
	}
}

func TestWaitForOtelCollectorReady_ReturnsImmediatelyWhenPortIsOpen(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	if err := waitForOtelCollectorReady(port, 2*time.Second, make(chan error)); err != nil {
		t.Fatalf("waitForOtelCollectorReady() error = %v, want nil", err)
	}
}

func TestWaitForOtelCollectorReady_TimesOutOnActualPortProbed(t *testing.T) {
	port := findFreePort(45000)

	start := time.Now()
	err := waitForOtelCollectorReady(port, time.Second, make(chan error))
	if err == nil {
		t.Fatal("expected an error when nothing ever opens the port")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(port)) {
		t.Errorf("expected error %q to mention the probed port (%d)", err, port)
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Errorf("returned before the timeout elapsed: %s", elapsed)
	}
}

func TestWaitForOtelCollectorReady_AbortsImmediatelyOnCrash(t *testing.T) {
	port := findFreePort(45100)

	crashed := make(chan error, 1)
	crashed <- fmt.Errorf("boom")

	start := time.Now()
	err := waitForOtelCollectorReady(port, 30*time.Second, crashed)
	if err == nil {
		t.Fatal("expected an error when the collector process has crashed")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q does not wrap the crash error", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("expected to abort quickly on crash instead of waiting out the timeout, took %s", elapsed)
	}
}

func TestSendOtelVerificationLog_PostsToGivenPort(t *testing.T) {
	var gotPath string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	if err := sendOtelVerificationLog(port, "hello from test"); err != nil {
		t.Fatalf("sendOtelVerificationLog() error = %v", err)
	}
	if gotPath != "/v1/logs" {
		t.Errorf("request path = %q, want /v1/logs", gotPath)
	}
	if !bytes.Contains(gotBody, []byte("hello from test")) {
		t.Errorf("request body does not contain the verification text:\n%s", gotBody)
	}
}

func TestSendOtelVerificationLog_NonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	err := sendOtelVerificationLog(port, "body")
	if err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q does not mention the response status code", err)
	}
}

func TestSendOtelVerificationLog_GivesUpAfterRetriesOnRefusedPort(t *testing.T) {
	port := findFreePort(45200)

	err := sendOtelVerificationLog(port, "body")
	if err == nil {
		t.Fatal("expected an error when nothing is listening on the port")
	}
	if !strings.Contains(err.Error(), "not ready after") {
		t.Errorf("error %q does not mention retry exhaustion", err)
	}
}

func TestVerifyOtelInstall_UsesGivenHTTPPort(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	origWait := waitForLogInDynatraceFn
	waitForLogInDynatraceFn = func(_, _, _ string, _ time.Duration) error { return nil }
	t.Cleanup(func() { waitForLogInDynatraceFn = origWait })

	var err error
	captureStdout(t, func() {
		err = verifyOtelInstall("https://env.example.com", "platform-token", "", port, make(chan error))
	})
	if err != nil {
		t.Fatalf("verifyOtelInstall() error = %v", err)
	}
	if !hit {
		t.Error("expected verifyOtelInstall to send the verification log to the given httpPort")
	}
}
