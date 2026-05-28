# Proposal: OTel Collector Home Directory

## Why

The OTel Collector install previously used the current working directory (`<cwd>/opentelemetry/`) as the installation path. This causes permission errors when the user runs `dtwiz install otel` from a directory they don't fully own (e.g., `/opt/`, system paths, or restricted project roots). Using the user's home directory (`~/opentelemetry/`) eliminates permission issues on all platforms and provides a stable, predictable install location regardless of where the CLI is invoked.

## What Changes

- `otelCollectorInstallDir()` now returns `~/opentelemetry` on all platforms (Linux, macOS, Windows) via `os.UserHomeDir()`
- `prepareCollectorPlan()` uses the new helper instead of `os.Getwd()`
- `updateOtelCollectorIfPresent()` uses the new helper to locate the config file
- The uninstall flow already checked `~/opentelemetry` as a candidate directory — no change needed there

## Capabilities

### New Capabilities

_None — this is a behavioral change to an existing capability._

### Modified Capabilities

- `install-demo`: The collector install directory is now `~/opentelemetry` instead of `<cwd>/opentelemetry`
- `java-auto-instrumentation`: The well-known collector config path changes from `<cwd>/opentelemetry/config.yaml` to `~/opentelemetry/config.yaml`

## Impact

- `pkg/installer/otel_collector.go` — new `otelCollectorInstallDir()` function; `prepareCollectorPlan()` updated
- `pkg/installer/otel_update.go` — `updateOtelCollectorIfPresent()` updated
- Uninstall is unaffected (already checks both `~/opentelemetry` and `<cwd>/opentelemetry`)
