## Why

`dtwiz install otel` can send its collector verification log before the OpenPipeline dynamic routes for OTel Host Monitoring are applied. On a first install, that log can be ingested before host-monitoring routing is ready, so it may not be associated with the host entity it came from.

## What Changes

- Ensure tenant-side OTel Host Monitoring prerequisites are applied before `install otel` emits collector verification telemetry.
- Apply the same ordering to selected project instrumentation, so app telemetry is not started before host-monitoring routes are ready.
- Keep route setup advisory: route failures still warn and the collector install continues.
- Apply the same ordering to the standalone `install otel-collector` path, which also emits a verification log.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `otel-host-monitoring-grail-routes`: dynamic routes must be applied before verification or auto-instrumented telemetry is emitted during OTel install flows.

## Impact

- Affected code: `pkg/installer/otel/otel.go`, `pkg/installer/otel/collector.go`, and OTel installer tests.
- Affected behavior: OTel installs may spend the existing bounded route/pipeline wait before starting the collector or application instrumentation.
- Dependencies: no new dependencies.
- Rollback: move route application back after collector execution if the ordering introduces an unexpected regression; existing advisory warning behavior limits tenant-side failure impact.