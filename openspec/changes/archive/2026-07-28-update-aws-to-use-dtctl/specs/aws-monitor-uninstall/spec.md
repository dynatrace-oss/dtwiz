# AWS Monitor Uninstall

## CHANGED Requirements

### Requirement: AWS uninstaller uses dtctl SDK for all extension API calls

`pkg/installer/aws/uninstall.go` and its supporting `dtapi.go` SHALL delegate all
Dynatrace Extensions API calls through `installer.ExtensionClient` (dtctl SDK) instead
of making inline HTTP requests via `pkg/extensions`.

#### Scenario: UninstallAWS deletes the monitoring config

- **GIVEN** `findExistingMonitoringConfig` returns a non-empty objectId matching the AWS account
- **WHEN** `UninstallAWS` executes after deleting the CloudFormation stack
- **THEN** it calls `dtc.deleteMonitoringConfig(objectId)` via the dtctl SDK
- **AND** the monitoring configuration is removed from the Dynatrace tenant

#### Scenario: UninstallAWS skips config deletion when none found

- **GIVEN** `findExistingMonitoringConfig` returns an empty string
- **WHEN** `UninstallAWS` evaluates the monitoring config step
- **THEN** it prints that no monitoring configuration was found for the account
- **AND** skips the deletion step without error

### Requirement: Uninstall command does not require setupClient()

`cmd/uninstall.go` SHALL resolve credentials for the AWS uninstall command using
`getDtEnvironment()` and `validateCredentials()` only, without calling `setupClient()`
or `setupClientFromCreds()`.

#### Scenario: uninstall aws command uses getDtEnvironment

- **GIVEN** a user runs `dtwiz uninstall aws`
- **WHEN** `uninstallAWSCmd.RunE` executes
- **THEN** it calls `getDtEnvironment()` and `validateCredentials()`
- **AND** passes `envURL` and `platformTok` directly to `awspkg.UninstallAWS`
- **AND** does NOT call `setupClient()` or `setupClientFromCreds()`
