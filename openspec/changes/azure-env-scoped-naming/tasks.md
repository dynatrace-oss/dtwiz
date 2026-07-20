# Implementation Tasks

## 1. Integration Name

- [x] 1.1 Replace the `integrationName` constant in `pkg/installer/azure/config.go`
  with an `integrationPrefix` constant and an `integrationNameForEnv(envURL string)`
  function that returns `dtwiz-azure-<first-dns-label-of-env-url>`.

## 2. Install

- [x] 2.1 Derive the integration name from the environment URL in
  `pkg/installer/azure/install.go` and use it for the Dynatrace connection lookup,
  connection creation, and monitoring configuration creation.

## 3. Update

- [x] 3.1 Derive the integration name from the environment URL in
  `pkg/installer/azure/update.go` and use it for connection and monitoring
  configuration lookup.
- [x] 3.2 Run account lookup before DT resource discovery (sequential, not
  concurrent) so a login failure aborts before querying Dynatrace.
- [x] 3.3 Add a `name` parameter to `selectUpdatableConnection` so error messages
  reference the actual derived name.

## 4. Uninstall

- [x] 4.1 Use prefix matching in `findAllConnections` in
  `pkg/installer/azure/dtapi.go` so a single query finds both old and new names.
- [x] 4.2 Use prefix matching in `FindAllMonitoringConfigs` in
  `pkg/installer/extension_client.go` for the same reason.
- [x] 4.3 In `uninstallAzureWithRunner`, split client IDs into current (from DT
  connections and env-scoped name search) and legacy (from old fixed-name search
  only). Pass them separately to the deletion steps.
- [x] 4.4 In `runUninstallSteps`, treat legacy client ID deletion failures as
  warnings only. Do not add them to the error list returned to the caller.
- [x] 4.5 Update `azureGatherClientIDs` to accept a slice of names so it can
  search under both the current and legacy names in one call from the caller.
- [x] 4.6 Update `connectionExistsWithClient` and `MonitoringConfigExists` to use
  the prefix so the status check finds integrations under either naming scheme.

## 5. Validation

- [x] 5.1 Run `go test ./pkg/installer/azure/... ./pkg/installer/...` and confirm
  all tests pass.
