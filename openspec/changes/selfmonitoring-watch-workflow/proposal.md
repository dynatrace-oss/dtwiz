# Proposal

## Why

Per VI PRODUCT-17847, every meaningful dtwiz action must be instrumented and visible in the self-monitoring tenant. The `dtwiz watch` command had no instrumentation, making it impossible to measure time to first data, confirm that WatchIngest signal detection is working, or understand how many watch sessions end with data vs. without.

## What Changes

- `dtwiz watch` emits a single post-session self-monitoring span to the Dynatrace Events v2 API when the session ends (user presses Enter or the 10-minute timeout fires).
- The span captures: session duration, exit reason (`user_exit` / `timeout`), which signal types appeared during the session, and the time to first data per signal type.
- The `selfmonitoring` package is extended with a `StepCompleted` constant, an `ExtraProps` field on `EventParams` for per-event structured data, and a dynamic event title derived from command and subcommand identifiers.
- The `WatchIngest` function is extended to return a `WatchSessionResult` so callers can access session telemetry without changing the watch logic.
- The flush cap for the watch completion event is set to **500ms** rather than the 200ms specified in the VI. The 200ms cap was designed for fast-exit commands where the delay is perceptible; watch sessions already take seconds to minutes, so a 500ms cap is imperceptible to the user and necessary to avoid dropping events against environments where the round-trip consistently exceeds 200ms.

## Capabilities

### New Capabilities

- `selfmonitoring-watch`: Emit one post-session event per `dtwiz watch` invocation carrying session duration, exit reason, signals seen, and time to first data per signal.

### Modified Capabilities

- `selfmonitoring`: Added `StepCompleted` constant, `ExtraProps` map on `EventParams`, dynamic event title, and pre-request debug logging.
- `ingest-watch`: `WatchIngest` now returns `WatchSessionResult`; signal first-data tracking runs on every poll cycle.

## Impact

- Affects `pkg/selfmonitoring/selfmonitoring.go`, `pkg/installer/ingest_watch.go`, `cmd/watch.go`, `cmd/root.go`.
- No new dependencies or breaking CLI changes.
- Gated by the existing `DTWIZ_SELF_MONITORING_POC` feature flag — no user-visible change when the flag is off.
- Deviates from the VI's 200ms flush cap: watch uses 500ms. All other commands remain at 200ms (fire-and-forget goroutine, no flush).
