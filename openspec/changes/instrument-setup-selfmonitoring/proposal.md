# Instrument Setup Self-Monitoring

## Why

The `setup` command is the primary onboarding flow and the most important command for understanding how users engage with dtwiz — which methods they select, where they drop off, and which installers succeed or fail. Without instrumentation, this flow is a blind spot in self-monitoring telemetry.

## What Changes

- Add 3 new `StepID` constants to `pkg/selfmonitoring`: `StepAnalyze` (`"ana"`), `StepRecommend` (`"rec"`), `StepInstall` (`"ist"`)
- Add 3 missing update-variant entries to `normSubMap` in `cmd/selfmonitoring.go`: `"otel-update"` → `"otlu"`, `"azure-update"` → `"azu"`, `"gcp-update"` → `"gcpu"`
- Fire 4 self-monitoring events in `cmd/setup.go` at the wizard's natural checkpoints: invocation, analysis, recommendation/selection, and install completion

## Capabilities

### New Capabilities

- `setup-selfmonitoring`: Self-monitoring instrumentation for the `setup` wizard — 4 events per run (inv, ana, rec, ist) that allow correlating a full setup session, measuring step durations, and identifying where users drop off or encounter failures.

### Modified Capabilities

(none — no existing spec-level behavior changes)

## Impact

- `pkg/selfmonitoring/selfmonitoring.go`: 3 new exported step constants
- `cmd/selfmonitoring.go`: 3 new entries in `normSubMap`
- `cmd/setup.go`: 4 new `fireSelfMonitoringEvent` call sites
- Gated by the existing `featureflags.SelfMonitoringPoC` feature flag — no behavioral change for users without the flag set
- Early events (`inv`, `ana`) will silently fail when `DT_ENVIRONMENT`/`DT_PLATFORM_TOKEN` are not set; this is expected for first-time setup users and acceptable given the best-effort nature of self-monitoring
