# Design: Consolidate Host Monitoring Logs Pipeline

## Context

`pkg/installer/otel/otel.tmpl` is a Go text template that renders the OTel Collector config. The `otelConfigData` struct (in `pkg/installer/otel/collector.go`) drives the template. Two relevant fields:

- `HostMonitoring bool` — true when `featureflags.Experimental` is enabled
- `IncludeJournald bool` — true on Linux when `HostMonitoring` is true

The current template service block for logs:

```yaml
    logs:
      receivers: [otlp]
      processors: []
      exporters: [otlp_http]
{{- if and .HostMonitoring .IncludeJournald }}
    logs/host:
      receivers: [otlp, journald]
      processors: [resource_detection]
      exporters: [otlp_http]
{{- end }}
```

`resource_detection` is already defined as a processor in the template (under `processors:`) when `HostMonitoring` is true — it is used by `metrics/host`. No new processor definition is needed; only the pipeline reference changes.

## Decision: Single `logs` pipeline, conditional receivers and processors

Replace both pipelines with one:

```yaml
    logs:
      receivers: [otlp{{- if and .HostMonitoring .IncludeJournald }}, journald{{- end }}]
      processors: [{{- if .HostMonitoring }}resource_detection{{- end }}]
      exporters: [otlp_http]
```

This is the minimal change that fixes both problems:

- `resource_detection` is applied when `HostMonitoring` is true regardless of platform, so OTLP logs get host attributes on all platforms
- `journald` is added as a receiver on Linux only, exactly as before, eliminating the separate pipeline and the double-export of OTLP logs
- App-only mode (`HostMonitoring = false`) renders `processors: []` — unchanged from today

### Why not keep `logs/host` and fix `logs` separately?

Fixing the `logs` pipeline to add `resource_detection` while keeping `logs/host` would still double-export OTLP logs on Linux (both pipelines receive from `otlp`). Removing `otlp` from `logs/host` receivers would fix the duplication but leave a pipeline named `logs/host` that only collects journald — misleading and unnecessarily complex. The single-pipeline design is simpler, matches the reference config, and has no duplication.

### Journald guard remains unchanged

The `IncludeJournald` field continues to gate both the `journald` receiver definition and any pipeline reference to it. The field is set in `prepareCollectorPlan` in `collector.go` based on `runtime.GOOS == "linux"` and `HostMonitoring`. No changes to that logic are needed.

## Affected Files

| File | Change |
|---|---|
| `pkg/installer/otel/otel.tmpl` | Replace `logs` + `logs/host` blocks with single conditional `logs` pipeline |
| `pkg/installer/otel/collector_test.go` | Update assertions from `logs/host` to check `logs` pipeline processors and receivers |

## Test Changes

Three test functions reference `logs/host`:

- `TestGenerateOtelConfig_AppOnly_Default` — remove the now-dead `logs/host` absence check
- `TestGenerateOtelConfig_Combined_ExperimentalEnabled` — replace `logs/host` existence check with assertions that `logs` has `resource_detection` in processors and `journald` in receivers (Linux) or not (non-Linux)
- `TestGenerateOtelConfig_JournaldConsistency` — update to check journald reference in `logs` pipeline instead of `logs/host`

No new test functions are needed; the existing ones cover the behavior after adjustment.
