# Proposal: Update Dynatrace OTel Collector

## Why

`dtwiz update otel` existed as a stub that silently patched a dtwiz-managed config in the background, with no picker, no preview, and no restart. Operators rotating credentials or changing tenant URLs had no supported interactive path to reconcile their running collector.

Beyond the config update itself, app services connected to the collector (via OTLP TCP connections or env-var tenant match) are disrupted when the collector restarts. The update flow should detect them, surface them in the preview, and restart them with a corrected OTLP environment.

Additionally, the Dynatrace-distributed collector (`dynatrace-otel-collector`) requires a different approach from upstream collectors: its config must be regenerated from the install template rather than patched by merging an extra exporter entry.

## What Changes

- `dtwiz update otel` runs a full interactive flow: running-collector picker (when `--config` is omitted), config diff preview, confirmation prompt, collector restart, and connected-service restart
- **Dynatrace OTel Collector** (`dynatrace-otel-collector` binary): config is regenerated from the install template with current tenant credentials; existing OTLP receiver ports are preserved so connected services keep their endpoint
- **Upstream OTel Collector**: the Dynatrace OTLP exporter is merged into the existing config using YAML node-level editing that preserves comments, key order, and flow/block style
- Clean exit when the Dynatrace collector config is already current (byte-identical to a fresh template render)
- Collector verification uses the OTLP HTTP port read from the config instead of hardcoding 4318

## Capabilities

### New Capabilities

- `dtwiz update otel`: Full interactive update — picker, diff preview, collector restart, connected-service restart
- Dynatrace OTel Collector update: template-based config regeneration; polls until new telemetry arrives in Dynatrace after successful restart
- Connected service detection: finds app processes connected to the collector via OTLP TCP connections or tenant-matched OTLP env vars
- Connected service restart: stops and relaunches connected services with reconciled or retargeted OTLP environment

### Modified Capabilities

- Collector install verification: uses OTLP HTTP port read from the config file (fallback 4318) instead of hardcoding it
- `dtwiz setup` and `dtwiz update otel`: treat "up to date" as a clean exit alongside user cancellation
