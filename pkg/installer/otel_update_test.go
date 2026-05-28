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

// mustPipelineExporters returns the exporters slice for a named pipeline,
// failing the test if the path or type assertion does not hold.
func mustPipelineExporters(t *testing.T, cfg map[string]interface{}, pipelineName string) []interface{} {
	t.Helper()
	svc, ok := cfg["service"].(map[string]interface{})
	if !ok {
		t.Fatalf("cfg[\"service\"] missing or wrong type")
	}
	pipelines, ok := svc["pipelines"].(map[string]interface{})
	if !ok {
		t.Fatalf("cfg[\"service\"][\"pipelines\"] missing or wrong type")
	}
	pipeline, ok := pipelines[pipelineName].(map[string]interface{})
	if !ok {
		t.Fatalf("pipeline %q not found or wrong type", pipelineName)
	}
	exporters, ok := pipeline["exporters"].([]interface{})
	if !ok {
		t.Fatalf("pipeline %q has no exporters slice", pipelineName)
	}
	return exporters
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

	exporters := mustPipelineExporters(t, cfg, "traces")
	if len(exporters) != 2 {
		t.Fatalf("expected 2 exporters, got %d: %v", len(exporters), exporters)
	}
	if exporters[1] != "otlp_http/dynatrace" {
		t.Errorf("expected otlp_http/dynatrace appended last, got %v", exporters[1])
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

	if exporters := mustPipelineExporters(t, cfg, "traces"); len(exporters) != 1 {
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
	oldLines := []string{"a", "b", "c"}
	newLines := []string{"a", "x", "c"}

	want := []diffEdit{
		{editKeep, "a"},
		{editDel, "b"},
		{editAdd, "x"},
		{editKeep, "c"},
	}
	got := diffLines(oldLines, newLines)
	if len(got) != len(want) {
		t.Fatalf("diffLines returned %d edits, want %d: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("edit[%d] = %+v, want %+v", i, got[i], w)
		}
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
	if result.BackupPath == "" {
		t.Error("expected non-empty BackupPath")
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Errorf("backup file not found: %v", err)
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

func TestWriteConfig_Error(t *testing.T) {
	_, err := writeConfig("/nonexistent/dir/config.yaml", []byte("data"))
	if err == nil {
		t.Error("expected error writing to non-existent directory")
	}
}

func TestBackupFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	data := []byte("receivers:\n  otlp: {}\n")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	backupPath, err := backupFile(configPath, data)
	if err != nil {
		t.Fatalf("backupFile error: %v", err)
	}
	if backupPath == "" {
		t.Error("expected non-empty backup path")
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup file not found at %s: %v", backupPath, err)
	}
}

func TestBackupFile_Error(t *testing.T) {
	_, err := backupFile("/nonexistent/dir/config.yaml", []byte("data"))
	if err == nil {
		t.Error("expected error when backup directory does not exist")
	}
}

func TestGeneratePipelineHint(t *testing.T) {
	hint := GeneratePipelineHint()
	if !strings.Contains(hint, "otlp_http/dynatrace") {
		t.Errorf("pipeline hint missing exporter name: %s", hint)
	}
	if !strings.Contains(hint, "exporters") {
		t.Errorf("pipeline hint missing 'exporters': %s", hint)
	}
}

func TestGenerateFullInstructions(t *testing.T) {
	instructions := GenerateFullInstructions("https://env.live.dynatrace.com/", "mytoken")
	if !strings.Contains(instructions, "exporters:") {
		t.Errorf("instructions missing exporters section header: %s", instructions)
	}
	if !strings.Contains(instructions, "otlp_http/dynatrace") {
		t.Errorf("instructions missing exporter name: %s", instructions)
	}
	if !strings.Contains(instructions, "mytoken") {
		t.Errorf("instructions missing token: %s", instructions)
	}
	if !strings.Contains(instructions, "https://env.live.dynatrace.com/api/v2/otlp") {
		t.Errorf("instructions missing endpoint URL: %s", instructions)
	}
}

func TestShowConfigDiff_NoChange(t *testing.T) {
	data := []byte("receivers:\n  otlp: {}\n")
	// Should not panic when inputs are identical.
	showConfigDiff(data, data)
}

func TestShowConfigDiff_WithChanges(t *testing.T) {
	orig := []byte("receivers:\n  otlp: {}\n")
	updated := []byte("receivers:\n  otlp: {}\nexporters:\n  logging: {}\n")
	// Should not panic.
	showConfigDiff(orig, updated)
}

func TestMergeDynatraceExporter_InvalidPipelineValue(t *testing.T) {
	// Pipeline value is a non-map type — should be skipped without panic.
	cfg := map[string]interface{}{
		"service": map[string]interface{}{
			"pipelines": map[string]interface{}{
				"traces": "not-a-map",
			},
		},
	}
	mergeDynatraceExporter(cfg, "https://env.dt.com", "tok")

	exporters, ok := cfg["exporters"].(map[string]interface{})
	if !ok {
		t.Fatal("exporters key missing")
	}
	if _, ok := exporters["otlp_http/dynatrace"]; !ok {
		t.Error("otlp_http/dynatrace exporter missing")
	}
}

func TestMergeDynatraceExporter_NoPipelinesKey(t *testing.T) {
	cfg := map[string]interface{}{
		"service": map[string]interface{}{
			"extensions": []interface{}{"health_check"},
		},
	}
	// Should not panic when service has no pipelines key.
	mergeDynatraceExporter(cfg, "https://env.dt.com", "tok")

	if _, ok := cfg["exporters"]; !ok {
		t.Error("exporters key should be created even without pipelines")
	}
}

func TestMergeDynatraceExporter_PipelineWithNoExporters(t *testing.T) {
	// Pipeline exists but has no exporters list yet.
	cfg := map[string]interface{}{
		"service": map[string]interface{}{
			"pipelines": map[string]interface{}{
				"traces": map[string]interface{}{
					"receivers": []interface{}{"otlp"},
				},
			},
		},
	}
	mergeDynatraceExporter(cfg, "https://env.dt.com", "tok")

	exporters := mustPipelineExporters(t, cfg, "traces")
	if len(exporters) != 1 || exporters[0] != "otlp_http/dynatrace" {
		t.Errorf("unexpected exporters: %v", exporters)
	}
}

func TestPatchConfigFile_NotFound(t *testing.T) {
	_, err := PatchConfigFile("/nonexistent/dir/config.yaml", "https://env.dt.com", "tok")
	if err == nil {
		t.Error("expected error for non-existent config file")
	}
}

func TestPatchConfigFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(configPath, []byte(": : invalid: yaml: [[["), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := PatchConfigFile(configPath, "https://env.dt.com", "tok")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestUpdateOtelConfig_DryRun(t *testing.T) {
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

	err := UpdateOtelConfig(configPath, "https://env.live.dynatrace.com", "mytoken", "", true)
	if err != nil {
		t.Fatalf("UpdateOtelConfig dry-run error: %v", err)
	}

	// In dry-run mode the config must not be modified.
	data, _ := os.ReadFile(configPath)
	if string(data) != initial {
		t.Error("config file was modified during dry-run")
	}
}

func TestFindExistingCollectorConfig_HomeWins(t *testing.T) {
	homeDir := t.TempDir()
	cwdDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatal(err)
	}
	// Registered after t.TempDir() so it runs first (LIFO): restores cwd before
	// the temp dirs are removed, avoiding a Windows file-lock on the cwd.
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Create config in both locations.
	homeConfig := filepath.Join(homeDir, "opentelemetry", "config.yaml")
	cwdConfig := filepath.Join(cwdDir, "opentelemetry", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(homeConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cwdConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(homeConfig, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cwdConfig, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := findExistingCollectorConfig()
	if got != homeConfig {
		t.Errorf("expected home config %q, got %q", homeConfig, got)
	}
}

func TestFindExistingCollectorConfig_FallsBackToCWD(t *testing.T) {
	homeDir := t.TempDir()
	cwdDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatal(err)
	}
	// Registered after t.TempDir() so it runs first (LIFO): restores cwd before
	// the temp dirs are removed, avoiding a Windows file-lock on the cwd.
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Only create config in CWD — home path absent.
	cwdConfig := filepath.Join(cwdDir, "opentelemetry", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cwdConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cwdConfig, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Build the expected path from os.Getwd() so it matches exactly what
	// findExistingCollectorConfig() will see — on Windows, os.Getwd() can
	// return the 8.3 short-path form (RUNNER~1) while t.TempDir() returns
	// the long form; using os.Getwd() keeps both sides consistent.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wantConfig := filepath.Join(cwd, "opentelemetry", "config.yaml")

	got := findExistingCollectorConfig()
	if got != wantConfig {
		t.Errorf("expected cwd config %q, got %q", wantConfig, got)
	}
}

func TestFindExistingCollectorConfig_NoneFound(t *testing.T) {
	homeDir := t.TempDir()
	cwdDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatal(err)
	}
	// Registered after t.TempDir() so it runs first (LIFO): restores cwd before
	// the temp dirs are removed, avoiding a Windows file-lock on the cwd.
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	got := findExistingCollectorConfig()
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
