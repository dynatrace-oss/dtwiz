# AWS Installer — Extensions Package Wiring

## CHANGED Requirements

### Requirement: AWS installer uses pkg/extensions for all extension API calls

`pkg/installer/aws.go` and `pkg/installer/aws_uninstall.go` SHALL delegate all
Dynatrace Extensions API calls to `pkg/extensions` instead of making inline HTTP
requests. All extension operations use the `*client.PlatformClient` (Bearer auth,
`.apps.` domain).

#### Scenario: InstallAWS activates the da-aws extension before creating monitoring config

- **GIVEN** a user runs `dtwiz install aws` (or selects AWS from `dtwiz setup`)
- **WHEN** `InstallAWS` executes after fetching AWS account info
- **THEN** it calls `extensions.InstallExtension(c, "com.dynatrace.extension.da-aws", "1.0.0", true)`
- **AND** `silent=true` so an existing installation does not abort the flow
- **AND** only after that does it call `findExistingMonitoringConfig`

#### Scenario: InstallAWS creates a monitoring config when none exists

- **GIVEN** `extensions.ListMonitoringConfigs` returns no entry matching the AWS account
- **WHEN** `findExistingMonitoringConfig` returns an empty string
- **THEN** `createDTMonitoringConfig` calls `extensions.CreateMonitoringConfigs` with the
  da-aws payload
- **AND** stores the returned objectId as `pMonitoringConfigId` in the CloudFormation parameters

#### Scenario: InstallAWS reuses an existing monitoring config

- **GIVEN** `extensions.ListMonitoringConfigs` returns an entry whose `value.aws.credentials[].accountId`
  matches the current AWS account
- **WHEN** `findExistingMonitoringConfig` returns that objectId
- **THEN** `createDTMonitoringConfig` is NOT called
- **AND** the existing objectId is passed to CloudFormation

#### Scenario: UninstallAWS deletes the monitoring config

- **GIVEN** `findExistingMonitoringConfig` returns a non-empty objectId
- **WHEN** `UninstallAWS` executes after deleting the CloudFormation stack
- **THEN** it calls `extensions.DeleteMonitoringConfig(c, "com.dynatrace.extension.da-aws", objectID)`

#### Scenario: UninstallAWS skips config deletion when none found

- **GIVEN** `findExistingMonitoringConfig` returns an empty string
- **WHEN** `UninstallAWS` evaluates the monitoring config step
- **THEN** it prints that no monitoring configuration was found
- **AND** skips the deletion step without error

### Requirement: AWS commands pass PlatformClient to installer functions

`cmd/install.go`, `cmd/uninstall.go`, and `cmd/setup.go` SHALL pass `c.Platform`
(obtained from `setupClient()`) to `InstallAWS` and `UninstallAWS`.

#### Scenario: install aws command injects PlatformClient

- **GIVEN** a user runs `dtwiz install aws`
- **WHEN** `installAWSCmd.RunE` executes
- **THEN** it calls `setupClient()` to create a `*client.Client`
- **AND** passes `c.Platform` to `installer.InstallAWS`

#### Scenario: uninstall aws command injects PlatformClient

- **GIVEN** a user runs `dtwiz uninstall aws`
- **WHEN** `uninstallAWSCmd.RunE` executes
- **THEN** it calls `setupClient()` to create a `*client.Client`
- **AND** passes `c.Platform` to `installer.UninstallAWS`
