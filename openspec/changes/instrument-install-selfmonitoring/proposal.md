# Proposal: Instrument Install Self-Monitoring

## Why

`dtwiz setup` is already instrumented end-to-end, but `dtwiz install <method>` called directly has no self-monitoring beyond the invocation event that fires from `PersistentPreRun`. The PM needs per-method install analytics — funnel drop-off, duration, feature activation, and pass/fail rates — that are structurally impossible to derive from setup events alone because users frequently bypass setup and call install directly.

## What Changes

- Each `dtwiz install <method>` handler emits an `ist` step event when the installer returns, carrying:
  - `install.duration_ms` — elapsed time from user confirmation to install completion, excluding think time at the prompt
  - Per-method feature flags (`install.host_monitoring_enabled`, `install.otel_pipelines`) — only fields applicable to the method are present; inapplicable fields are absent
  - An error field when the install fails or the user cancels; cancellation emits `ist` with `error.type = user_cancelled` so drop-off is visible even without a subsequent data event
- Each handler also wires up a `com` event callback to the post-install WatchIngest session, so first-data timing is captured for all methods (matching the setup pattern)
- A new `ExecutionStart` package-level var in `pkg/installer` records the moment the user confirms, giving accurate duration that excludes the time spent reading the confirmation prompt
- A new `OnWatchComplete` package-level var allows cloud installers (AWS, Azure, GCP) that run WatchIngest internally to receive the event callback without changing their function signatures or breaking existing tests
- Three new WatchIngest variants (`WatchIngestAWSWithEvent`, `WatchIngestCloudWithEvent`, `WatchIngestCloudFromTimeWithEvent`) support the callback path

All 13 install methods are covered: `oneagent`, `kubernetes`, `otel`, `otel-collector`, `otel-python`, `otel-node`, `otel-java`, `aws`, `aws-lambda`, `azure`, `gcp`, `docker` (experimental), `demo` (experimental).

## Capabilities

### New Capabilities

- `install-selfmonitoring`: Defines the observable event contract for `dtwiz install <method>` — which events fire, when, and what properties each carries, including the feature-flag encoding and duration measurement semantics.

### Modified Capabilities

- `setup-selfmonitoring`: No requirement changes — setup events are unaffected. The `ist` step and `com` event patterns introduced here mirror the existing setup contract; no modification to the setup spec is needed.

## Impact

- `cmd/install.go` — all 13 install handlers updated
- `cmd/selfmonitoring.go` — new `fireInstallEvent` and `installMethodFeatures` helpers
- `pkg/installer/installer.go` — new `ExecutionStart` and `OnWatchComplete` package-level vars; `confirmProceed` sets `ExecutionStart` on confirmation
- `pkg/installer/ingest_watch.go` — three new `WithEvent` WatchIngest variants
- `pkg/installer/aws/install.go`, `azure/install.go`, `azure/update.go`, `gcp/install.go`, `gcp/update.go` — switch to `WithEvent` variants consuming `OnWatchComplete`
