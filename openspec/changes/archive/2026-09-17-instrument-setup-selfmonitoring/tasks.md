# Tasks: Instrument Setup Self-Monitoring

## 1. Extend StepID constants

- [x] 1.1 Add `StepAnalyze = "ana"`, `StepRecommend = "rec"`, and `StepInstall = "ist"` constants to `pkg/selfmonitoring/selfmonitoring.go` alongside the existing `StepInvoked` and `StepCompleted`

## 2. Extend normSubMap for update variants

- [x] 2.1 Add `"otel-update": "otlu"`, `"azure-update": "azu"`, `"gcp-update": "gcpu"` to `normSubMap` in `cmd/selfmonitoring.go`

## 3. Instrument cmd/setup.go

- [x] 3.1 Fire `inv` event at the top of the `RunE` body, before `printBanner()`, using `buildEventParams(cmd, selfmonitoring.StepInvoked)`
- [x] 3.2 Fire `ana` event immediately after `analyzeSystem()` returns; set `Err = "err"` if the call returned an error (fire before returning the error to the caller)
- [x] 3.3 Fire `rec` event after the user enters a valid numeric selection, with `SubID` overridden to `normSub(string(selected.Method))`
- [x] 3.4 Fire `rec` event with `SubID = "uni"` when the user enters `u` (view uninstall help), before returning
- [x] 3.5 Fire `rec` event with `SubID = "demo"` when the user enters `d` (demo path), before `getDtEnvironment()` is called
- [x] 3.6 Fire `ist` event after the demo installer returns (`otel.InstallDemo`); set `Err = "err"` on failure, `SubID = "demo"`
- [x] 3.7 Fire `ist` event after the main install switch returns; set `Err = "err"` on non-cancellation failure, `SubID = normSub(string(selected.Method))`; do not fire if `installErr` is `ErrInstallCancelled` or `otel.ErrUpToDate`

## 4. Tests

- [x] 4.1 Add unit tests to `pkg/selfmonitoring` (or its existing test file) verifying `StepAnalyze`, `StepRecommend`, and `StepInstall` constant values are `"ana"`, `"rec"`, and `"ist"` respectively
- [x] 4.2 Add unit tests for `normSub` in `cmd/` verifying the three new update-variant entries: `"otel-update"` → `"otlu"`, `"azure-update"` → `"azu"`, `"gcp-update"` → `"gcpu"`
- [x] 4.3 Verify build passes: `make build`
- [x] 4.4 Verify all tests pass: `make test`
- [x] 4.5 Verify no new lint issues: `make lint`
