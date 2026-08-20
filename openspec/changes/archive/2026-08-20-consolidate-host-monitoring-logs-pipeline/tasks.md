# Tasks: Consolidate Host Monitoring Logs Pipeline

## 1. Update `otel.tmpl`

- [x] In `pkg/installer/otel/otel.tmpl`, replace the `logs` + `logs/host` service pipeline block with a single `logs` pipeline:
  - `receivers`: `[otlp]` by default; append `, journald` when `{{- if and .HostMonitoring .IncludeJournald }}`
  - `processors`: empty (`[]`) by default; set to `[resource_detection]` when `{{- if .HostMonitoring }}`
  - `exporters`: `[otlp_http]` (unchanged)
- [x] Remove the `{{- if and .HostMonitoring .IncludeJournald }} logs/host: ... {{- end }}` block entirely

## 2. Update tests in `collector_test.go`

- [x] `TestGenerateOtelConfig_AppOnly_Default` — remove the stale check `if _, ok := parsed.Service.Pipelines["logs/host"]; ok { t.Error(...) }`
- [x] `TestGenerateOtelConfig_Combined_ExperimentalEnabled` — replace the `logs/host` existence assertions with:
  - Assert `logs` pipeline exists
  - Assert `resource_detection` is present in `logs` pipeline processors
  - On `runtime.GOOS == "linux"`: assert `journald` is in `logs` pipeline receivers
  - On other platforms: assert `journald` is NOT in `logs` pipeline receivers
- [x] `TestGenerateOtelConfig_JournaldConsistency` — update to check the `logs` pipeline (not `logs/host`) for the journald receiver reference

## 3. Update the spec

- [x] In `openspec/specs/otel-host-monitoring/spec.md`, update the "Platform-aware host signal selection" requirement and its scenarios to describe the consolidated `logs` pipeline behavior:
  - Replace the `logs/host` pipeline description with: `logs` pipeline applies `resource_detection` on all platforms when host monitoring is enabled; `journald` added as receiver on Linux only
  - Update the Linux scenario: `logs` pipeline includes `journald` receiver and `resource_detection` processor
  - Update the macOS/Windows scenario: `logs` pipeline has no `journald` receiver but still applies `resource_detection`

## 4. Verification

- [x] `make test` — all tests pass
- [x] `make lint` — no new issues
