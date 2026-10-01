# Proposal

## Why

`dtwiz watch` records time-to-first-data for eight signal types, but emits its self-monitoring event exactly once: on the first poll cycle where *any* signal has data. Everything that arrives later is tracked in memory and then discarded. A session where hosts appear at 1s and services at 8s reports only the hosts, so the `t=` vector is not "time to first data per signal type" but "time to first data for whichever signal won the first race, plus zeros."

That makes the central question unanswerable: which kinds of data show up faster than others, and how long does each take to appear in a tenant.

## What Changes

- **BREAKING (wire format):** introduce a new step shortcode `st=snp` (`step: "snapshot"` in the event body) for watch signal snapshots. Watch no longer reports signal timings under `st=com`. Existing queries filtering `step == "completed"` for watch timing data must move to `step == "snapshot"`.
- Emit one `st=snp` event each time the set of seen signals grows, instead of once for the whole session. Each event carries a **cumulative** snapshot: once a signal's first-data time is recorded it never changes, so later events repeat earlier values and add new ones. The last `st=snp` of a session is the complete tally.
- Emit a dedicated `st=com` event on every watch exit path, with no `t=` field and no signal properties, matching how `analyze`, `status`, `recommend`, `update`, `uninstall`, and `version` already emit completion. This makes session bracketing explicit: `inv` + `com` with no `snp` means the session ran and nothing appeared; `snp` without `com` means the process died with timing data still valid.
- Remove the conditional "fire an empty event on timeout" branch and the `eventFired` one-shot latch, both made redundant by the unconditional `st=com`.
- Measure signal timings from session start rather than from the continuation-reset clock, so a session extended past the 10-minute prompt no longer reports post-continuation signals against a fresh epoch.
- Clamp each per-signal value in the `t=` header to 999 seconds. The header is the fallback telemetry channel with a 64-character HAProxy capture limit; the event body keeps full millisecond precision and remains the query surface.
- Rename `installer.OnWatchComplete` to reflect that it now fires repeatedly per session.

Signal set, `t=` positional CSV encoding, body property names, and the 5-second poll interval all stay exactly as they are. Signals that become visible within the same poll cycle coalesce into a single event.

## Capabilities

### New Capabilities

None. This changes the behavior of an existing capability.

### Modified Capabilities

- `selfmonitoring-watch`: the "emits two self-monitoring events per invocation" requirement becomes N+2 events (`inv`, zero or more `snp`, one `com`); signal timings move from `st=com` to the new `st=snp` step; `st=com` becomes unconditional rather than timeout-gated; the `t=` encoding gains a 999-second clamp and a documented session-start epoch.

## Impact

**Code:**

- `pkg/installer/ingest_watch.go` — emit watermark replaces the `eventFired` latch; deferred `st=com` fire; timeout-branch fire sites removed; `trackWatchSignals` epoch switched to `sessionStart`.
- `pkg/installer/installer.go` — `OnWatchComplete` renamed (6 assignment sites across `cmd/install.go`, `cmd/setup.go`, `cmd/update.go`; 5 read sites across the `aws`, `azure`, and `gcp` installers).
- `pkg/selfmonitoring/selfmonitoring.go` — new `StepSnapshot` constant and `stepFullNames` entry.
- `cmd/selfmonitoring.go` — `buildWatchEventCallback` branches on `ExitReason` to emit `snp` versus `com`; `watchSignalCSV` gains the clamp.

**Data consumers:** dashboards, notebooks, and DQL that read watch timing data from `step == "completed"` need to move to `step == "snapshot"`. `test/integration/grail/events.go` filters on the full step name and will need the new value for watch assertions.

**Event volume:** up to 8 `snp` events plus 1 `com` per watch session, against 1 today. Non-TTY sessions (CI, piped installs) have no user-exit path and always run to the 10-minute timeout, so they sit at the upper end.

**Not in scope:** the unused `WatchSessionResult.Duration` and the ignored return values on the `WatchIngest*` functions stay as they are. `Duration` does not reach the event payload.

**Feature flag:** none. Self-monitoring is already unconditional for these commands; this changes what is sent, not whether.

**Rollback:** the change is confined to emission logic with no persisted state. Reverting restores single-event behavior; already-ingested `st=snp` events remain queryable and self-describing, since each is a complete cumulative snapshot rather than a delta that needs its siblings to interpret.
