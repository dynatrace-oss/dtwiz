# Tasks: Install Outcome Events

## 1. Event helpers

- [x] 1.1 Add `fireInstallReportEvent(cmd, sub, err)` to `cmd/selfmonitoring.go`: build `ist` params, override `Sub` when `sub` is non-empty, call `applyInstallOutcomes`, and send only when `p.Install != nil`; never set `Err` or `ExtraProps`
- [x] 1.2 Add `fireInstallEvents(cmd, err)` to `cmd/selfmonitoring.go`: call `fireInstallReportEvent(cmd, "", err)`, then send `com` when `err == nil`, `can` via `fireSelfMonitoringEventWithError` when `errors.Is(err, installer.ErrInstallCancelled)`, and `fai` via `fireSelfMonitoringEventWithError` otherwise
- [x] 1.3 Document on both helpers which events they send and that pre-install failures must not call them

## 2. Direct install

- [x] 2.1 Replace every `fireInstallEvent(cmd, installErr)` call in `cmd/install.go` with `fireInstallEvents(cmd, installErr)` for `oneagent`, `kubernetes`, `docker`, `otel`, `otel-collector`, `otel-python`, `otel-node`, `otel-java`, `aws`, `aws-lambda`, `azure`, `gcp`, `demo`
- [x] 2.2 Confirm the pre-install error paths in `cmd/install.go` still send only `fai`

## 3. Setup

- [x] 3.1 Replace `fireSetupInstallEvent(cmd, method, err)` in `cmd/setup.go` (method path and demo path) with `fireInstallReportEvent(cmd, method, err)`; leave the `can`/`fai`/`com` branches unchanged
- [x] 3.2 Remove `fireInstallEvent` and `fireSetupInstallEvent` from `cmd/selfmonitoring.go` once no callers remain

## 4. Tests (`cmd/selfmonitoring_test.go`, using `captureInstallEvents`)

- [x] 4.1 `fireInstallReportEvent`: work started and success sends one `ist` with report and no `Err`/`ExtraProps`
- [x] 4.2 `fireInstallReportEvent`: work started and failure (`DependencyMissingError`) sends one `ist` with report and no `Err`/`ExtraProps`
- [x] 4.3 `fireInstallReportEvent`: no work started (dry-run, failure before confirmation) sends nothing
- [x] 4.4 `fireInstallReportEvent`: `ErrInstallCancelled` sends nothing, even when the timer started
- [x] 4.5 `fireInstallReportEvent` with a non-empty `sub` sets the method as subcommand
- [x] 4.6 `fireInstallEvents`: success with work sends `ist` + `com`; dry-run sends only `com`
- [x] 4.7 `fireInstallEvents`: cancellation sends only `can` with error `user_cancelled`
- [x] 4.8 `fireInstallEvents`: failure after work sends `ist` + `fai` with the classified error and attributes (e.g. `dependency.name`); failure before work sends only `fai`
- [x] 4.9 Replace the existing `TestInstallEvent_*` and `TestSetupInstallEvent_*` tests with the cases above; keep the report-content assertions (outcomes, work time)

## 5. Spec, changelog, verification

- [x] 5.1 Add a `Changed` entry under `[Unreleased]` in `CHANGELOG.md`: `ist` events no longer carry errors and are sent only when install work started; direct install runs now end with a `completed`, `failed`, or `cancelled` event
- [x] 5.2 Run `make test` (`./pkg/...`), `go test ./cmd`, and `make lint`
