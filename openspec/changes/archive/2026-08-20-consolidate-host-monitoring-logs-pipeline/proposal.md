# Proposal: Consolidate Host Monitoring Logs Pipeline

## Why

When host monitoring is enabled, `otel.tmpl` generates two separate log pipelines:

- `logs` — receives from `otlp` only, no processors — application logs reach Dynatrace but carry no host resource attributes
- `logs/host` — receives from `otlp` + `journald` (Linux only), applies `resource_detection` — journald logs are correlated with the host

This split has two problems:

1. **OTLP logs are not correlated with the host.** Because the `logs` pipeline has no `resource_detection` processor, logs arriving over OTLP (from instrumented apps running on the host) have no `host.name`, `host.id`, or other host attributes. Dynatrace cannot associate them with the host entity, so they are invisible in the host's log view.

2. **OTLP logs are double-exported on Linux.** When `IncludeJournald` is true, `otlp` appears as a receiver in both `logs` and `logs/host`. OTel fans a single receiver out to all pipelines that reference it, so every OTLP log record is exported twice to Dynatrace.

The intended behavior — consistent with the reference Dynatrace host-monitoring config — is a single `logs` pipeline that applies `resource_detection` for all logs, with `journald` added as a source on Linux.

## What Changes

- The separate `logs/host` pipeline is removed from the template
- The `logs` pipeline conditionally applies `resource_detection` when host monitoring is enabled, ensuring all logs (OTLP and journald) are correlated with the host in Dynatrace
- On Linux (`IncludeJournald = true`), `journald` is added as a receiver in the `logs` pipeline alongside `otlp`
- On macOS and Windows, the `logs` pipeline receives only from `otlp` (journald is still omitted — this is unchanged)
- Tests are updated to assert the new pipeline shape instead of `logs/host`

## Unchanged

- The `journald` receiver definition itself — still gated behind `IncludeJournald`, still Linux-only
- All host metrics pipelines (`metrics/host`, `metrics/apps`)
- App-only mode (experimental flag off) — no `resource_detection` added, behavior identical to today
- The `IncludeJournald` field in `otelConfigData` — the guard remains; only the pipeline that references it changes
