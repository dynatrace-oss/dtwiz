# Proposal

## Why

dtwiz has no signal that confirms it is being used by customers. Without telemetry, it is impossible to know whether installs succeed, which methods are popular, or whether the tool is reaching real environments. Pipeline propagation of a dtwiz-sourced event has been confirmed: a `CUSTOM_INFO` event sent via the Events v2 API survives the Dynatrace ingest pipeline and can be correlated. This change lays the foundation for self-monitoring by introducing the `pkg/selfmonitoring` package and wiring it into the command lifecycle behind an opt-in feature flag.

## What Changes

- Introduce a new `pkg/selfmonitoring` package with a single exported function that fires a `CUSTOM_INFO` event to the Dynatrace Events v2 API.
- Encode per-invocation metadata in event body properties (`e`, `c`, `st`, optional `s`, `er`, `t`) and dual HTTP headers: `User-Agent` (operation identity: cmd, step, sub, err, type) and `Tab-Id` (execution context: exec ID, mode, OS).
- Embed a custom `dtwiz-monitoring: dtwiz-start` header so the event can be identified and traced through the pipeline independently of the event body.
- Gate the feature behind the env var `DTWIZ_SELF_MONITORING_POC=true` (or `=1`) via the feature-flag registry; strictly opt-in, never runs in a normal user session.
- Call the event sender at the start of every dtwiz command via `PersistentPreRun` hooks on root, install, update, and uninstall commands.

## Capabilities

### New Capabilities

- None (PoC only; no user-visible behavior).

### Modified Capabilities

- None (opt-in env-var gate; existing commands are unaffected when the var is absent).

## Impact

- Adds `pkg/selfmonitoring/selfmonitoring.go` (new package, no external dependencies beyond stdlib).
- Registers `SelfMonitoringPoC` feature flag in `pkg/featureflags/featureflags.go` with env var `DTWIZ_SELF_MONITORING_POC`.
- Adds `fireSelfMonitoringEvent()` and supporting functions in `cmd/root.go`; normalizes command/subcommand names via lookup maps.
- Adds import and `fireSelfMonitoringEvent()` call in `cmd/install.go`, `cmd/update.go`, `cmd/uninstall.go`.
- No new CLI flags, no user-visible output, no breaking changes.
- All errors are swallowed at debug level; the tool's normal operation is unaffected even if the event fails.
