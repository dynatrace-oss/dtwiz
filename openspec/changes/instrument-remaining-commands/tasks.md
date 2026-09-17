# Tasks: Instrument Remaining Commands

## 1. Add fireCompletedEvent helper

- [ ] 1.1 Add `fireCompletedEvent(cmd *cobra.Command, err error)` to `cmd/selfmonitoring.go`: build params with `buildEventParams(cmd, selfmonitoring.StepCompleted)`, set `Err="err"` when `err != nil`, call `fireSelfMonitoringEvent(params)`

## 2. Instrument informational commands

- [ ] 2.1 In `cmd/analyze.go`: call `fireCompletedEvent(cmd, err)` at the end of `RunE`, before returning (pass the error from `analyzeSystem()` or `enc.Encode()`)
- [ ] 2.2 In `cmd/recommend.go`: call `fireCompletedEvent(cmd, err)` at the end of `RunE`, before returning (pass the error from `analyzeSystem()` or `enc.Encode()`)
- [ ] 2.3 In `cmd/status.go`: call `fireCompletedEvent(cmd, err)` at the end of `RunE`, before returning (pass the error returned by `RunE`)
- [ ] 2.4 In `cmd/version.go`: call `fireCompletedEvent(cmd, nil)` at the end of `Run` (version cannot fail)

## 3. Tests

- [ ] 3.1 Add a unit test for `fireCompletedEvent` in `cmd/` verifying that `Err` is set to `"err"` when a non-nil error is passed and is empty when `nil` is passed
- [ ] 3.2 Add unit tests verifying `normCmd` returns `"ana"` for `"analyze"`, `"rec"` for `"recommend"`, `"sta"` for `"status"`, `"ver"` for `"version"` (confirms CmdID distinguishability)
- [ ] 3.3 Verify build passes: `make build`
- [ ] 3.4 Verify all tests pass: `make test`
- [ ] 3.5 Verify no new lint issues: `make lint`
