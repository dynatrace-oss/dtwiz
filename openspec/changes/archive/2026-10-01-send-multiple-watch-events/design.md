# Design

## Context

`watchIngest` in `pkg/installer/ingest_watch.go` polls Dynatrace every 5 seconds and latches the first-data timestamp for each of eight signal types into `WatchSessionResult.FirstDataMs`. That per-signal latch is correct and complete. The problem is downstream: a single `eventFired` boolean gates emission, so the callback runs on the first poll cycle where *any* signal has data and never again. Everything recorded after that moment is accumulated in memory and thrown away when the session ends.

Constraints the design has to respect:

- **64-character User-Agent capture limit.** The header is a HAProxy capture, and it is the only telemetry that survives when the Events API rejects the request (auth failure, quota). It is therefore a real channel, not a duplicate of the body, and it cannot be allowed to truncate silently.
- **`c=wch` must stay on every watch event.** It was removed once and added back after it made querying awkward. Not up for renegotiation.
- **`t=` positional CSV encoding, signal set, signal order, and body property names stay as they are.**
- **Five callers of a shared package-level callback var** (`installer.OnWatchComplete`, read by the AWS, Azure, and GCP installers) plus five `WatchIngest*WithEvent` wrappers over `watchIngest`, so any signature change fans out.
- **No SIGINT handler exists anywhere in the codebase.** `selfmonitoring.Flush(500ms)` in `cmd.Execute` is the only flush and it only runs on normal return.

## Goals / Non-Goals

### Goals

- Record time-to-first-data for every signal type that receives data during a watch session, not just the first one.
- Make each emitted event independently interpretable, so partial data from an abandoned session is still usable.
- Make "the session ended cleanly" distinguishable from "the process died."
- Keep the header encoding bounded regardless of session length.

### Non-Goals

- Improving measurement resolution. The 5-second poll interval stays, so recorded values are quantized to poll boundaries and include the time taken to run the queries. A recorded value is therefore the time at which data was *observed*, not the time at which it was produced, and a value of `1` means the data was already present on the session's first poll. Shortening the interval to sharpen this would multiply query load for marginal gain.
- Recording *which* signal type triggered a given event. Events are combined downstream, so the cumulative snapshot is sufficient and a delta marker would be redundant.
- Putting session duration or exit reason into the event payload.
- Removing the dead `WatchSessionResult.Duration` and `ExitReason` fields, or the ignored `WatchIngest*` return values. Deliberately out of scope to keep the diff proportional to the feature. Splitting the callback in two means `ExitReason` stays written-but-never-read rather than becoming load-bearing.
- Adding a signal handler so events survive Ctrl+C.

## Decisions

### Emit on change, not on an interval

Emit when the count of seen signal types exceeds the count at the last emission. An `emittedCount int` watermark replaces the `eventFired bool`; since `FirstDataMs` only ever grows and entries are never removed, the count alone is a sufficient watermark.

*Alternative: interval-triggered heartbeat.* Emitting a snapshot on every poll cycle regardless of change would have made session duration and death time fall out for free (the last event's ingest timestamp *is* the death time), and would have made the completion event unnecessary. Rejected on volume: roughly 120 events per 10-minute session, multiplied by every install that ends in a post-install watch, to buy liveness data that is obtainable more cheaply.

*Alternative: one event at session end.* Simplest possible change, and carries identical information to the final incremental snapshot. Rejected because watch is the command users walk away from and Ctrl+C out of, and with no signal handler an end-of-session-only event is lost exactly in the cases that matter most.

### Cumulative snapshots, not deltas

Each `st=snp` event carries every signal type seen so far. A value, once recorded, is repeated verbatim in all later events.

The analysis model is therefore "last snapshot per `executionId` wins," and every intermediate event is a valid prefix of the final answer. A delta encoding would be shorter and would identify the newly-seen signal for free, but it makes every event dependent on its siblings: lose one and the reconstruction is wrong, with no way to detect that it is wrong.

### New step shortcode `st=snp`

Add `StepSnapshot = "snp"` with `stepFullNames["snp"] = "snapshot"`. Verified free against the existing shortcodes (`inv`, `ana`, `rec`, `ist`, `com`, `fai`, `can`).

*Alternative: keep everything under `st=com`.* Rejected because with N events per session, "completed" is false for all but the last, and watch snapshots would only be separable from real completions by also filtering on `command == "watch"`. The wire break is paid once and buys an honest, directly filterable step value.

Header length is unaffected: `snp` and `com` are both three characters.

### Unconditional `st=com` on every exit path

Fire the completion event from a `defer` in `watchIngest`, placed *after* the platform-token guard. That covers the four real exit paths (standard input error, Enter pressed, non-interactive timeout, interactive timeout declined) exactly once each, and correctly skips the token guard, which returns before any watching happens.

This deletes the two conditional `onEvent(snapEvent("timeout"))` call sites and the `eventFired` latch outright. The old "fire an empty event on timeout so there is at least one event per execution" hack becomes redundant, because now there is always a terminal event.

The completion event carries no `t=` and no signal properties, matching `analyze`, `status`, `recommend`, `update`, `uninstall`, and `version`, all of which already emit a bare `buildEventParams(cmd, StepCompleted)`.

### Two callbacks, one per event kind

`watchIngest` takes `onSnapshot` and `onComplete` instead of a single `onEvent`. Each fires for exactly one event kind, so there is no discriminator to carry and no branch to get wrong:

```text
seen-set grew    → onSnapshot(snapEvent(...))  → st=snp;t=...  + body props
session ended    → onComplete(snapEvent(...))  → st=com        (no t=, no props)
```

On the `cmd/` side this splits into two builders. The existing `buildWatchEventCallback` becomes the completion callback, losing its `Type` and `ExtraProps` assignments and keeping `StepCompleted`. A new `buildWatchSnapshotEventCallback` emits `StepSnapshot` with the signal payload. Both keep `params.Cmd = "watch"` and `params.Sub = ""`.

The package-level var splits the same way: `installer.OnWatchComplete` keeps its name, and now finally matches its behavior, while a new `installer.OnWatchSnapshot` sits beside it.

Both callbacks take `WatchSessionResult`, even though the completion event reads nothing from it. Two reasons. First, `snapEvent` is what guarantees the `maps.Clone`, so routing both through it keeps the clone on the only path in; a `func(map[string]int64)` signature would let `go onSnapshot(result.FirstDataMs)` compile and race. Second, adding `watch.duration_s` to the completion event later (see Open Questions) would otherwise force a signature change across six functions and every call site. A `func()` signature would be more honest about what the completion event carries today, at the cost of that future churn.

*Alternative: one callback with a discriminator*, either the existing `ExitReason` string or a new `Final bool`. Smaller diff: one parameter stays one parameter. Rejected because it puts a branch in the callback where the caller already knows which event it wants, and in the `ExitReason` variant that branch is a string comparison where a typo compiles and silently emits the wrong step.

*Alternative: a `WatchCallbacks` struct* holding both functions. Keeps arity at one and so avoids touching call-site argument lists. Rejected as indirection that buys only brevity; two plain parameters read more directly at every call site, and the call-site edits are mechanical.

The cost is roughly 30 mechanical line edits: six signatures (`watchIngest` plus five `*WithEvent` wrappers), six internal calls in the non-event variants that pass `nil, nil`, thirteen `cmd/` call sites that pass both builders, six `cmd/` sites that assign both package vars, and five cloud-installer sites that forward both.

### Measure from session start

`trackWatchSignals` currently receives `watchStart`, which is reset to `time.Now()` when the user accepts the 10-minute continuation prompt. `sessionStart` is never reset. Pass `sessionStart` instead.

Today this is nearly invisible, because the single event has already fired long before minute 10 in any session that saw data. With incremental emission it becomes a correctness bug: a signal first seen at minute 12 would report ~120 seconds while `hosts` in the same `t=` vector still reports its value from the original epoch, putting two different clocks in one vector with nothing to indicate it.

The display clock keeps using `watchStart`, because resetting the on-screen elapsed counter on continuation is intended behavior. `snapEvent` already computes `Duration` from `sessionStart` and is unaffected; the unused deferred `result.Duration` assignment is repointed at `sessionStart` for consistency rather than left as a landmine.

### Clamp `t=` values at 999 seconds

Switching to the session-start epoch makes the values unbounded, because repeated continuations extend the session indefinitely. Previously they were implicitly capped at 600 by the reset.

Worst case with the clamp: `dtwiz/1.10.1-next;c=wch;st=snp;t=999,999,999,999,999,999,999,999` = 64 characters, exactly at the limit. This is **identical to the current worst case** (`st=com;t=600,600,...`, also 64, because the reset-bounded values were already three digits), so the change is length-neutral.

Measured, the fixed overhead is 53 characters, leaving exactly 11 for the version string. Goreleaser's snapshot template (`{{ incpatch .Version }}-next`) produces exactly 11 today (`1.10.1-next`), so there is zero slack: at v1.10.9 the next snapshot build is `1.10.10-next` at 12 characters, and the tail of `t=` truncates. **This cliff is pre-existing and out of scope here**, since the completion event being replaced encoded the same eight three-digit values, but it is real and close, and worth a follow-up that shortens the header (dropping `c=wch` from snapshots, or a denser encoding than three-digit decimal).

A unit test pins the budget rather than a specific version: it asserts the leftover room for the version string is at least 11 characters, so anything that lengthens the header (another field, a wider clamp, a ninth signal type) fails the test instead of silently truncating in production.

Clamping is cheap because the body property carries the unclamped millisecond value. Header lossiness only degrades the fallback channel, which matters solely when the event itself fails to land.

*Alternative: 5-second ticks in the header* (value = seconds / 5). Same header length, but pushes the lossy threshold from 16 minutes to 83 minutes, and costs nothing in precision since the poll interval is 5 seconds anyway. Rejected as a footgun: `t=120` reading as 600 seconds is confusing for anyone inspecting raw headers, and the clamp threshold is already beyond realistic session lengths.

*Alternative: stop emitting snapshots after the first 10-minute round.* Rejected: bounds the values by discarding the slow tail (Smartscape relationships, K8s topology, cloud resources), which is the most interesting part of the data.

*Alternative: drop `c=wch` from snapshot events* to buy 6 characters. Rejected by prior experience; it was removed once and restored because it made querying awkward.

### Asynchronous completion event, existing flush budget

The completion event goes out through the existing `eventSink`, which registers with the `WaitGroup` synchronously and then spawns a goroutine, so `Flush(500ms)` will wait for it. The 500ms budget against a 3-second HTTP client timeout means a slow link can drop the final event.

Accepted. The consequence is that a missing `st=com` means "probably ended abnormally" rather than "definitely," which is documented rather than engineered away. Firing synchronously would add up to 3 seconds of exit latency; raising the global flush timeout would slow every command including fast ones like `version`.

## Risks / Trade-offs

- **User-Agent worst case sits exactly at the 64-character limit, with zero slack** → Unchanged from today, so not a regression, but a two-digit patch number in a snapshot build (from v1.10.9 onward) overflows it. Mitigated only partially: a unit test pins the 11-character version budget so header growth fails loudly, but it cannot catch growth in the version string itself. Recommended follow-up outside this change.
- **A dropped `st=com` looks like a crashed session** → Documented in the spec as a probabilistic signal. The timing data in the `st=snp` events is unaffected, since those were sent earlier and independently.
- **Event volume rises from 1 to as many as 9 per watch session** → Bounded by the eight signal types. Non-TTY sessions have no user-exit path and always run the full 10 minutes, so CI and piped installs sit at the upper end. Judged acceptable against the value of the data.
- **Data race on `FirstDataMs` between the emit goroutine and the next `trackWatchSignals` write** → `snapEvent` already does `maps.Clone` before the goroutine is spawned, and that is the only thing preventing the race. With N goroutines per session it matters more and it is exactly the invariant a refactor drops silently. Called out as a task, and the clone must stay inside `snapEvent` rather than moving to the call site.
- **Existing queries on `step == "completed"` silently return watch completions with no timing data** → They will not error, they will return rows with no signal properties, which is worse than failing. Migration is listed below and needs to be communicated to whoever owns the dashboards.
- **Aggregated numbers over-read as precise** → `hosts: 1` means "already present on the first poll," not "took one second." See the resolution note under Non-Goals; whoever builds the dashboards needs to know this, because the spec deliberately carries only the encoding rule, not the interpretation caveat.

## Migration Plan

1. Land the code and spec changes together. No persisted state, no schema migration, no feature flag.
2. Update any dashboard, notebook, or saved DQL that reads watch signal timings: `filter step == "completed"` becomes `filter step == "snapshot"`, and for a per-session tally take the last snapshot per `executionId`.
3. Update `test/integration/grail/events.go` call sites that assert on watch timing events to use the new step name.
4. After the release, both shapes coexist in the tenant's event history: older executions carry timings on `step == "completed"`, newer ones on `step == "snapshot"`. Queries spanning the boundary need to union both, or be scoped to one side of the release.

**Rollback:** revert the commit. Single-event behavior returns. Already-ingested `st=snp` events stay queryable and correct, because each is a self-contained cumulative snapshot rather than a delta requiring its siblings.

## Open Questions

None blocking. Two things deliberately deferred:

- Whether `st=com` should eventually carry `watch.duration_s`, mirroring the existing `install.duration_s` property. Out of scope now, and cheap to add later: the data is already in `WatchSessionResult.Duration` and `onComplete` already receives it, so it would be a change to `buildWatchEventCallback` alone.
- Whether watch should install a SIGINT handler so in-flight events survive Ctrl+C. Would reduce the false-crash rate, but it is a broader change to process lifecycle than this feature warrants.
