## Why

The install telemetry can't answer the two questions we care most about: did the features we enable during an install actually get enabled, and how long did the install itself take? Today `install.host_monitoring_enabled` is a fixed value per method, sent as `"true"` even when the extension activation failed. `install.duration_s` is never sent for installs started from `dtwiz setup`, and for Azure, GCP and AWS it includes the entire post-install watch session. Neither value appears in the User-Agent, so both are lost whenever the event body is rejected.

## What Changes

- The `ist` event reports the **real outcome** of each feature dtwiz tries to enable, recorded where the work happens rather than looked up per method:
  - OTel config written
  - Host Monitoring enabled
  - RUM enabled (reserved; wired up when RUM injection lands)
  - Synthetic Monitoring enabled (reserved)
  - RDS extensions enabled (reserved)
- Each feature has three states: succeeded (including "already in place"), tried and failed, not tried.
- The User-Agent gets a positional field `f=` with one character per feature (`1`, `0`, `-`) and no separators, e.g. `f=11---`. Positions are fixed and add-only.
- The event body gets one readable property per feature, e.g. `install.host_monitoring: "succeeded" | "failed" | "not_tried"`.
- **BREAKING (telemetry):** `install.host_monitoring_enabled` and `install.otel_pipelines`, shipped in v1.10.0, are removed in favor of the new properties.
- Install duration measures **work time only**: from the install confirmation to the end of the installer's own work. User think time at later prompts and the post-install watch are excluded.
  - Fixes cloud installs (AWS, Azure, GCP) counting the watch session.
  - Fixes the Python "recreate virtualenv?" prompt resetting the start time mid-install.
  - AWS ends when both the CloudFormation deploy and the Lambda instrumentation have finished.
- **BREAKING (telemetry):** `install.duration_s` (whole seconds) is replaced by `install.duration_ms` in the body. The User-Agent gets `d=<seconds>`, clamped to 9999.
- `dtwiz setup`'s `ist` event carries the same duration and feature properties as `dtwiz install <method>`.

Out of scope: direct `dtwiz update otel|azure|gcp` commands (setup's update choices are in scope), and changing when events are fired. Setup still sends no `ist` for cancelled or up-to-date runs.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `install-selfmonitoring`: the `ist` event's feature properties change from fixed per-method values to real three-state outcomes, gaining the `f=` User-Agent field. Duration semantics change to work time with explicit end points, gaining the `d=` User-Agent field.
- `setup-selfmonitoring`: setup's `ist` event carries the same duration and feature outcomes as direct installs.

## Impact

- `cmd/selfmonitoring.go`: `installMethodFeatures` removed; the `ist` builders read recorded outcomes and the stopwatch.
- `cmd/install.go`, `cmd/setup.go`: reset the recorder and stopwatch before each install; drop `installer.ExecutionStart`.
- `pkg/selfmonitoring`: new `Features` and `DurationS` fields on `EventParams`, encoded as `f=` and `d=`.
- `pkg/installer`: new install stopwatch and feature-outcome recorder (package-level, following the `AutoConfirm`/`OnWatchComplete` pattern); `confirmProceed` starts or pauses the stopwatch.
- Installers that record outcomes or mark end of work: `otel` (collector config write, host monitoring activation, update paths reached from setup), `oneagent`, `docker`, `aws`, `azure`, `gcp`.
- Dashboards and queries that use `install.duration_s`, `install.host_monitoring_enabled` or `install.otel_pipelines` must switch to the new properties. Only v1.10.0 events carry the old keys.
- Rollback: revert the change. The event endpoint and the other steps are untouched, so older and newer CLI versions can report side by side.
