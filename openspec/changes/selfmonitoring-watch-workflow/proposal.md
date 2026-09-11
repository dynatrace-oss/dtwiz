# Proposal

## Why

Per VI PRODUCT-17847, every meaningful dtwiz action must be instrumented and visible in the self-monitoring tenant. The `dtwiz watch` command had no instrumentation, making it impossible to measure time to first data, confirm that WatchIngest signal detection is working, or understand how many watch sessions end with data vs. without.

## What Changes

- `dtwiz watch` emits **two** self-monitoring events per invocation:
  1. `st=inv` — sent immediately at command start via the standard root `PersistentPreRun` hook (no watch-specific override needed).
  2. `st=com` — sent mid-session when first data is received, or synchronously at the 10-minute timeout. Does not wait for the user to exit.
- The `st=com` event encodes which signal types appeared and the time to first data per signal type in the User-Agent `t=` field as a positional comma-separated list of whole-second values. No watch-specific body properties are added.
- The `c=` field (command identifier) is absent from the `st=com` event's User-Agent to distinguish it from the invocation event.
- `WatchIngestWithEvent` is added as a public wrapper around `watchIngest`, accepting an `onEvent func(WatchSessionResult)` callback that fires mid-session at first data (asynchronously) or at timeout (synchronously before return).
- The `selfmonitoring` package gains a `StepCompleted` constant and a dynamic event title derived from command and subcommand identifiers.

## Capabilities

### New Capabilities

- `selfmonitoring-watch`: Emit two events per `dtwiz watch` invocation — one on start, one on first data or timeout — carrying signal timing data in the User-Agent header.

### Modified Capabilities

- `selfmonitoring`: Added `StepCompleted` constant, dynamic event title, and pre-request debug logging.
- `ingest-watch`: `WatchIngestWithEvent` public wrapper with mid-session event callback; `trackWatchSignals` records first-data timestamps per poll cycle.

## Impact

- Affects `pkg/selfmonitoring/selfmonitoring.go`, `pkg/installer/ingest_watch.go`, `cmd/watch.go`, `cmd/root.go`.
- No new dependencies or breaking CLI changes.
- Gated by the existing `DTWIZ_SELF_MONITORING_POC` feature flag — no user-visible change when the flag is off.
- No deviation from the VI's flush behaviour: `st=com` is fire-and-forget (goroutine), same as all other commands. The timeout path calls `onEvent` synchronously, but `fireSelfMonitoringEvent` inside it spawns a goroutine and returns immediately.
