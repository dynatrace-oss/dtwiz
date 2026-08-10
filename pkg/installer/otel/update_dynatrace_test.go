package otel

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// parseRoot unmarshals YAML and returns the root mapping node.
func parseRoot(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		t.Fatalf("expected document node")
	}
	return doc.Content[0]
}

// roundtrip serialises root back to a YAML string for assertions.
func roundtrip(t *testing.T, root *yaml.Node) string {
	t.Helper()
	out, err := marshalNode(root)
	if err != nil {
		t.Fatalf("marshalNode: %v", err)
	}
	return string(out)
}

// ── isHostMonitoringPresent ────────────────────────────────────────────────

func TestIsHostMonitoringPresent_False_Empty(t *testing.T) {
	if isHostMonitoringPresent([]byte("receivers:\n  otlp: {}\n")) {
		t.Error("expected false for config without hostmetrics")
	}
}

func TestIsHostMonitoringPresent_True(t *testing.T) {
	cfg := `
receivers:
  hostmetrics/10s:
    collection_interval: 10s
`
	if !isHostMonitoringPresent([]byte(cfg)) {
		t.Error("expected true when hostmetrics receiver is present")
	}
}

func TestIsHostMonitoringPresent_False_InvalidYAML(t *testing.T) {
	if isHostMonitoringPresent([]byte("not: valid: yaml: [")) {
		t.Error("expected false for invalid YAML")
	}
}

// ── seqContains ──────────────────────────────────────────────────────────

func TestSeqContains(t *testing.T) {
	root := parseRoot(t, "seq: [a, b, c]\n")
	seq := nodeMappingGet(root, "seq")
	if !seqContains(seq, "b") {
		t.Error("expected true for present value")
	}
	if seqContains(seq, "z") {
		t.Error("expected false for absent value")
	}
	if seqContains(nil, "a") {
		t.Error("expected false for nil node")
	}
}

// ── ensureInExtensionsList ────────────────────────────────────────────────

func TestEnsureInExtensionsList_AddsWhenAbsent(t *testing.T) {
	root := parseRoot(t, "service: {}\n")
	svc := nodeMappingGet(root, "service")
	ensureInExtensionsList(svc, "health_check")

	out := roundtrip(t, root)
	if !strings.Contains(out, "health_check") {
		t.Errorf("expected health_check in output:\n%s", out)
	}
}

func TestEnsureInExtensionsList_NoDuplicate(t *testing.T) {
	root := parseRoot(t, "service:\n  extensions: [health_check]\n")
	svc := nodeMappingGet(root, "service")
	ensureInExtensionsList(svc, "health_check")

	seq := nodeMappingGet(svc, "extensions")
	count := 0
	for _, n := range seq.Content {
		if n.Value == "health_check" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 health_check entry, got %d", count)
	}
}

// ── matchesHostMonitoring ─────────────────────────────────────────────────

func TestMatchesHostMonitoring_EmptyCurrentReturnsFalse(t *testing.T) {
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	current := parseRoot(t, "receivers:\n  otlp: {}\n")
	if matchesHostMonitoring(current, ref) {
		t.Error("expected false when host monitoring is absent")
	}
}

func TestMatchesHostMonitoring_TrueAfterMerge(t *testing.T) {
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	current := parseRoot(t, "receivers:\n  otlp: {}\n")
	mergeHostMonitoringIntoConfig(current, ref)

	if !matchesHostMonitoring(current, ref) {
		t.Error("expected true after merge with reference")
	}
}

func TestMatchesHostMonitoring_FalseOnModifiedReceiver(t *testing.T) {
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	// Build a current config that has hostmetrics but with a wrong collection interval.
	current := parseRoot(t, `
receivers:
  host_metrics/10s:
    collection_interval: 99s
`)
	if matchesHostMonitoring(current, ref) {
		t.Error("expected false when receiver config differs from reference")
	}
}

// ── mergeHostMonitoringIntoConfig ────────────────────────────────────────

func TestMergeHostMonitoring_AddsHostmetricsReceivers(t *testing.T) {
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	current := parseRoot(t, "receivers:\n  otlp: {}\n")
	mergeHostMonitoringIntoConfig(current, ref)

	out := roundtrip(t, current)
	for _, key := range []string{"host_metrics/10s", "host_metrics/5m", "host_metrics/1h"} {
		if !strings.Contains(out, key+":") {
			t.Errorf("expected %q in output:\n%s", key, out)
		}
	}
}

func TestMergeHostMonitoring_AddsProcessors(t *testing.T) {
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	current := parseRoot(t, "processors:\n  cumulative_to_delta: {}\n")
	mergeHostMonitoringIntoConfig(current, ref)

	out := roundtrip(t, current)
	for _, key := range []string{"resource_detection", "transform", "filter/delete-metrics"} {
		if !strings.Contains(out, key+":") {
			t.Errorf("expected processor %q in output:\n%s", key, out)
		}
	}
}

func TestMergeHostMonitoring_AddsMetricsHostPipeline(t *testing.T) {
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	current := parseRoot(t, "service:\n  pipelines:\n    traces:\n      receivers: [otlp]\n")
	mergeHostMonitoringIntoConfig(current, ref)

	out := roundtrip(t, current)
	if !strings.Contains(out, "metrics/host:") {
		t.Errorf("expected metrics/host pipeline in output:\n%s", out)
	}
}

func TestMergeHostMonitoring_PreservesExistingHealthCheckPort(t *testing.T) {
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	current := parseRoot(t, `
extensions:
  health_check:
    endpoint: 0.0.0.0:19999
service:
  extensions: [health_check]
`)
	mergeHostMonitoringIntoConfig(current, ref)

	out := roundtrip(t, current)
	if !strings.Contains(out, "19999") {
		t.Errorf("expected existing health_check port 19999 to be preserved:\n%s", out)
	}
}

func TestMatchesHostMonitoring_TrueAfterRoundtripWithOldName(t *testing.T) {
	// Regression: after a successful update on a config that uses the legacy
	// "cumulativetodelta" name, a second run of update must see the config as
	// up to date, not trigger another update.
	//
	// Simulate: merge → serialize to disk → re-parse → compare with fresh ref.
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	current := parseRoot(t, `
processors:
  cumulativetodelta:
    max_staleness: 25h
service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [cumulativetodelta]
      exporters: [otlp_http]
`)
	mergeHostMonitoringIntoConfig(current, ref)

	// Simulate writing to disk and reading back (round-trip).
	diskBytes, err := marshalNode(current)
	if err != nil {
		t.Fatalf("marshalNode: %v", err)
	}
	reloaded := parseRoot(t, string(diskBytes))

	// Fresh reference — as if a second `dtwiz update otel` run just started.
	ref2, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef (ref2): %v", err)
	}

	if !matchesHostMonitoring(reloaded, ref2) {
		t.Error("expected matchesHostMonitoring to return true after round-trip: second run should see config as up to date")
	}
}

func TestMergeHostMonitoring_OldCumulativeNameAdapted(t *testing.T) {
	// Configs installed by older dtwiz versions use "cumulativetodelta" (no underscores).
	// The reference template uses "cumulative_to_delta". The metrics/host pipeline must
	// reference the name that is actually defined in the processors section.
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	current := parseRoot(t, `
processors:
  cumulativetodelta:
    max_staleness: 25h
service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [cumulativetodelta]
      exporters: [otlp_http]
`)
	mergeHostMonitoringIntoConfig(current, ref)

	out := roundtrip(t, current)

	// The metrics/host pipeline must reference the old name.
	if strings.Contains(out, "cumulative_to_delta") {
		t.Errorf("expected old name 'cumulativetodelta' to be used in metrics/host pipeline, got 'cumulative_to_delta':\n%s", out)
	}
	if !strings.Contains(out, "cumulativetodelta") {
		t.Errorf("expected 'cumulativetodelta' to remain in output:\n%s", out)
	}
}

func TestMergeHostMonitoring_PreservesExistingReceivers(t *testing.T) {
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}
	current := parseRoot(t, "receivers:\n  otlp: {}\n  my_custom: {}\n")
	mergeHostMonitoringIntoConfig(current, ref)

	out := roundtrip(t, current)
	if !strings.Contains(out, "my_custom:") {
		t.Errorf("expected custom receiver to be preserved:\n%s", out)
	}
	if !strings.Contains(out, "otlp:") {
		t.Errorf("expected otlp receiver to be preserved:\n%s", out)
	}
}

// ── updateOtlpExporter ──────────────────────────────────────────────────────

func TestUpdateOtlpExporter_ReturnsFalseWhenNoExporter(t *testing.T) {
	root := parseRoot(t, "receivers:\n  otlp: {}\n")
	if updateOtlpExporter(root, "https://env.dt.com", "tok") {
		t.Error("expected false when exporters section is absent")
	}
}

func TestUpdateOtlpExporter_ReturnsFalseWhenAlreadyCurrent(t *testing.T) {
	cfg := `
exporters:
  otlp_http:
    endpoint: https://env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer tok"
`
	root := parseRoot(t, cfg)
	if updateOtlpExporter(root, "https://env.dt.com", "tok") {
		t.Error("expected false when endpoint and auth are already correct")
	}
}

func TestUpdateOtlpExporter_UpdatesEndpoint(t *testing.T) {
	cfg := `
exporters:
  otlp_http:
    endpoint: https://old.env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer tok"
`
	root := parseRoot(t, cfg)
	changed := updateOtlpExporter(root, "https://new.env.dt.com", "tok")
	if !changed {
		t.Error("expected true when endpoint differs")
	}
	out := roundtrip(t, root)
	if !strings.Contains(out, "https://new.env.dt.com/api/v2/otlp") {
		t.Errorf("expected new endpoint in output:\n%s", out)
	}
}

func TestUpdateOtlpExporter_UpdatesAuthHeader(t *testing.T) {
	cfg := `
exporters:
  otlp_http:
    endpoint: https://env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer old-token"
`
	root := parseRoot(t, cfg)
	changed := updateOtlpExporter(root, "https://env.dt.com", "new-token")
	if !changed {
		t.Error("expected true when auth header differs")
	}
	out := roundtrip(t, root)
	if !strings.Contains(out, "Bearer new-token") {
		t.Errorf("expected new auth header in output:\n%s", out)
	}
}

func TestUpdateOtlpExporter_TrailingSlashStripped(t *testing.T) {
	cfg := `
exporters:
  otlp_http:
    endpoint: https://env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer tok"
`
	root := parseRoot(t, cfg)
	// URL with trailing slash should resolve to the same endpoint — no change.
	changed := updateOtlpExporter(root, "https://env.dt.com/", "tok")
	if changed {
		t.Error("expected false: trailing slash should be stripped before comparison")
	}
}

// ── isHostMonitoringPresent (canonical name) ──────────────────────────────

func TestIsHostMonitoringPresent_True_NewName(t *testing.T) {
	cfg := `
receivers:
  host_metrics/10s:
    collection_interval: 10s
`
	if !isHostMonitoringPresent([]byte(cfg)) {
		t.Error("expected true when host_metrics receiver is present")
	}
}

// ── needsDTExporterUpdate ─────────────────────────────────────────────────

func TestNeedsDTExporterUpdate_TrueWhenExportersAbsent(t *testing.T) {
	root := parseRoot(t, "receivers:\n  otlp: {}\n")
	if !needsDTExporterUpdate(root, "https://env.dt.com", "tok") {
		t.Error("expected true when exporters section is absent")
	}
}

func TestNeedsDTExporterUpdate_TrueWhenDTExporterAbsent(t *testing.T) {
	cfg := `
exporters:
  otlp_http:
    endpoint: https://env.dt.com/api/v2/otlp
service:
  pipelines:
    traces:
      exporters: [otlp_http]
`
	root := parseRoot(t, cfg)
	if !needsDTExporterUpdate(root, "https://env.dt.com", "tok") {
		t.Error("expected true when otlp_http/dynatrace is absent")
	}
}

func TestNeedsDTExporterUpdate_TrueWhenEndpointDiffers(t *testing.T) {
	cfg := `
exporters:
  otlp_http/dynatrace:
    endpoint: https://old.env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer tok"
service:
  pipelines:
    traces:
      exporters: [otlp_http/dynatrace]
`
	root := parseRoot(t, cfg)
	if !needsDTExporterUpdate(root, "https://new.env.dt.com", "tok") {
		t.Error("expected true when endpoint differs")
	}
}

func TestNeedsDTExporterUpdate_TrueWhenTokenDiffers(t *testing.T) {
	cfg := `
exporters:
  otlp_http/dynatrace:
    endpoint: https://env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer old-tok"
service:
  pipelines:
    traces:
      exporters: [otlp_http/dynatrace]
`
	root := parseRoot(t, cfg)
	if !needsDTExporterUpdate(root, "https://env.dt.com", "new-tok") {
		t.Error("expected true when token differs")
	}
}

func TestNeedsDTExporterUpdate_TrueWhenMissingFromPipeline(t *testing.T) {
	cfg := `
exporters:
  otlp_http/dynatrace:
    endpoint: https://env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer tok"
service:
  pipelines:
    traces:
      exporters: [otlp_http/dynatrace]
    metrics:
      exporters: [otlp_http]
`
	root := parseRoot(t, cfg)
	if !needsDTExporterUpdate(root, "https://env.dt.com", "tok") {
		t.Error("expected true when otlp_http/dynatrace is missing from a pipeline")
	}
}

func TestNeedsDTExporterUpdate_FalseWhenFullyCurrent(t *testing.T) {
	cfg := `
exporters:
  otlp_http/dynatrace:
    endpoint: https://env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer tok"
service:
  pipelines:
    traces:
      exporters: [otlp_http/dynatrace]
    metrics:
      exporters: [otlp_http/dynatrace]
`
	root := parseRoot(t, cfg)
	if needsDTExporterUpdate(root, "https://env.dt.com", "tok") {
		t.Error("expected false when exporter is current in all pipelines")
	}
}

// ── nodeMappingRename ─────────────────────────────────────────────────────

func TestNodeMappingRename_RenamesKey(t *testing.T) {
	root := parseRoot(t, "m:\n  old_name: 1\n")
	m := nodeMappingGet(root, "m")
	if !nodeMappingRename(m, "old_name", "new_name") {
		t.Error("expected true when old key exists")
	}
	out := roundtrip(t, root)
	if strings.Contains(out, "old_name:") {
		t.Errorf("expected old key removed:\n%s", out)
	}
	if !strings.Contains(out, "new_name:") {
		t.Errorf("expected new key present:\n%s", out)
	}
}

func TestNodeMappingRename_ReturnsFalseWhenAbsent(t *testing.T) {
	root := parseRoot(t, "m:\n  a: 1\n")
	m := nodeMappingGet(root, "m")
	if nodeMappingRename(m, "z", "w") {
		t.Error("expected false when old key is absent")
	}
}

// ── migrateDeprecatedAliases ──────────────────────────────────────────────

func TestMigrateDeprecatedAliases_NoOp(t *testing.T) {
	cfg := `
receivers:
  otlp: {}
processors:
  cumulative_to_delta: {}
`
	root := parseRoot(t, cfg)
	if migrateDeprecatedAliases(root) {
		t.Error("expected false when no deprecated aliases present")
	}
}

func TestMigrateDeprecatedAliases_RenamesHostMetrics(t *testing.T) {
	cfg := `
receivers:
  hostmetrics/10s:
    collection_interval: 10s
  hostmetrics/5m:
    collection_interval: 5m
service:
  pipelines:
    metrics/host:
      receivers: [hostmetrics/10s, hostmetrics/5m]
      exporters: [otlp_http]
`
	root := parseRoot(t, cfg)
	if !migrateDeprecatedAliases(root) {
		t.Error("expected true when deprecated aliases present")
	}
	out := roundtrip(t, root)
	if strings.Contains(out, "hostmetrics/") {
		t.Errorf("expected old 'hostmetrics' names to be removed:\n%s", out)
	}
	if !strings.Contains(out, "host_metrics/10s") || !strings.Contains(out, "host_metrics/5m") {
		t.Errorf("expected new 'host_metrics' names in output:\n%s", out)
	}
}

func TestMigrateDeprecatedAliases_RenamesCumulativeToDelta(t *testing.T) {
	cfg := `
processors:
  cumulativetodelta:
    max_staleness: 25h
service:
  pipelines:
    metrics:
      processors: [cumulativetodelta]
      exporters: [otlp_http]
`
	root := parseRoot(t, cfg)
	if !migrateDeprecatedAliases(root) {
		t.Error("expected true when deprecated processor alias present")
	}
	out := roundtrip(t, root)
	if strings.Contains(out, "cumulativetodelta") {
		t.Errorf("expected old name to be removed:\n%s", out)
	}
	if !strings.Contains(out, "cumulative_to_delta") {
		t.Errorf("expected new name in output:\n%s", out)
	}
}

// ── second-update idempotency ─────────────────────────────────────────────

// TestMatchesHostMonitoring_TrueWhenOnlyExportersDiffer verifies that a config
// where otlp_http/dynatrace has already been added to host monitoring pipelines
// is considered up to date — extra exporters must not trigger a re-merge.
func TestMatchesHostMonitoring_TrueWhenOnlyExportersDiffer(t *testing.T) {
	ref, err := renderHostMonitoringRef("https://env.dt.com", "tok")
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}

	// Start with a bare config, merge host monitoring (giving it the full reference
	// structure), then add otlp_http/dynatrace — simulating the state after a first
	// successful update.
	current := parseRoot(t, "receivers:\n  otlp: {}\n")
	mergeHostMonitoringIntoConfig(current, ref)
	mergeDynatraceExporter(current, "https://env.dt.com", "tok")

	// matchesHostMonitoring must return true: extra exporters are user-managed and
	// must not be treated as a host-monitoring mismatch.
	if !matchesHostMonitoring(current, ref) {
		t.Error("expected matchesHostMonitoring true: only exporters differ, not receivers/processors")
	}
}

// TestMergeHostMonitoring_DTExporterRestoredWhenReceiverStale verifies fix #1:
// when a genuine receiver change triggers mergeHostMonitoringIntoConfig, the
// pipeline node is replaced from the reference (which strips otlp_http/dynatrace),
// and the subsequent needsDTExporterUpdate re-check must detect and restore it.
func TestMergeHostMonitoring_DTExporterRestoredWhenReceiverStale(t *testing.T) {
	const apiURL = "https://env.dt.com"
	const token = "tok"

	ref, err := renderHostMonitoringRef(apiURL, token)
	if err != nil {
		t.Fatalf("renderHostMonitoringRef: %v", err)
	}

	// Config with a stale receiver interval AND otlp_http/dynatrace already wired.
	cfg := `
receivers:
  otlp: {}
  host_metrics/10s:
    collection_interval: 99s
exporters:
  otlp_http:
    endpoint: https://env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer tok"
  otlp_http/dynatrace:
    endpoint: https://env.dt.com/api/v2/otlp
    headers:
      Authorization: "Bearer tok"
service:
  extensions: [health_check]
  pipelines:
    traces:
      exporters: [otlp_http, otlp_http/dynatrace]
    metrics/host:
      receivers: [host_metrics/10s]
      exporters: [otlp_http, otlp_http/dynatrace]
`
	current := parseRoot(t, cfg)

	// Stale receiver must be detected.
	if matchesHostMonitoring(current, ref) {
		t.Fatal("expected matchesHostMonitoring false for stale receiver interval")
	}

	// Merge from reference: overwrites host_metrics/10s and the metrics/host pipeline.
	// The pipeline replacement strips otlp_http/dynatrace from metrics/host exporters.
	mergeHostMonitoringIntoConfig(current, ref)

	// Fix #1: re-check detects the exporter is now missing from metrics/host.
	if !needsDTExporterUpdate(current, apiURL, token) {
		t.Error("expected needsDTExporterUpdate true: merge stripped otlp_http/dynatrace from metrics/host")
	}

	mergeDynatraceExporter(current, apiURL, token)

	out := roundtrip(t, current)
	root2 := parseRoot(t, out)
	hostPipeline := nodeMappingGet(pipelinesNode(root2), "metrics/host")
	if hostPipeline == nil {
		t.Fatal("metrics/host pipeline missing after merge")
	}
	if !seqContains(nodeMappingGet(hostPipeline, "exporters"), "otlp_http/dynatrace") {
		t.Errorf("expected otlp_http/dynatrace in metrics/host exporters after re-merge:\n%s", out)
	}
}

func TestMigrateDeprecatedAliases_UpdatesBothReceiversAndPipelines(t *testing.T) {
	cfg := `
receivers:
  hostmetrics/10s:
    collection_interval: 10s
processors:
  cumulativetodelta: {}
service:
  pipelines:
    metrics/host:
      receivers: [hostmetrics/10s]
      processors: [cumulativetodelta]
      exporters: [otlp_http]
`
	root := parseRoot(t, cfg)
	migrateDeprecatedAliases(root)
	out := roundtrip(t, root)

	if strings.Contains(out, "hostmetrics") || strings.Contains(out, "cumulativetodelta") {
		t.Errorf("expected all deprecated aliases replaced:\n%s", out)
	}
	if !strings.Contains(out, "host_metrics/10s") || !strings.Contains(out, "cumulative_to_delta") {
		t.Errorf("expected canonical names in output:\n%s", out)
	}
}
