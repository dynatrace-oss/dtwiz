# Proposal: Update Dynatrace OTel Collector

## Why

`dtwiz update otel` existed as a stub that silently patched a dtwiz-managed config in the background, with no picker, no preview, and no restart. Operators rotating credentials or changing tenant URLs had no supported interactive path to reconcile their running collector.

Beyond the config update itself, app services connected to the collector (via OTLP TCP connections or env-var tenant match) are disrupted when the collector restarts. The update flow should detect them, surface them in the preview, and restart them with a corrected OTLP environment.

Additionally, the Dynatrace-distributed collector (`dynatrace-otel-collector`) requires a different approach from upstream collectors: its config must be regenerated from the install template rather than patched by merging an extra exporter entry.

## What Changes

- `dtwiz update otel` runs a full interactive flow: running-collector picker (when `--config` is omitted), config diff preview, confirmation prompt, collector restart, and connected-service restart
- **Dynatrace OTel Collector** (`dynatrace-otel-collector` binary): config is regenerated from the install template with current tenant credentials; existing OTLP receiver ports are preserved so connected services keep their endpoint
- **Upstream OTel Collector**: the Dynatrace OTLP exporter is merged into the existing config using YAML node-level editing that preserves comments, key order, and flow/block style
- `ErrUpToDate` sentinel: clean exit when the Dynatrace collector config is already current (byte-identical to a fresh template render)
- `verifyOtelInstall` and `waitForOtelCollectorReady` now accept the OTLP HTTP port from the config instead of hardcoding 4318
- `renderOtelTemplate` extracted to a shared helper used by both install and update paths

## Capabilities

### New Capabilities

- `dtwiz update otel`: Full interactive update — picker, diff preview, collector restart, connected-service restart
- `updateDynatraceCollector`: Template-based config regeneration for Dynatrace-distributed collectors; calls `WatchIngest()` after successful restart
- `detectConnectedServices`: Finds app processes connected to the collector via OTLP TCP connections or tenant-matched OTLP env vars
- `restartConnectedServices`: Stops and relaunches connected services with reconciled or retargeted OTLP environment

### Modified Capabilities

- `verifyOtelInstall`, `waitForOtelCollectorReady`: Accept `httpPort` parameter (port read from config, fallback 4318)
- `cmd/setup.go`, `cmd/update.go`: Treat `ErrUpToDate` as a clean exit alongside `ErrInstallCancelled`

## Impact

- **New files**: `pkg/installer/otel/service_detect.go`, `pkg/installer/otel/service_detect_unix.go`, `pkg/installer/otel/service_detect_unix_test.go`, `pkg/installer/otel/service_detect_windows.go`, `pkg/installer/otel/service_detect_test.go`, `pkg/installer/otel/update_dynatrace.go`, `pkg/installer/otel/update_dynatrace_test.go`
- **Modified files**: `pkg/installer/otel/collector.go`, `pkg/installer/otel/collector_test.go`, `pkg/installer/otel/update.go`, `pkg/installer/otel/update_test.go`, `cmd/update.go`, `cmd/setup.go`
- **Dependencies**: none new — uses existing `gopkg.in/yaml.v3`; `lsof(1)` and `ps(1)` on Unix (system tools)
- **APIs used**: none new
