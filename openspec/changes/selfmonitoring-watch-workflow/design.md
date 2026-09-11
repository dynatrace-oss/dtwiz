# Design

## Context

`dtwiz watch` polls Dynatrace DQL every 5 seconds and renders a live terminal summary. Prior to this change, the command emitted no self-monitoring events. The existing self-monitoring package (`pkg/selfmonitoring`) sends fire-and-forget invocation events for all commands via `rootCmd.PersistentPreRun`. The VI specifies that `dtwiz watch` must emit one span covering session telemetry (time to first data, signals seen). The implementation fires two events: the standard `st=inv` on start, and a `st=com` mid-session when first data arrives or the session times out.

## Goals / Non-Goals

**Goals:**

- Emit one `st=inv` event at start (standard root hook) and one `st=com` event mid-session per `dtwiz watch` invocation.
- Track which of the 8 watch signals (services, hosts, cloud, kubernetes, relationships, logs, requests, exceptions) first appeared and when.
- Encode signal timing compactly in the User-Agent `t=` field to stay within the 64-char HAProxy capture limit.
- Keep `fireSelfMonitoringEvent` fire-and-forget (goroutine) for all events including `st=com`, consistent with all other commands.
- Extend the `selfmonitoring` package with reusable primitives (`StepCompleted`, dynamic title) that benefit future instrumentation beyond watch.

**Non-Goals:**

- Handle Ctrl+C gracefully — the process is killed before the completion path runs; this is a known gap acceptable for the current iteration.
- Instrument watch sessions triggered from within `dtwiz setup` or `dtwiz install` — those are covered by their own spans.
- Change the polling or rendering behaviour of `watchIngest`.

## Decisions

### Two spans: st=inv at start + st=com mid-session

The VI requires session telemetry (duration, signals, timing). The only point where that data is available is when first data arrives or the session times out — both of which happen mid-session, before the user exits. `st=inv` is sent by the standard root `PersistentPreRun` hook (no override needed on `watchCmd`). `st=com` is sent via the `onEvent` callback in `WatchIngestWithEvent`, which fires asynchronously at first data and synchronously at timeout (to guarantee the callback runs before `watchIngest` returns, even though the subsequent `fireSelfMonitoringEvent` call is itself non-blocking).

- Alternative considered: suppress `st=inv` via a `PersistentPreRun` override and emit a single post-session span when the user exits. This loses the invocation signal if the user Ctrl+C's, and requires mirroring root hook additions manually in `watchCmd`. Discarded.

### Signal timing encoded in User-Agent t= (positional CSV, not body)

Watch session data goes into the `t=` field of the User-Agent header rather than into event body properties. This avoids body schema churn and keeps both events structurally consistent with other commands. The format is a positional comma-separated list of whole-second values in fixed signal order (`cld,exc,hst,k8s,log,rel,req,svc`). `0` means the signal was not seen. A signal that was seen encodes as the number of whole seconds to first data, minimum `1` — sub-second arrivals encode as `1`, not `0`, so `0` exclusively means absent. The `t=` field is omitted when no signals were seen.

Worst-case header: `dtwiz/1.8.0;st=com;t=60,45,12,5,30,9,25,3` = 43 chars, within the 64-char HAProxy limit.

- Alternative considered: `t=hst:12,k8s:5,rel:9,svc:3` (named key-value pairs, only present signals). This approach is variable-length and harder to parse at scale; worst case with all 8 signals and 2-digit seconds would approach 64 chars. Discarded in favour of positional encoding.

### c= absent from st=com User-Agent

The `st=com` event sets `params.CmdID = ""` before calling `fireSelfMonitoringEvent`. This drops the `c=wch` segment from the User-Agent, distinguishing completion events from invocation events and saving header space for the `t=` value.

### WatchIngestWithEvent with mid-session callback

A new public wrapper `WatchIngestWithEvent(envURL, pTok, from, onEvent)` is added alongside the existing `WatchIngest`. The internal `watchIngest` function gains an `onEvent func(WatchSessionResult)` parameter. At first data, `onEvent` is called in a goroutine (watch continues uninterrupted). At timeout, `onEvent` is called synchronously before the return statement to guarantee the callback runs. All existing callers pass `nil` and are unaffected.

An `eventFired bool` guard set on the main goroutine before the `go onEvent(...)` call prevents double-firing if timeout occurs after first data.

### Signal tracking via trackWatchSignals

Called after every `pollAll` cycle. For each of the 8 signals, records `time.Since(watchStart).Milliseconds()` the first time the signal transitions from absent to present. Subsequent polls leave the timestamp unchanged. Logs and requests are considered present when `Count > 0 OR Status != ""` — the `Status` field is set during the probe phase before the aggregated count catches up.

### Dynamic event title

All events previously used the hardcoded title `"dtwiz started"`. The title is now derived from `CmdID` and `SubID`: `"dtwiz " + cmdID [+ " " + subID]`. For the `st=com` event (where `CmdID` is cleared), the title is `"dtwiz "` — acceptable since the User-Agent identifies the event fully. This fix applies consistently to all existing and future events.

## Risks / Trade-offs

- Ctrl+C during watch produces no `st=com` event — session telemetry is silently lost. Acceptable for the current iteration; requires context propagation through `watchIngest` to fix.
- `fireSelfMonitoringEvent` for the `st=com` event is fire-and-forget; at timeout, the goroutine may outlive the process. This is the same behaviour as all other commands and is considered acceptable — the timeout path calls `onEvent` synchronously ensuring the goroutine is at least spawned, but there is no guarantee of flush.
- `WatchSessionResult` lives in `pkg/installer` because that is where `watchIngest` lives. If session tracking is needed for other watch variants in future, this type is available for reuse.
