## 1. Install telemetry state in `pkg/installer`

- [ ] 1.1 Create `pkg/installer/install_telemetry.go` with the `Feature` type and constants (`FeatureOtelConfig`, `FeatureHostMonitoring`, `FeatureRUM`, `FeatureSynthetic`, `FeatureRDSExtensions`), an `Outcome` type (not tried / succeeded / failed), and the mutex-guarded recorder: `RecordFeature(f, ok)` (last call wins), `FeatureOutcomes()`, `ResetInstallTelemetry()`
- [ ] 1.2 Add the stopwatch to the same file: `StartInstallTimer()`, internal pause intervals, `MarkInstallDone()` (first call wins), `MarkInstallDoneAt(t)`, `InstallWorkTime() (time.Duration, started bool)`. Elapsed time = end − start − pauses that started before the end; the end defaults to the read time when unmarked
- [ ] 1.3 Change `confirmProceed` in `pkg/installer/installer.go`: when the stopwatch isn't running and the user (or `AutoConfirm`) confirms, start it; when it is running, record the prompt time as a pause. Remove `ExecutionStart` and update the doc comments on `ConfirmProceed`/`ShouldProceed`
- [ ] 1.4 Add `ConfirmQuestion(prompt)`, a prompt that never starts the stopwatch but pauses it when running, and switch the "Select another project?" prompts in `pkg/installer/otel/otel.go` and `pkg/installer/otel/nodejs.go` to it
- [ ] 1.5 Unit tests in `pkg/installer/install_telemetry_test.go`: recorder default (not tried), last write wins, reset clears both recorder and stopwatch, concurrent `RecordFeature` (run with `-race`); stopwatch not started reports `started=false`, single pause subtracted, pause after a past `MarkInstallDoneAt` not subtracted, first `MarkInstallDone` wins. Use an injectable clock
- [ ] 1.6 Unit tests for `confirmProceed`: confirm starts, decline does not start, a prompt while running pauses rather than restarts, `AutoConfirm` starts with zero pause, `ConfirmQuestion` never starts

## 2. Record feature outcomes at the source

- [ ] 2.1 `pkg/installer/otel/otel.go` `activateHostMonitoringExtension`: `RecordFeature(FeatureHostMonitoring, false)` at each early return, `true` at the end
- [ ] 2.2 `pkg/installer/otel/collector.go`: record `FeatureOtelConfig` on the result of the config `os.WriteFile`
- [ ] 2.3 `pkg/installer/otel/update.go` `writeConfig`: record `FeatureOtelConfig` on its result (covers both update paths); in `pkg/installer/otel/update_dynatrace.go`, record success when the config is unchanged but tenant prerequisites are applied
- [ ] 2.4 `pkg/installer/oneagent/oneagent.go` `InstallOneAgentV2`: record `FeatureHostMonitoring` on the result of `ExecuteInstallCommand` only (dry-run, print-endpoints and connectivity-check-only leave it not tried)
- [ ] 2.5 `pkg/installer/docker.go` `InstallDocker`: record `FeatureHostMonitoring` on the result of `docker run`
- [ ] 2.6 Tests using the existing stubs (`activateHostMonitoringExtensionFn` fakes, `newExtensionManagerFn`, command runners): host monitoring success, failure at each step, already-active counts as success; config write success/failure; OneAgent connectivity-only leaves host monitoring not tried

## 3. Mark end of install work

- [ ] 3.1 `pkg/installer/azure/install.go`, `pkg/installer/azure/update.go`, `pkg/installer/gcp/install.go`, `pkg/installer/gcp/update.go`: call `MarkInstallDone()` immediately before `WatchIngestCloudFromTimeWithEvent`
- [ ] 3.2 `pkg/installer/aws/install.go`: record the CloudFormation goroutine's completion time and the Lambda completion time; after `wg.Wait()`, call `MarkInstallDoneAt` with the later of the two
- [ ] 3.3 `pkg/installer/otel/update_dynatrace.go`: call `MarkInstallDone()` immediately before `WatchIngest`
- [ ] 3.4 Add a comment on the `WatchIngest*WithEvent` variants in `pkg/installer/ingest_watch.go` stating that installers running the watch internally must call `MarkInstallDone()` first
- [ ] 3.5 Tests: for Azure and GCP install/update (existing runner and dtclient fakes), the mark is set before the watch starts; for AWS, the end is the later of a fake deploy and Lambda completion, not the watch end

## 4. Encode `f=` and `d=` in `pkg/selfmonitoring`

- [ ] 4.1 Add `Features` and `DurationS` to `EventParams` in `pkg/selfmonitoring/selfmonitoring.go`; append `f=` and `d=` in `buildUserAgent` after the existing pairs, omitted when empty; add the property constants
- [ ] 4.2 Tests in `pkg/selfmonitoring/selfmonitoring_test.go`: `f=`/`d=` present and ordered, omitted when empty, not copied into the body by `buildEventProps`; worst-case `ist` User-Agent (`10.10.10` version, `awsl`, `ifl`, `f=-----`, `d=9999`) ≤ 64 chars

## 5. Build and fire the `ist` event in `cmd`

- [ ] 5.1 In `cmd/selfmonitoring.go`, add `installFeatureOrder` (add-only, documented like `watchSignalOrder`), the full body names, and helpers that build the `f=` string, the five body properties (`succeeded`/`failed`/`not_tried`), `install.durationMs` and the clamped `d=` seconds (cap 9999)
- [ ] 5.2 Add one shared helper that applies duration and features to `EventParams` only when the stopwatch was started; use it in both `fireInstallEvent` and `fireSetupInstallEvent`. Remove `installMethodFeatures` and the `duration time.Duration` parameter
- [ ] 5.3 `cmd/install.go`: replace every `installer.ExecutionStart = …` with `installer.ResetInstallTelemetry()` before the installer; call `installer.StartInstallTimer()` before `oneagent` and `docker` when not in dry-run; remove `installDuration()`
- [ ] 5.4 `cmd/setup.go`: call `installer.ResetInstallTelemetry()` before the selected installer and the demo installer; call `installer.StartInstallTimer()` before OneAgent and Docker when not in dry-run
- [ ] 5.5 Tests in `cmd/selfmonitoring_test.go` (using `eventSink` capture): `f=` string and body for each outcome combination; RUM/Synthetic/RDS always `-`; no `f=`/`d=`/body properties when the stopwatch never started (dry-run, cancellation); duration present on failure; `d=` clamping with the unclamped body value; setup `ist` carries the same properties as direct install; setup still sends no `ist` on cancellation or `ErrUpToDate`
- [ ] 5.6 Update existing tests that assert `install.duration_s`, `install.host_monitoring_enabled` or `install.otel_pipelines`

## 6. Docs and verification

- [ ] 6.1 Add CHANGELOG entries under `[Unreleased]`: **Changed** (body keys replaced: `install.duration_s` → `install.durationMs`, `install.host_monitoring_enabled` → `install.hostMonitoring`, `install.otel_pipelines` removed; new `f=` and `d=` User-Agent fields) and **Fixed** (cloud install duration included the watch; Python virtualenv prompt reset the duration)
- [ ] 6.2 Run `make test` (with `-race` for `pkg/installer`) and `make lint`
- [ ] 6.3 Manual check with `--debug`: run `dtwiz install otel-collector` and `dtwiz setup` → OTel Collector against a test tenant, and confirm the logged User-Agent and body for success, dry-run and cancel
