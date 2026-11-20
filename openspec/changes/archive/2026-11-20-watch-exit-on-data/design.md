# Design

## Context

Every watch, standalone or after an install, runs through the private `watchIngest()` loop in `pkg/installer/ingest_watch.go`. Around 20 call sites in `cmd/` and the cloud installers reach it through 11 exported wrappers (`WatchIngest`, `WatchIngestWithEvent`, `WatchIngestOtel...`, `WatchIngestCloud...`, `WatchIngestAWS...`, `WatchIngestWithStatus`). Five of them have no callers. `watchIngest()` takes nine positional parameters, so wrappers read like `nil, "", false, "", nil, nil`. `dtwiz watch` calls `WatchIngestWithEvent` from `cmd/watch.go`.

Each loop iteration does: wait for a tick or Enter, check the 10-minute timeout (prompt on a TTY, exit on a non-TTY), poll, record first-data timings, fire the snapshot callback in a goroutine (`go onSnapshot(snap)`), render the frame.

`WatchSessionResult.ExitReason` already documents `"first_data"` as a value, but nothing sets it today.

Self-monitoring sends register with `selfmonitoring.TrackSend()` inside `eventSink`, and `cmd/root.go` waits for them with `selfmonitoring.Flush(500ms)` before exit.

## Goals / Non-Goals

**Goals:**

- Make exit-on-data reachable only from `dtwiz watch`, so post-install watches cannot pick it up by accident.
- Keep the change inside the existing loop rather than adding a second loop.

**Non-Goals:**

- Changing exit codes. Every end path keeps exiting `0`.
- Making the 10-minute timeout configurable.
- Any change to `--yes` handling in `install`, `update`, `uninstall` or `setup`.

## Decisions

### 1. A session option plus one new wrapper, not a package-level switch

Exit-on-data is a per-session setting, `exitOnData`. Only one exported entry point sets it: `WatchIngestStandalone(envURL, pToken, fromClause string, exitOnData bool, onSnapshot, onComplete)`, called from `cmd/watch.go`. Every other wrapper leaves it off.

Alternative considered: a package-level variable like `installer.AutoConfirm`. Rejected, because global state is exactly what would let the behaviour leak into post-install watches, which the specs forbid.

### 2. A private options struct for the watch session

Adding `exitOnData` as a tenth positional parameter would make the wrapper calls unreadable (`nil, "", false, "", false, nil, nil`). Instead, the private `watchIngest()` takes one `watchOptions` struct. Each field is documented, and its zero value means "off": no status line, no AWS account scope, the QuickStart footer, no manual-language call to action, no exit on data, no callbacks. Each wrapper sets only the fields it needs, by name.

The struct stays unexported, and the exported wrapper signatures do not change, so no call site in `cmd/` or the method packages changes. The five wrappers without callers (`WatchIngestOtel`, `WatchIngestWithStatus`, `WatchIngestAWS`, `WatchIngestCloud`, `WatchIngestCloudFromTime`) are deleted rather than ported. Their doc comments move to the remaining wrappers that described themselves in terms of them.

This is a pure refactor and lands in its own commit, separate from the exit-on-data logic.

Alternatives considered:

- An exported `WatchOptions` with a single `WatchIngest(WatchOptions)` replacing all wrappers. Rejected for this change: it touches about 20 call sites across `cmd/` and the aws, azure, gcp and otel installers without making them shorter, because named fields are longer than today's positional calls.
- A `cmd` helper for the 10 identical post-install `WatchIngestWithEvent(...)` calls. That is what would shorten the call sites, but it is unrelated to exit-on-data. It is left for a follow-up.

### 3. Check for data after rendering, not before

In exit-on-data mode, after the frame is rendered, return with `ExitReason = "first_data"` when `len(result.FirstDataMs) > 0`. Checking after rendering leaves the frame that found the data on screen. Because the first poll runs immediately, data that already exists (for example with `--from now()-1h`) ends the watch without waiting for a tick.

`FirstDataMs` is the right signal: it is filled by `trackWatchSignals` from the same eight sections the display shows, and it is what the snapshot events report, so "the watch ended on data" and "a snapshot was sent" always agree.

### 4. Drop the "Press Enter" hint only on the final frame

While waiting, the frame keeps "Press Enter to continue or keep watching...", because Enter still ends the watch. The frame that ends the watch leaves that line out, since nothing is waiting for input any more. The loop decides this per frame (`finalFrame`) while building the footer, rather than having a second footer.

### 5. Skip the timeout prompt in exit-on-data mode

At the 10-minute check, treat exit-on-data mode like non-TTY: set `ExitReason = "timeout"` and return. The `isTTY` input goroutine still runs so Enter keeps working before the timeout.

### 6. Send the final snapshot synchronously

Today the snapshot callback runs as `go onSnapshot(snap)`. `TrackSend()` is only called inside that goroutine, so if the loop returns right after, `Flush` in `cmd/root.go` can run before the send is registered and the process exits without it. On the exit-on-data path, call `onSnapshot(snap)` directly before returning. That is cheap: `eventSink` only calls `TrackSend()` and starts its own goroutine, so the call does not block on the network. The other paths keep the asynchronous call, so the display loop is never delayed.

Alternative considered: call `TrackSend()` before spawning the goroutine in the watch loop. Rejected, because it would make `pkg/installer` depend on `pkg/selfmonitoring` internals.

## Risks / Trade-offs

- [The first data can be a single, early signal, for example one host before any services] → This is intended: the flag answers "is anything arriving?". Users who want the full picture run `dtwiz watch` without the flag.
- [The 10-minute timeout is a constant, so the timeout path is slow to test] → Turn `watchPollInterval` and `watchTimeout` from constants into package variables that in-package tests can shorten.
- [Moving `watchIngest()` to an options struct touches every wrapper and the loop body] → All wrappers are in one file and only forward arguments, the exported signatures stay the same, and the existing loop tests cover the result; the diff is mechanical.
- [Deleting the unused wrappers could break a caller outside this repository] → dtwiz is a CLI, not a library, and `pkg/installer` has no external consumers.

## Migration Plan

None. The flag is opt-in, and without it every call path behaves as before. Rollback is removing the flag and the new wrapper. The options struct can stay, since it does not change behaviour.
