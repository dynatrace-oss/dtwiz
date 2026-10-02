# Tasks: Instrument Install Self-Monitoring

## 1. Installer package — shared state

- [x] 1.1 Add `ExecutionStart time.Time` package-level var to `pkg/installer/installer.go`
- [x] 1.2 Add `OnWatchComplete func(WatchSessionResult)` package-level var to `pkg/installer/installer.go`
- [x] 1.3 Set `ExecutionStart = time.Now()` in `confirmProceed` when the user confirms (interactive and AutoConfirm paths)

## 2. WatchIngest — WithEvent variants for cloud methods

- [x] 2.1 Add `WatchIngestCloudWithEvent` to `pkg/installer/ingest_watch.go`
- [x] 2.2 Add `WatchIngestCloudFromTimeWithEvent` to `pkg/installer/ingest_watch.go`
- [x] 2.3 Add `WatchIngestAWSWithEvent` to `pkg/installer/ingest_watch.go`

## 3. Cloud installers — consume OnWatchComplete

- [x] 3.1 Switch `pkg/installer/aws/install.go` from `WatchIngestAWS` to `WatchIngestAWSWithEvent(..., installer.OnWatchComplete)`
- [x] 3.2 Switch `pkg/installer/azure/install.go` from `WatchIngestCloudFromTime` to `WatchIngestCloudFromTimeWithEvent(..., installer.OnWatchComplete)`
- [x] 3.3 Switch `pkg/installer/azure/update.go` from `WatchIngestCloudFromTime` to `WatchIngestCloudFromTimeWithEvent(..., installer.OnWatchComplete)`
- [x] 3.4 Switch `pkg/installer/gcp/install.go` from `WatchIngestCloudFromTime` to `WatchIngestCloudFromTimeWithEvent(..., installer.OnWatchComplete)`
- [x] 3.5 Switch `pkg/installer/gcp/update.go` from `WatchIngestCloudFromTime` to `WatchIngestCloudFromTimeWithEvent(..., installer.OnWatchComplete)`

## 4. Selfmonitoring helpers

- [x] 4.1 Add `installMethodFeatures(method string) map[string]string` to `cmd/selfmonitoring.go` returning per-method feature flag maps
- [x] 4.2 Add `fireInstallEvent(cmd, duration, err)` to `cmd/selfmonitoring.go` emitting the `ist` event with duration and features

## 5. Install command handlers

- [x] 5.1 Add `installDuration()` helper to `cmd/install.go`
- [x] 5.2 Instrument `install oneagent`: set `ExecutionStart` before call (no confirmation prompt), call `fireInstallEvent`, switch to `WatchIngestWithEvent`
- [x] 5.3 Instrument `install kubernetes`: reset `ExecutionStart`, call `fireInstallEvent`, switch to `WatchIngestWithEvent`
- [x] 5.4 Instrument `install otel`: reset `ExecutionStart`, call `fireInstallEvent`, switch to `WatchIngestOtelWithEvent`
- [x] 5.5 Instrument `install otel-collector`: reset `ExecutionStart`, call `fireInstallEvent`, switch to `WatchIngestWithEvent`
- [x] 5.6 Instrument `install otel-python`: reset `ExecutionStart`, call `fireInstallEvent`, switch to `WatchIngestWithEvent`
- [x] 5.7 Instrument `install otel-node`: reset `ExecutionStart`, call `fireInstallEvent`
- [x] 5.8 Instrument `install otel-java`: reset `ExecutionStart`, call `fireInstallEvent`, switch to `WatchIngestWithEvent`
- [x] 5.9 Instrument `install aws`: reset `ExecutionStart`, set `OnWatchComplete`, call `fireInstallEvent`, clear `OnWatchComplete`
- [x] 5.10 Instrument `install aws-lambda`: reset `ExecutionStart`, call `fireInstallEvent`, switch to `WatchIngestWithEvent`
- [x] 5.11 Instrument `install azure`: reset `ExecutionStart`, set `OnWatchComplete`, call `fireInstallEvent`, clear `OnWatchComplete`
- [x] 5.12 Instrument `install gcp`: reset `ExecutionStart`, set `OnWatchComplete`, call `fireInstallEvent`, clear `OnWatchComplete`
- [x] 5.13 Instrument `install docker`: reset `ExecutionStart`, call `fireInstallEvent`, switch to `WatchIngestWithEvent`
- [x] 5.14 Instrument `install demo`: reset `ExecutionStart`, call `fireInstallEvent`, switch to `WatchIngestWithEvent`

## 6. Verification

- [x] 6.1 `make build` passes with no errors
- [x] 6.2 `make test` passes for all packages including `aws`, `azure`, `gcp` (no test signature changes required)
- [x] 6.3 `make lint` passes with 0 issues
