# Instrument Remaining Commands

## Why

The `analyze`, `recommend`, `status`, and `version` commands currently fire only a `StepInvoked` self-monitoring event (via root's `PersistentPreRun`), leaving no signal for whether a command completed successfully or failed. Adding a `StepCompleted` event closes that gap and brings these commands to the same observability level as `setup` and `watch`.

## What Changes

- Add a `fireCompletedEvent(cmd, err)` helper to `cmd/selfmonitoring.go`, analogous to the existing `fireInvokedEvent`
- Fire `StepCompleted` at the end of each command's `Run`/`RunE`; set `Err="err"` when the command returns a non-nil error
- No new `StepID` constants needed — `StepCompleted = "com"` already exists
- No new `normSubMap` entries needed — all four are top-level commands with no subcommand
- All events remain gated by the existing `featureflags.SelfMonitoringPoC` flag

## Capabilities

### New Capabilities

- `command-selfmonitoring`: StepCompleted self-monitoring instrumentation for the `analyze`, `recommend`, `status`, and `version` commands — one additional event per run that signals whether the command completed cleanly or failed.

### Modified Capabilities

(none)

## Impact

- `cmd/selfmonitoring.go`: new `fireCompletedEvent(cmd *cobra.Command, err error)` helper
- `cmd/analyze.go`: one new call site at the end of `RunE`
- `cmd/recommend.go`: one new call site at the end of `RunE`
- `cmd/status.go`: one new call site at the end of `RunE`
- `cmd/version.go`: one new call site at the end of `Run`
- `pkg/selfmonitoring/selfmonitoring.go`: no changes
- Gated by `featureflags.SelfMonitoringPoC` — no behavioral change for users without the flag set
