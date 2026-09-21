# Proposal

## Why

Every dtwiz command failure must be classified and visible in the self-monitoring tenant. Currently, the `EventParams.Err` field exists in `pkg/selfmonitoring` but is never populated — all self-monitoring events carry an empty error field regardless of outcome. This means the dashboard cannot distinguish a successful install from a failed one, and the drop-off funnel (which commands fail, at what step, and why) is invisible.

A second gap compounds the problem: self-monitoring events are fired in detached goroutines. Fast-failing commands — auth errors, missing dependencies, user cancellations — exit in under 100ms. The goroutine is killed before the HTTP request to the tenant completes, so the most interesting failure signals are systematically lost.

Both problems must be solved together. Error taxonomy without flush classifies errors correctly but loses the classified events for exactly the fast-fail cases the funnel depends on. Flush without taxonomy produces reliable events with empty error fields that cannot power failure analysis.

## What Changes

- **Flush strategy:** Replace the fire-and-forget goroutine in `fireSelfMonitoringEvent` with an enqueue model. A `Flush(200ms)` call in `Execute()` drains all pending events concurrently before process exit — on every path (success, error, cancel, CTRL+C).
- **Error taxonomy:** Introduce typed error types and a `ClassifyError` function that maps any Go error to a structured taxonomy category and its additional attributes, following the structured error taxonomy.
- **Terminal events:** Fire a `StepFailed` or `StepCancelled` event at every `RunE` return point across all command handlers, carrying the classified error type.
- **Typed errors at source:** Replace opaque `fmt.Errorf` strings in auth validation, dependency checks, and platform-unsupported guards with typed errors that carry machine-readable fields.

## Capabilities

### New Capabilities

- `selfmonitoring-error-taxonomy`: Classify command failures into `user_cancelled`, `auth_error`, `config_error`, `dependency_missing`, `network_error`, `install_failed`, or `platform_unsupported`, and include the classification and additional attributes in self-monitoring events.
- `selfmonitoring-flush`: Enqueue self-monitoring events and synchronously flush them (≤200ms) before process exit, ensuring fast-fail events reach the tenant.

### Modified Capabilities

- `selfmonitoring`: Add `Enqueue`, `Flush`, `StepFailed`, and `StepCancelled` to `pkg/selfmonitoring`. `fireSelfMonitoringEvent` resolves credentials eagerly and enqueues; the goroutine is removed.
- `installer-errors`: New `pkg/installer/errors.go` defines `ErrorType` constants, typed error structs (`AuthError`, `ConfigError`, `DependencyMissingError`, `NetworkError`, `InstallFailedError`, `ErrPlatformUnsupported`), and `ClassifyError`.

## Impact

- Affects `pkg/selfmonitoring/selfmonitoring.go`, `pkg/installer/errors.go` (new), `cmd/selfmonitoring.go`, `cmd/root.go`, `cmd/auth.go`, `cmd/install.go`, `cmd/setup.go`, `cmd/update.go`, `cmd/uninstall.go`, `pkg/installer/oneagent/oneagent.go`, `pkg/installer/otel/collector.go`, and ~16 `exec.LookPath` failure sites across installer packages.
- No new external dependencies. No new CLI flags or breaking changes.
- Gated by the existing `DTWIZ_SELF_MONITORING_POC` feature flag — no user-visible change when the flag is off.
- Known limitation: `config_error` with missing `DT_ENVIRONMENT` is always lost regardless of flush — the tenant URL is unknown, so there is no destination to send to. This is an accepted constraint with no workaround.
