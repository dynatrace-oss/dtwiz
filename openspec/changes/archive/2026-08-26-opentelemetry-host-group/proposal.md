# Proposal: opentelemetry-host-group

## Why

When a user instruments their application with `dtwiz install otel`, the telemetry flowing through the collector has no consistent machine-level grouping attribute. Without `dt.host_group.id`, data from the same host cannot reliably be grouped together in Dynatrace queries and dashboards.

## What Changes

- The OTel Collector config template gains a new `resource/add-host-group-id` processor that upserts `dt.host_group.id` with the hostname of the machine at install time.
- The `otelConfigData` struct gains a `HostGroupID` field, populated via `os.Hostname()` during config generation.
- All pipelines (traces, metrics, logs — with and without host monitoring) include `resource/add-host-group-id` in their processor chains.

## Capabilities

### New Capabilities

- `otel-host-group-id`: Ensures all OTel telemetry emitted from a machine carries `dt.host_group.id` set to the machine hostname, enabling consistent grouping in Dynatrace.

### Modified Capabilities

<!-- none -->

## Impact

- `pkg/installer/otel/otel.tmpl` — new processor definition and pipeline references
- `pkg/installer/otel/collector.go` — `otelConfigData` struct and `generateOtelConfig()` function
- `pkg/installer/otel/collector_test.go` — tests for config generation will need to assert the new processor is present
