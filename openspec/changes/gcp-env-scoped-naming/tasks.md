# Implementation Tasks

## 1. Integration Name

- [x] 1.1 Replace the `integrationName` and `serviceAccountName` constants in
  `pkg/installer/gcp/config.go` with an `integrationPrefix` constant and an
  `integrationNameForEnv(envURL string) string` function that returns
  `dtwiz-gcp-<first-dns-label-of-env-url>`.

## 2. Install

- [x] 2.1 Derive the integration name from the environment URL in
  `pkg/installer/gcp/install.go` and use it for the Dynatrace connection lookup,
  connection creation, service account creation, and monitoring configuration creation.
- [x] 2.2 Add a `name string` parameter to `gcpResumableConnection` so error messages
  reference the actual derived name.

## 3. Update

- [x] 3.1 Derive the integration name from the environment URL in
  `pkg/installer/gcp/update.go` and use it for connection and monitoring
  configuration lookup and for the config struct.
- [x] 3.2 Add a `name string` parameter to `selectUpdatableConnection` so error
  messages reference the actual derived name.

## 4. Uninstall

- [x] 4.1 Use prefix matching in `findAllConnections` in
  `pkg/installer/gcp/dtapi.go` so a single query finds connections under both old
  and new names.
- [x] 4.2 Use `integrationPrefix` in `connectionExistsWithClient` and
  `MonitoringConfigExists` so the status check finds integrations under either
  naming scheme.
- [x] 4.3 Refactor `gcpGatherServiceAccounts` to accept `currentName` and return
  separate current and legacy SA email slices.
- [x] 4.4 In `uninstallGCPWithRunner`, derive `currentName` from the environment
  URL, split SA emails into current and legacy, and pass them separately to the
  deletion steps.
- [x] 4.5 In `runUninstallSteps`, treat legacy SA cleanup failures as warnings only.
  Do not add them to the error list returned to the caller.
- [x] 4.6 Update `uninstallStepCount` to accept separate current and legacy SA email
  slices.

## 5. Validation

- [x] 5.1 Run `go test ./pkg/installer/gcp/...` and confirm all tests pass.
