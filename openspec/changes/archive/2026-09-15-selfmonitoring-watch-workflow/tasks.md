# Tasks

## 1. selfmonitoring package extensions (`pkg/selfmonitoring/selfmonitoring.go`)

- [x] 1.1 Add `StepCompleted = "com"` constant alongside `StepInvoked = "inv"`.
- [x] 1.2 Derive event title dynamically from `CmdID` and `SubID` instead of hardcoding `"dtwiz started"`.
- [x] 1.3 Make all User-Agent fields conditional on non-empty (including `c=`) so `st=com` for watch omits `c=wch` naturally.
- [x] 1.4 Add pre-request debug log showing URL, User-Agent, Tab-Id, and body.

## 2. Watch session result tracking (`pkg/installer/ingest_watch.go`)

- [x] 2.1 Define `WatchSessionResult` struct with `ExitReason string` and `FirstDataMs map[string]int64`.
- [x] 2.2 Add `onEvent func(WatchSessionResult)` parameter to internal `watchIngest`; add `WatchIngestWithEvent` public wrapper that passes the callback.
- [x] 2.3 Set `ExitReason = "user_exit"` or `"timeout"` at each `return` point in the watch loop.
- [x] 2.4 Add `trackWatchSignals` helper: called after every `pollAll` cycle, records `time.Since(watchStart).Milliseconds()` the first time each of the 8 signals transitions from absent to present (`svc`, `hst`, `cld`, `k8s`, `rel`, `log`, `req`, `exc`). Logs and requests are detected from `Count > 0 OR Status != ""` to catch the probe-phase transition.
- [x] 2.5 Fire `onEvent` asynchronously (goroutine) at first data; fire synchronously at both timeout return paths. Use `eventFired bool` guard (set before goroutine spawn) to prevent double-firing.

## 3. Watch command wiring (`cmd/watch.go`)

- [x] 3.1 Call `installer.WatchIngestWithEvent` instead of `WatchIngest`; pass a callback that builds `EventParams` with `StepCompleted`, clears `CmdID` (drops `c=wch`), encodes signal timing via `watchSignalCSV`, and calls `fireSelfMonitoringEvent`.
- [x] 3.2 Add `watchSignalOrder` and `watchSignalCSV`: encodes `FirstDataMs` as a positional comma-separated list of whole-second values in fixed order (`cld,exc,hst,k8s,log,rel,req,svc`), with `0` for absent signals; returns `""` when no signals seen (causing `t=` to be omitted from the header).

## 4. Tests

- [x] 4.1 Unit-test `trackWatchSignals`: verify first-data timestamps are recorded on first appearance and not overwritten on subsequent polls; cover the `Status != ""` probe-transition case for logs and requests.
- [x] 4.2 Unit-test `watchSignalCSV`: verify positional encoding, correct whole-second truncation, `0` for absent signals, and empty return for nil map.
- [x] 4.3 Unit-test `WatchSessionResult` zero value: verify that ranging over a nil `FirstDataMs` map (early-return path when `pToken == ""`) does not panic.
- [x] 4.4 Run `go test ./pkg/installer/... ./pkg/selfmonitoring/... ./cmd/...` and confirm all pass.
