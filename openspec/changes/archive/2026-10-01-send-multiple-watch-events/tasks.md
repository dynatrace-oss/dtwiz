# Tasks

## 1. Add the snapshot step to the self-monitoring package

- [x] 1.1 Add `StepSnapshot = "snp"` to the step constants in `pkg/selfmonitoring/selfmonitoring.go`, alongside the existing `StepInvoked`/`StepCompleted` block
- [x] 1.2 Add `StepSnapshot: "snapshot"` to the `stepFullNames` map so the event body carries the unabbreviated step name
- [x] 1.3 Add a test in `pkg/selfmonitoring/selfmonitoring_test.go` asserting `buildEventProps` emits `step: "snapshot"` for `StepSnapshot`, and that `buildUserAgent` emits `;st=snp`

## 2. Split the watch callback into snapshot and completion

- [x] 2.1 In `pkg/installer/ingest_watch.go`, replace the single `onEvent func(WatchSessionResult)` parameter on `watchIngest` with `onSnapshot` and `onComplete`, both `func(WatchSessionResult)`
- [x] 2.2 Thread both parameters through the 5 `*WithEvent` wrappers: `WatchIngestWithEvent`, `WatchIngestOtelWithEvent`, `WatchIngestCloudWithEvent`, `WatchIngestCloudFromTimeWithEvent`, `WatchIngestAWSWithEvent`
- [x] 2.3 Update the 6 non-event variants (`WatchIngest`, `WatchIngestOtel`, `WatchIngestCloud`, `WatchIngestCloudFromTime`, `WatchIngestWithStatus`, `WatchIngestAWS`) to pass `nil, nil` to `watchIngest`
- [x] 2.4 Add `installer.OnWatchSnapshot` beside the existing `installer.OnWatchComplete` in `pkg/installer/installer.go`; keep `OnWatchComplete`'s name and update its doc comment, and document that `OnWatchSnapshot` fires once per newly seen signal type
- [x] 2.5 Update the 6 package-var assignment sites to set both vars: `cmd/install.go:350`, `cmd/install.go:409`, `cmd/install.go:433`, `cmd/setup.go:170`, `cmd/update.go:90`, `cmd/update.go:119`
- [x] 2.6 Update the 5 cloud-installer sites to forward both vars: `pkg/installer/aws/install.go:278`, `pkg/installer/azure/install.go:183`, `pkg/installer/azure/update.go:93`, `pkg/installer/gcp/install.go:239`, `pkg/installer/gcp/update.go:88`
- [x] 2.7 Update the 13 direct `cmd/` call sites to pass both builders: `cmd/watch.go:37`, `cmd/setup.go:131`, `cmd/setup.go:228`, `cmd/setup.go:230`, and `cmd/install.go` lines 104, 141, 173, 204, 235, 268, 329, 388, 471
- [x] 2.8 Run `make build` to confirm every call site was updated

## 3. Fix the timing epoch

- [x] 3.1 In `pkg/installer/ingest_watch.go`, change the `trackWatchSignals(&result, watchStart, state)` call to pass `sessionStart`, leaving the display `elapsed` calculation on `watchStart`
- [x] 3.2 Repoint the deferred `result.Duration = time.Since(watchStart)` assignment at `sessionStart` so the field is not a landmine for future readers
- [x] 3.3 Add a test in `pkg/installer/ingest_watch_test.go` verifying `trackWatchSignals` records a value greater than 600000 ms when passed a start time more than 10 minutes in the past (covers the "session extended past the timeout prompt" scenario)

## 4. Replace the one-shot latch with a change watermark

- [x] 4.1 Replace the `eventFired bool` with an `emittedCount int` watermark in `watchIngest`
- [x] 4.2 Change the emit condition to fire `onSnapshot` when `len(result.FirstDataMs) > emittedCount`, updating `emittedCount` to the new length
- [x] 4.3 Keep the `maps.Clone(result.FirstDataMs)` inside `snapEvent`, before the goroutine is spawned; do not move the clone to the call site (it is the only guard against a race with the next `trackWatchSignals` write)
- [x] 4.4 Delete the two conditional `onEvent(snapEvent("timeout"))` call sites in the timeout branch (`pkg/installer/ingest_watch.go:296` and `:305`), keeping the surrounding `result.ExitReason` assignments and returns

## 5. Emit the completion event on every exit path

- [x] 5.1 Add a `defer` in `watchIngest`, placed after the platform-token guard and after `result.FirstDataMs` is initialized, that calls `onComplete(snapEvent(result.ExitReason))` when `onComplete` is non-nil
- [x] 5.2 Verify by inspection that all four real exit paths (standard input error, Enter pressed, non-TTY timeout, TTY timeout declined) are covered by the defer, and that the token guard returns before the defer is registered
- [x] 5.3 Add a test asserting the token guard path (`pToken == ""`) fires neither callback

## 6. Build the two event callbacks

- [x] 6.1 In `cmd/selfmonitoring.go`, strip the `Type` and `ExtraProps` assignments from `buildWatchEventCallback` so it emits a bare `StepCompleted` event, and update its doc comment to say it handles session end
- [x] 6.2 Add `buildWatchSnapshotEventCallback(cmd *cobra.Command) func(installer.WatchSessionResult)` emitting `selfmonitoring.StepSnapshot` with `Type` from `watchSignalCSV` and `ExtraProps` from `watchSignalProps`
- [x] 6.3 Keep `params.Cmd = "watch"` and `params.Sub = ""` in both builders so `c=wch` is present on every watch event
- [x] 6.4 Add the 999-second clamp to `watchSignalCSV`, leaving `watchSignalProps` emitting unclamped milliseconds

## 7. Tests for the new emission behavior

- [x] 7.1 Convert `cmd/root_test.go:86` (`TestBuildWatchEventCallback_AlwaysEmitsWatchCmd`) from a single `captured` variable to a slice, and cover both builders so the `Cmd == "watch"` / `Sub == ""` assertion holds for each
- [x] 7.2 Test that `buildWatchSnapshotEventCallback` produces `StepID == StepSnapshot` with a non-empty `Type` and populated `ExtraProps`
- [x] 7.3 Test that `buildWatchEventCallback` produces `StepID == StepCompleted` with an empty `Type` and nil `ExtraProps`, even when the passed `WatchSessionResult` has a non-empty `FirstDataMs`
- [x] 7.4 Test `watchSignalCSV` clamping: a signal at 1500000 ms encodes as `999`, and the matching `watchSignalProps` entry stays `1500000`
- [x] 7.5 Test the worst-case User-Agent budget: build params with all 8 signals clamped to 999 and an empty version, then assert the leftover budget for the version string is at least 11 characters (the length goreleaser's snapshot template produces today). Asserting a 12-character version fits would fail: measured overhead is 53 chars, so 11 is the exact ceiling
- [x] 7.6 Test that an unchanged signal set produces no further snapshot (covers the "signal type that was already seen reports data again" scenario)
- [x] 7.7 Test that two signal types appearing in one poll cycle coalesce into a single snapshot containing both

## 8. Update the integration test helper

- [x] 8.1 Update `test/integration/grail/events.go` doc comment on `SelfMonitoringQuery.Step` to include `"snapshot"` in the list of valid full step names
- [x] 8.2 No change needed: the only `Step:` filter in the test suite is `"invoked"` (`test/e2e/selfmonitoring_test.go:100`), and no test asserts watch timing data

## 9. Verify

- [x] 9.1 Run `make test` and confirm all tests pass
- [x] 9.2 Run `make lint` and confirm no new findings
- [x] 9.3 Manual check against a real tenant: run `dtwiz watch --debug` and confirm from the debug log that multiple `st=snp` events are emitted with growing `t=` vectors, followed by exactly one `st=com` with no `t=`, and that the Events API accepts all of them (no dedup or rate-limit rejection on the repeated `"dtwiz watch"` title)
- [x] 9.4 Add a `CHANGELOG.md` entry under `[Unreleased]`, noting the `step == "completed"` to `step == "snapshot"` query migration for watch timing data
