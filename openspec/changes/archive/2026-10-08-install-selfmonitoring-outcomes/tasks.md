# Tasks: Install Self-Monitoring Outcomes

## 1. Install telemetry state in `pkg/installer`

- [x] 1.1 Create `pkg/installer/install_telemetry.go` with the `Feature` type and constants (`FeatureOtelConfig`, `FeatureHostMonitoring`, `FeatureRUM`, `FeatureSynthetic`, `FeatureRDSExtensions`), an `Outcome` type (not tried / succeeded / failed), and the mutex-guarded recorder: `RecordFeature(f, ok)` (last call wins), `FeatureOutcomes()`, `ResetInstallTelemetry()`
- [x] 1.2 Add the stopwatch to the same file: `StartInstallTimer()`, internal pause intervals, `MarkInstallDone()` (first call wins), `MarkInstallDoneAt(t)`, `InstallWorkTime() (time.Duration, started bool)`. Elapsed time = end − start − pauses that started before the end; the end defaults to the read time when unmarked
- [x] 1.3 Change `confirmProceed` in `pkg/installer/installer.go`: when the stopwatch isn't running and the user (or `AutoConfirm`) confirms, start it; when it is running, record the prompt time as a pause. Remove `ExecutionStart` and update the doc comments on `ConfirmProceed`/`ShouldProceed`
- [x] 1.4 Add `ConfirmQuestion(prompt)`, a prompt that never starts the stopwatch but pauses it when running, and switch the "Select another project?" prompts in `pkg/installer/otel/otel.go` and `pkg/installer/otel/nodejs.go` to it
- [x] 1.5 Unit tests in `pkg/installer/install_telemetry_test.go`: recorder default (not tried), last write wins, reset clears both recorder and stopwatch, concurrent `RecordFeature` (run with `-race`); stopwatch not started reports `started=false`, single pause subtracted, pause after a past `MarkInstallDoneAt` not subtracted, first `MarkInstallDone` wins. Use an injectable clock
- [x] 1.6 Unit tests for `confirmProceed`: confirm starts, decline does not start, a prompt while running pauses rather than restarts, `AutoConfirm` starts with zero pause, `ConfirmQuestion` never starts

## 2. Record feature outcomes at the source

- [x] 2.1 `pkg/installer/otel/otel.go` `activateHostMonitoringExtension`: `RecordFeature(FeatureHostMonitoring, false)` at each early return, `true` at the end
- [x] 2.2 `pkg/installer/otel/collector.go`: record `FeatureOtelConfig` on the result of the config `os.WriteFile`
- [x] 2.3 `pkg/installer/otel/update.go` `writeConfig`: record `FeatureOtelConfig` on its result (covers both update paths); in `pkg/installer/otel/update_dynatrace.go`, record success when the config is unchanged but tenant prerequisites are applied
- [x] 2.4 `pkg/installer/oneagent/oneagent.go` `InstallOneAgentV2`: record `FeatureHostMonitoring` on the result of `ExecuteInstallCommand` only (dry-run, print-endpoints and connectivity-check-only leave it not tried)
- [x] 2.5 `pkg/installer/docker.go` `InstallDocker`: record `FeatureHostMonitoring` on the result of `docker run`
- [x] 2.6 Tests using the existing stubs (`activateHostMonitoringExtensionFn` fakes, `newExtensionManagerFn`, command runners): host monitoring success, failure at each step, already-active counts as success; config write success/failure; OneAgent connectivity-only leaves host monitoring not tried

## 3. Mark end of install work

- [x] 3.1 `pkg/installer/azure/install.go`, `pkg/installer/azure/update.go`, `pkg/installer/gcp/install.go`, `pkg/installer/gcp/update.go`: call `MarkInstallDone()` immediately before `WatchIngestCloudFromTimeWithEvent`
- [x] 3.2 `pkg/installer/aws/install.go`: record the CloudFormation goroutine's completion time and the Lambda completion time; after `wg.Wait()`, call `MarkInstallDoneAt` with the later of the two
- [x] 3.3 `pkg/installer/otel/update_dynatrace.go`: call `MarkInstallDone()` immediately before `WatchIngest`
- [x] 3.4 Add a comment on the `WatchIngest*WithEvent` variants in `pkg/installer/ingest_watch.go` stating that installers running the watch internally must call `MarkInstallDone()` first
- [x] 3.5 Tests: for Azure and GCP install/update (existing runner and dtclient fakes), the mark is set before the watch starts; for AWS, the end is the later of a fake deploy and Lambda completion, not the watch end

## 4. Encode `f=` and `d=` in `pkg/selfmonitoring`

- [x] 4.1 Add the `Install *InstallReport` field to `EventParams` in `pkg/selfmonitoring/selfmonitoring.go`, and `pkg/selfmonitoring/install_outcomes.go` with `InstallReport`, the add-only feature order, and the `f=`, `d=` and body encoders; emit them in the `StepInstall` case of `buildUserAgent` (`useragent.go`) and `buildEventProps` (`eventbody.go`); add the property constants
- [x] 4.2 Tests in `pkg/selfmonitoring/install_outcomes_test.go` (encoding: positions, header/body agreement, clamp, rounding), `useragent_test.go` and `eventbody_test.go` (emitted on `ist` only, omitted without a report, not copied across header and body); worst-case `ist` User-Agent leaves the 11-char version budget

## 5. Build and fire the `ist` event in `cmd`

- [x] 5.1 In `cmd/selfmonitoring.go`, add `applyInstallOutcomes`, which copies `installer.FeatureOutcomes()` and `installer.InstallWorkTime()` into an `InstallReport` (the encoding lives in `pkg/selfmonitoring`, see 4.1)
- [x] 5.2 Use `applyInstallOutcomes` in both `fireInstallEvent` and `fireSetupInstallEvent`, only when the stopwatch was started. Remove `installMethodFeatures` and the `duration time.Duration` parameter
- [x] 5.3 `cmd/install.go`: replace every `installer.ExecutionStart = …` with `installer.ResetInstallTelemetry()` before the installer; call `installer.StartInstallTimer()` before `oneagent` and `docker` when not in dry-run; remove `installDuration()`
- [x] 5.4 `cmd/setup.go`: call `installer.ResetInstallTelemetry()` before the selected installer and the demo installer; call `installer.StartInstallTimer()` before OneAgent and Docker when not in dry-run
- [x] 5.5 Tests in `cmd/selfmonitoring_test.go` (using `eventSink` capture): the report carries the recorded outcomes and work time; no report when the stopwatch never started (dry-run) or on cancellation; the report is kept on failure next to the error attributes; setup carries the same report as direct install; setup still sends no `ist` on cancellation or `ErrUpToDate`
- [x] 5.6 Update existing tests that assert `install.duration_s`, `install.host_monitoring_enabled` or `install.otel_pipelines`

## 6. Docs and verification

- [x] 6.1 Add CHANGELOG entries under `[Unreleased]`: **Changed** (body keys replaced: `install.duration_s` → `install.duration_ms`, `install.host_monitoring_enabled` → `install.host_monitoring`, `install.otel_pipelines` removed; new `f=` and `d=` User-Agent fields) and **Fixed** (cloud install duration included the watch; Python virtualenv prompt reset the duration)
- [x] 6.2 Run `make test` (with `-race` for `pkg/installer`) and `make lint`
- [x] 6.3 Manual check with `--debug`: run `dtwiz install otel-collector` and `dtwiz setup` → OTel Collector against a test tenant, and confirm the logged User-Agent and body for success, dry-run and cancel
