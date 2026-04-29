package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDtOTLPEndpoint(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.live.dynatrace.com", "https://example.live.dynatrace.com/api/v2/otlp"},
		{"https://example.live.dynatrace.com/", "https://example.live.dynatrace.com/api/v2/otlp"},
		{"https://example.live.dynatrace.com//", "https://example.live.dynatrace.com/api/v2/otlp"},
	}
	for _, tc := range tests {
		got := dtOTLPEndpoint(tc.input)
		if got != tc.want {
			t.Errorf("dtOTLPEndpoint(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGenerateExporterSnippet(t *testing.T) {
	snippet := GenerateExporterSnippet("https://env.live.dynatrace.com/", "mytoken")
	if !strings.Contains(snippet, "https://env.live.dynatrace.com/api/v2/otlp") {
		t.Errorf("snippet missing expected endpoint: %s", snippet)
	}
	if !strings.Contains(snippet, "mytoken") {
		t.Errorf("snippet missing token: %s", snippet)
	}
	// Ensure trailing slash on input URL is cleaned up — no double slash.
	if strings.Contains(snippet, "//api/v2/otlp") {
		t.Errorf("snippet contains double slash: %s", snippet)
	}
}

func TestMergeDynatraceExporter_EmptyConfig(t *testing.T) {
	cfg := map[string]interface{}{}
	mergeDynatraceExporter(cfg, "https://env.dt.com", "tok")

	exporters, ok := cfg["exporters"].(map[string]interface{})
	if !ok {
		t.Fatal("exporters key missing or wrong type")
	}
	dt, ok := exporters["otlp_http/dynatrace"].(map[string]interface{})
	if !ok {
		t.Fatal("otlp_http/dynatrace exporter missing")
	}
	if dt["endpoint"] != "https://env.dt.com/api/v2/otlp" {
		t.Errorf("unexpected endpoint: %v", dt["endpoint"])
	}
}

func TestMergeDynatraceExporter_AppendsToPipelines(t *testing.T) {
	cfg := map[string]interface{}{
		"service": map[string]interface{}{
			"pipelines": map[string]interface{}{
				"traces": map[string]interface{}{
					"exporters": []interface{}{"logging"},
				},
			},
		},
	}
	mergeDynatraceExporter(cfg, "https://env.dt.com", "tok")

	pipeline := cfg["service"].(map[string]interface{})["pipelines"].(map[string]interface{})["traces"].(map[string]interface{})
	exporters := pipeline["exporters"].([]interface{})
	found := false
	for _, e := range exporters {
		if e == "otlp_http/dynatrace" {
			found = true
		}
	}
	if !found {
		t.Errorf("otlp_http/dynatrace not appended to pipeline exporters: %v", exporters)
	}
	if len(exporters) != 2 {
		t.Errorf("expected 2 exporters, got %d: %v", len(exporters), exporters)
	}
}

func TestMergeDynatraceExporter_NoDuplicates(t *testing.T) {
	cfg := map[string]interface{}{
		"service": map[string]interface{}{
			"pipelines": map[string]interface{}{
				"traces": map[string]interface{}{
					"exporters": []interface{}{"otlp_http/dynatrace"},
				},
			},
		},
	}
	mergeDynatraceExporter(cfg, "https://env.dt.com", "tok")

	pipeline := cfg["service"].(map[string]interface{})["pipelines"].(map[string]interface{})["traces"].(map[string]interface{})
	exporters := pipeline["exporters"].([]interface{})
	if len(exporters) != 1 {
		t.Errorf("expected 1 exporter (no duplicate), got %d: %v", len(exporters), exporters)
	}
}

func TestMergeDynatraceExporter_NoServiceSection(t *testing.T) {
	cfg := map[string]interface{}{
		"receivers": map[string]interface{}{"otlp": nil},
	}
	// Should not panic when service section is absent.
	mergeDynatraceExporter(cfg, "https://env.dt.com", "tok")

	if _, ok := cfg["exporters"]; !ok {
		t.Error("exporters key should still be created even without service section")
	}
}

func TestDiffLines_BasicCases(t *testing.T) {
	old := []string{"a", "b", "c"}
	new := []string{"a", "x", "c"}
	edits := diffLines(old, new)

	counts := map[editKind]int{}
	for _, e := range edits {
		counts[e.kind]++
	}
	if counts[editKeep] != 2 {
		t.Errorf("expected 2 keep edits, got %d", counts[editKeep])
	}
	if counts[editDel] != 1 {
		t.Errorf("expected 1 delete edit, got %d", counts[editDel])
	}
	if counts[editAdd] != 1 {
		t.Errorf("expected 1 add edit, got %d", counts[editAdd])
	}
}

func TestDiffLines_EmptyInputs(t *testing.T) {
	edits := diffLines(nil, nil)
	if len(edits) != 0 {
		t.Errorf("expected no edits for empty inputs, got %d", len(edits))
	}
}

func TestPatchConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	initial := `receivers:
  otlp:
    protocols:
      grpc: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [logging]
`
	if err := os.WriteFile(configPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := PatchConfigFile(configPath, "https://env.dt.com/", "mytoken")
	if err != nil {
		t.Fatalf("PatchConfigFile error: %v", err)
	}

	if result.ConfigPath != configPath {
		t.Errorf("unexpected ConfigPath: %s", result.ConfigPath)
	}
	if !result.Modified {
		t.Error("Modified should be true")
	}

	// Updated file should contain the DT exporter.
	updatedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading updated config: %v", err)
	}
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(updatedData, &cfg); err != nil {
		t.Fatalf("parsing updated config: %v", err)
	}
	exporters, ok := cfg["exporters"].(map[string]interface{})
	if !ok {
		t.Fatal("exporters section missing from updated config")
	}
	if _, ok := exporters["otlp_http/dynatrace"]; !ok {
		t.Error("otlp_http/dynatrace exporter missing from updated config")
	}
}

func TestPatchConfigFile_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "empty.yaml")

	if err := os.WriteFile(configPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := PatchConfigFile(configPath, "https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("PatchConfigFile error: %v", err)
	}
	if !result.Modified {
		t.Error("Modified should be true")
	}
}

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	updated := []byte("original: true\nnew: field\n")

	if err := os.WriteFile(configPath, []byte("original: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := writeConfig(configPath, updated)
	if err != nil {
		t.Fatalf("writeConfig error: %v", err)
	}
	if !result.Modified {
		t.Error("Modified should be true")
	}

	configData, _ := os.ReadFile(configPath)
	if string(configData) != string(updated) {
		t.Error("config file does not match updated data")
	}
}
