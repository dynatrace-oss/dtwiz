# Tasks

## 1. selfmonitoring package extensions (`pkg/selfmonitoring/selfmonitoring.go`)

- [x] 1.1 Add `StepCompleted = "com"` constant alongside `StepInvoked = "inv"`.
- [x] 1.2 Add `ExtraProps map[string]string` field to `EventParams`; merge into event body properties via `maps.Copy` (excluded from User-Agent and Tab-Id header construction).
- [x] 1.3 Derive event title dynamically from `CmdID` and `SubID` instead of hardcoding `"dtwiz started"`.
- [x] 1.4 Add pre-request debug log showing URL, User-Agent, Tab-Id, and body.

## 2. Watch session result tracking (`pkg/installer/ingest_watch.go`)

- [x] 2.1 Define `WatchSessionResult` struct with `Duration time.Duration`, `ExitReason string`, and `FirstDataMs map[string]int64`.
- [x] 2.2 Change `watchIngest` to a named return of `WatchSessionResult`; use `defer` to capture final duration at every exit path.
- [x] 2.3 Set `ExitReason = "user_exit"` or `"timeout"` at each `return` point in the watch loop.
- [x] 2.4 Add `trackWatchSignals` helper: called after every `pollAll` cycle, records `time.Since(watchStart).Milliseconds()` the first time each of the 8 signals transitions from absent to present (`svc`, `hst`, `cld`, `k8s`, `rel`, `log`, `req`, `exc`). Logs and requests are detected from `Count > 0 OR Status != ""` to catch the probe-phase transition.
- [x] 2.5 Change `WatchIngest` to return `WatchSessionResult`; existing callers that discard the result require no changes.

## 3. Watch command wiring (`cmd/watch.go`)

- [x] 3.1 Add `PersistentPreRun` override on `watchCmd` that reproduces `logger.Init` and `featureflags.ApplyCLIOverrides` without calling `fireSelfMonitoringEvent`, suppressing the automatic `StepInvoked` event so watch has exactly one span.
- [x] 3.2 Capture `WatchSessionResult` from `WatchIngest` and pass it to `fireSelfMonitoringWatchComplete`.

## 4. Watch completion event flush (`cmd/root.go`)

- [x] 4.1 Add `fireSelfMonitoringWatchComplete`: checks feature flag (with debug log on skip), builds params with `StepCompleted`, encodes session result via `watchResultToProps`, fires event in a goroutine, and blocks up to 500ms for the flush to complete.
- [x] 4.2 Add `watchResultToProps`: encodes `watch.dur` (ms), `watch.exit` (exit reason), `watch.sig` (sorted CSV of signals seen), and `watch.t_<signal>` (ms to first data) into a `map[string]string` for `ExtraProps`.
- [x] 4.3 Add debug logs for: feature flag skipped, successful send, send failure, and flush timeout.

## 5. Tests

- [x] 5.1 Unit-test `trackWatchSignals`: verify first-data timestamps are recorded on first appearance and not overwritten on subsequent polls; cover the `Status != ""` probe-transition case for logs and requests.
- [x] 5.2 Unit-test `watchResultToProps`: verify correct encoding of duration, exit reason, signal CSV, and per-signal timing fields; verify signals absent from `FirstDataMs` produce no `watch.t_*` keys.
- [x] 5.3 Unit-test `WatchSessionResult` zero value: verify that ranging over a nil `FirstDataMs` map (early-return path when `pToken == ""`) does not panic.
- [x] 5.4 Run `go test ./pkg/installer/... ./pkg/selfmonitoring/... ./cmd/...` and confirm all pass.
