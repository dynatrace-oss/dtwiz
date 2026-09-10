# Design

## Context

`dtwiz watch` polls Dynatrace DQL every 5 seconds and renders a live terminal summary. Prior to this change, the command emitted no self-monitoring events. The existing self-monitoring package (`pkg/selfmonitoring`) sends fire-and-forget invocation events for all commands via `rootCmd.PersistentPreRun`. The VI specifies that `dtwiz watch` must emit **one** span — not the standard invocation-on-start pattern used by install commands — because the valuable signal for watch is the full session result: duration, exit reason, and time to first data per signal type.

## Goals / Non-Goals

**Goals:**

- Emit one post-session event per `dtwiz watch` invocation with session telemetry.
- Track which of the 8 watch signals (services, hosts, cloud, kubernetes, relationships, logs, requests, exceptions) first appeared and when.
- Suppress the automatic `StepInvoked` event that `rootCmd.PersistentPreRun` fires for all commands, so watch has exactly one span total.
- Keep the event body compact enough to flush within the timeout cap under normal network conditions.
- Extend the `selfmonitoring` package with reusable primitives (`StepCompleted`, `ExtraProps`, dynamic title) that benefit future instrumentation beyond watch.

**Non-Goals:**

- Handle Ctrl+C gracefully — the process is killed before the completion path runs; this is a known gap acceptable for the current iteration.
- Instrument watch sessions triggered from within `dtwiz setup` or `dtwiz install` — those are covered by their own spans.
- Change the polling or rendering behaviour of `watchIngest`.

## Decisions

### One span, post-session (not invocation-first)

The VI explicitly requires one span for watch, covering the session from start to first data or timeout. The standard invocation-first pattern (used for install commands) exists specifically to capture process kills mid-install. Watch sessions are non-destructive; a missed Ctrl+C event is acceptable. Firing at the end is the only point where session data (duration, signals, timing) is available.

- Alternative considered: fire an invocation event at start, discard the session data. This satisfies the "one span" count but loses the only meaningful data watch produces.

### Suppress `rootCmd.PersistentPreRun` via override

`watchCmd` defines its own `PersistentPreRun` that reproduces the essential setup (`logger.Init`, `featureflags.ApplyCLIOverrides`) without calling `fireSelfMonitoringEvent`. This is the standard Cobra pattern for per-command pre-run customisation.

- Maintenance risk: future additions to `rootCmd.PersistentPreRun` need to be mirrored in `watchCmd.PersistentPreRun`. The override comment documents the reason.
- Alternative considered: skip the override and filter inside `fireSelfMonitoringEvent` based on command name. This is fragile and couples the selfmonitoring package to command names.

### 500ms flush cap (deviation from VI's 200ms)

The VI specifies 200ms. Testing against `dev.dynatracelabs.com` showed a round-trip of ~201ms, causing the event to be dropped on every invocation. The 200ms cap was designed for fast-exit commands (cancelled installs) where a longer wait would be noticeable. Watch sessions are already seconds to minutes long; 500ms is imperceptible. All other commands remain unaffected (they use a fire-and-forget goroutine with no cap).

### `WatchIngest` returns `WatchSessionResult`

The internal `watchIngest` function is changed to a named return. `WatchIngest` (the public wrapper) now returns `WatchSessionResult`. Existing callers that don't need the result can discard it — Go allows calling a value-returning function without capturing the return. No call sites required changes.

### Signal tracking via `trackWatchSignals`

Called after every `pollAll` cycle. For each of the 8 signals, records `time.Since(watchStart).Milliseconds()` the first time the signal transitions from absent to present. Subsequent polls leave the timestamp unchanged. Logs and requests are considered present when `Count > 0 OR Status != ""` — the `Status` field is set during the probe phase before the aggregated count catches up.

### `ExtraProps` on `EventParams`

Watch session data (duration, exit reason, per-signal timings) goes into the event body `properties` map, not into the User-Agent or Tab-Id headers which have strict HAProxy capture limits (64 and 16 chars respectively). `ExtraProps map[string]string` is merged into the properties map via `maps.Copy` and intentionally excluded from header construction.

### Dynamic event title

All events previously used the hardcoded title `"dtwiz started"`. The title is now derived from `CmdID` and `SubID`: `"dtwiz " + cmdID [+ " " + subID]`. This fixes the misleading title on completion events and applies consistently to all existing and future events without requiring a per-call title field.

## Risks / Trade-offs

- Ctrl+C during watch produces no event — session telemetry is silently lost. Acceptable for the current iteration; requires context propagation through `watchIngest` to fix.
- The 500ms cap can still be exceeded on very high-latency connections. The event is dropped silently; the timeout debug log makes this diagnosable with `--debug`.
- `watchCmd.PersistentPreRun` is a maintenance trap — future root hook additions require a manual mirror. Documented via comment.
- `WatchSessionResult` lives in `pkg/installer` because that is where `watchIngest` lives. If session tracking is needed for other watch variants in future, this type is available for reuse.
