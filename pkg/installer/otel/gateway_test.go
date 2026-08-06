package otel

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func TestCountConfigFlags(t *testing.T) {
	tests := []struct {
		name string
		args string
		want int
	}{
		{"single --config", `otelcol --config /etc/otelcol/config.yaml`, 1},
		{"single -c short flag", `otelcol -c /etc/otelcol/config.yaml`, 1},
		{"equals form", `otelcol --config=/etc/otelcol/config.yaml`, 1},
		{"multiple --config flags", `otelcol --config base.yaml --config overrides.yaml`, 2},
		{"no config flag", `otelcol --feature-gates=foo`, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := countConfigFlags(tc.args); got != tc.want {
				t.Errorf("countConfigFlags(%q) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}

func TestLooksLikeInlineProvider(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{"env:MY_CONFIG", true},
		{"yaml:receivers: {}", true},
		{"/etc/otelcol/config.yaml", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := looksLikeInlineProvider(tc.val); got != tc.want {
			t.Errorf("looksLikeInlineProvider(%q) = %v, want %v", tc.val, got, tc.want)
		}
	}
}

func TestValidateConfigSource(t *testing.T) {
	t.Run("single writable file passes", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte("receivers: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		res := validateConfigSource(collectorInstance{configPath: path})
		if !res.OK {
			t.Errorf("expected OK, got failure: %s", res.Reason)
		}
	})

	t.Run("container with no host-mounted config fails", func(t *testing.T) {
		res := validateConfigSource(collectorInstance{
			containerRuntime:    "docker",
			containerName:       "otelcol",
			containerConfigPath: "/etc/otelcol/config.yaml",
			configPath:          "",
		})
		if res.OK {
			t.Error("expected failure for baked-in container config")
		}
	})

	t.Run("empty config path fails", func(t *testing.T) {
		res := validateConfigSource(collectorInstance{})
		if res.OK {
			t.Error("expected failure for empty config path")
		}
	})

	t.Run("inline env provider fails", func(t *testing.T) {
		res := validateConfigSource(collectorInstance{configPath: "env:MY_CONFIG"})
		if res.OK {
			t.Error("expected failure for env: inline provider")
		}
	})

	t.Run("inline yaml provider fails", func(t *testing.T) {
		res := validateConfigSource(collectorInstance{configPath: "yaml:receivers: {}"})
		if res.OK {
			t.Error("expected failure for yaml: inline provider")
		}
	})

	t.Run("nonexistent path fails", func(t *testing.T) {
		res := validateConfigSource(collectorInstance{configPath: filepath.Join(t.TempDir(), "missing.yaml")})
		if res.OK {
			t.Error("expected failure for nonexistent config path")
		}
	})

	t.Run("directory path fails", func(t *testing.T) {
		res := validateConfigSource(collectorInstance{configPath: t.TempDir()})
		if res.OK {
			t.Error("expected failure when config path is a directory")
		}
	})

	t.Run("unwritable file fails", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte("receivers: {}\n"), 0o400); err != nil {
			t.Fatal(err)
		}
		res := validateConfigSource(collectorInstance{configPath: path})
		if res.OK {
			t.Error("expected failure for unwritable config file")
		}
	})
}

func TestBuildGatewayExporterDef(t *testing.T) {
	def := buildGatewayExporterDef(4319)

	if def["endpoint"] != "localhost:4319" {
		t.Errorf("unexpected endpoint: %v", def["endpoint"])
	}
	tls, ok := def["tls"].(map[string]interface{})
	if !ok || tls["insecure"] != true {
		t.Errorf("expected tls.insecure=true, got %v", def["tls"])
	}
	queue, ok := def["sending_queue"].(map[string]interface{})
	if !ok || queue["block_on_overflow"] != false {
		t.Errorf("expected sending_queue.block_on_overflow=false, got %v", def["sending_queue"])
	}

	// Secret hygiene: the gateway exporter must never carry auth headers or a token.
	if _, hasHeaders := def["headers"]; hasHeaders {
		t.Error("gateway exporter must not include a headers/auth block")
	}
}

func TestMergeGatewayExporter_AdditiveOnly(t *testing.T) {
	cfg := map[string]interface{}{
		"receivers": map[string]interface{}{
			"otlp": map[string]interface{}{"protocols": map[string]interface{}{"grpc": nil}},
		},
		"exporters": map[string]interface{}{
			"otlphttp/grafana": map[string]interface{}{"endpoint": "https://grafana.example.com"},
		},
		"service": map[string]interface{}{
			"pipelines": map[string]interface{}{
				"traces": map[string]interface{}{
					"receivers": []interface{}{"otlp"},
					"exporters": []interface{}{"otlphttp/grafana"},
				},
				"metrics": map[string]interface{}{
					"receivers": []interface{}{"otlp"},
					"exporters": []interface{}{"otlphttp/grafana"},
				},
			},
		},
	}

	alreadyPresent := mergeGatewayExporter(cfg, 4319)
	if alreadyPresent {
		t.Fatal("expected first merge to report not-already-present")
	}

	// Original exporter untouched.
	exporters := cfg["exporters"].(map[string]interface{})
	grafana := exporters["otlphttp/grafana"].(map[string]interface{})
	if grafana["endpoint"] != "https://grafana.example.com" {
		t.Errorf("original exporter was modified: %v", grafana)
	}

	// New exporter added.
	if _, ok := exporters[gatewayExporterKey]; !ok {
		t.Fatal("gateway exporter was not added")
	}

	// Both pipelines got the new exporter appended, original entry preserved.
	pipelines := cfg["service"].(map[string]interface{})["pipelines"].(map[string]interface{})
	for _, name := range []string{"traces", "metrics"} {
		pipeline := pipelines[name].(map[string]interface{})
		exportersList := pipeline["exporters"].([]interface{})
		if len(exportersList) != 2 {
			t.Errorf("pipeline %s: expected 2 exporters, got %v", name, exportersList)
		}
		if exportersList[0] != "otlphttp/grafana" {
			t.Errorf("pipeline %s: original exporter no longer first: %v", name, exportersList)
		}
		if exportersList[1] != gatewayExporterKey {
			t.Errorf("pipeline %s: gateway exporter not appended: %v", name, exportersList)
		}
		// Receivers must be untouched.
		receivers := pipeline["receivers"].([]interface{})
		if len(receivers) != 1 || receivers[0] != "otlp" {
			t.Errorf("pipeline %s: receivers were modified: %v", name, receivers)
		}
	}
}

func TestMergeGatewayExporter_Idempotent(t *testing.T) {
	cfg := map[string]interface{}{
		"service": map[string]interface{}{
			"pipelines": map[string]interface{}{
				"traces": map[string]interface{}{
					"exporters": []interface{}{"otlphttp/grafana"},
				},
			},
		},
	}

	if alreadyPresent := mergeGatewayExporter(cfg, 4319); alreadyPresent {
		t.Fatal("expected first call to report not-already-present")
	}
	if alreadyPresent := mergeGatewayExporter(cfg, 4319); !alreadyPresent {
		t.Fatal("expected second call to report already-present")
	}

	pipelines := cfg["service"].(map[string]interface{})["pipelines"].(map[string]interface{})
	exportersList := pipelines["traces"].(map[string]interface{})["exporters"].([]interface{})
	count := 0
	for _, e := range exportersList {
		if e == gatewayExporterKey {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected gateway exporter to appear exactly once, got %d occurrences in %v", count, exportersList)
	}
}

func TestPatchForeignConfigForForwarding(t *testing.T) {
	original := []byte(`
receivers:
  otlp:
    protocols:
      grpc: {}
exporters:
  otlphttp/grafana:
    endpoint: https://grafana.example.com
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlphttp/grafana]
`)

	updated, alreadyPresent, err := patchForeignConfigForForwarding(original, 4319)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alreadyPresent {
		t.Fatal("expected first patch to report not-already-present")
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(updated, &cfg); err != nil {
		t.Fatalf("patched config is not valid YAML: %v", err)
	}
	exporters := cfg["exporters"].(map[string]interface{})
	if _, ok := exporters[gatewayExporterKey]; !ok {
		t.Fatal("patched config missing gateway exporter")
	}
	if _, ok := exporters["otlphttp/grafana"]; !ok {
		t.Fatal("patched config lost the original exporter")
	}

	// Re-patching the already-updated config is idempotent.
	_, alreadyPresent2, err := patchForeignConfigForForwarding(updated, 4319)
	if err != nil {
		t.Fatalf("unexpected error on re-patch: %v", err)
	}
	if !alreadyPresent2 {
		t.Error("expected re-patch of already-patched config to report already-present")
	}
}

func TestParseOtlpGRPCPort(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		data := []byte(`
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
`)
		port, err := parseOtlpGRPCPort(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if port != 4317 {
			t.Errorf("got port %d, want 4317", port)
		}
	})

	t.Run("missing otlp receiver", func(t *testing.T) {
		data := []byte(`receivers: {}`)
		if _, err := parseOtlpGRPCPort(data); err == nil {
			t.Error("expected error for config with no otlp receiver")
		}
	})
}

// TestUpdateNonDynatraceCollector_InvalidConfigSourceMakesNoChanges verifies
// the spec requirement that a failed config-source check makes zero changes:
// no backup file, no gateway install directory, and the original config is
// left byte-for-byte untouched.
func TestUpdateNonDynatraceCollector_InvalidConfigSourceMakesNoChanges(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	inst := collectorInstance{
		pid:        0,
		binaryPath: "/usr/bin/otelcol",
		configPath: "env:MY_CONFIG", // inline provider — fails config-source validation
	}

	helpers.CaptureStdout(t, func() {
		err := UpdateNonDynatraceCollector("https://env.live.dynatrace.com", "mytoken", "", inst, false)
		if err != nil {
			t.Errorf("expected nil error on invalid config source, got: %v", err)
		}
	})

	gatewayDir := filepath.Join(dir, "opentelemetry-gateway")
	if _, err := os.Stat(gatewayDir); !os.IsNotExist(err) {
		t.Errorf("expected no gateway install directory to be created, but found one at %s", gatewayDir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files created under HOME, found: %v", entries)
	}
}
