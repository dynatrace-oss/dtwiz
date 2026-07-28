# Tasks

## 1. Create `pkg/installer/aws/` package

**Files:** `pkg/installer/aws/config.go` (create), `pkg/installer/aws/dtapi.go` (create)

- [x] 1.1 Create `config.go` with `awsStackConfig` struct, `awsTemplateURL` const, `defaultFeatureSets` var
- [x] 1.2 Define `dtclient` interface in `dtapi.go` with methods: `installExtension`, `isExtensionActive`, `latestExtensionVersion`, `findExistingMonitoringConfig`, `createMonitoringConfig`, `enableMonitoringConfig`, `deleteMonitoringConfig`
- [x] 1.3 Implement `sdkDTClient` embedding `*installer.ExtensionClient`; implement all `dtclient` methods via dtctl SDK
- [x] 1.4 Implement `enableMonitoringConfig`: GET config → flip `value.enabled` and all `value.aws.credentials[].enabled` to `true` → PUT
- [x] 1.5 Implement `newSDKDTClient(envURL, token string) (*sdkDTClient, error)` factory

## 2. Migrate install logic to `pkg/installer/aws/install.go`

**Files:** `pkg/installer/aws/install.go` (create), `pkg/installer/aws.go` (modify)

- [x] 2.1 Create `install.go` with public `InstallAWS(envURL, token string, dryRun bool, startTime string) error`
- [x] 2.2 Move all install logic to internal `installAWSWithClient(..., dtc dtclient) error`
- [x] 2.3 Update `pkg/installer/aws.go`: export `GetAWSCallerInfo()` and `IsAWSCLIInstalled()`, remove all other logic

## 3. Migrate uninstall logic to `pkg/installer/aws/uninstall.go`

**Files:** `pkg/installer/aws/uninstall.go` (create), `pkg/installer/aws_uninstall.go` (delete)

- [x] 3.1 Create `uninstall.go` with public `UninstallAWS(envURL, token string, dryRun bool) error`
- [x] 3.2 Move all uninstall logic to internal `uninstallAWSWithClient(dryRun bool, dtc dtclient) error`
- [x] 3.3 Delete `pkg/installer/aws_uninstall.go`

## 4. Update cmd handlers

**Files:** `cmd/install.go` (modify), `cmd/uninstall.go` (modify), `cmd/setup.go` (modify)

- [x] 4.1 Add `awspkg` import alias in `install.go`, `uninstall.go`, `setup.go`
- [x] 4.2 Replace `installer.InstallAWS(c.Platform, ...)` with `awspkg.InstallAWS(...)` in all three files
- [x] 4.3 Replace `installer.UninstallAWS(c.Platform, ...)` with `awspkg.UninstallAWS(...)` in `uninstall.go`
- [x] 4.4 Remove `setupClient()` / `setupClientFromCreds()` calls from AWS handlers

## 5. Update `aws_lambda.go` for renamed helpers

**Files:** `pkg/installer/aws_lambda.go` (modify)

- [x] 5.1 Rename `getAWSCallerInfo()` → `GetAWSCallerInfo()` (exported)
- [x] 5.2 Rename `isAWSCLIInstalled()` → `IsAWSCLIInstalled()` (exported, two call sites)

## 6. Add tests

**Files:** `pkg/installer/aws/dtapi_test.go` (create), `pkg/installer/aws_test.go` (delete)

- [x] 6.1 Create `dtapi_test.go` with `TestEnableMonitoringConfig_FlipsTopLevelAndCredentials` (httptest round-trip)
- [x] 6.2 Create `TestEnableMonitoringConfig_PropagatesGetError` (404 error propagation)
- [x] 6.3 Delete `pkg/installer/aws_test.go` (old tests no longer applicable)

## 7. Lint and build verification

- [x] 7.1 Fix SA9003 empty branch: add `logger.Debug` call in extension-wait failure branch
- [x] 7.2 Fix unused `dtclient` type: add `installAWSWithClient` / `uninstallAWSWithClient` internal functions consuming the interface
- [x] 7.3 `make lint` passes with no new issues
- [x] 7.4 `make test` passes
