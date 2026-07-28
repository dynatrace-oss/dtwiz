# AWS Monitor Install

## CHANGED Requirements

### Requirement: AWS installer uses dtctl SDK for all extension API calls

`pkg/installer/aws/install.go` and its supporting `dtapi.go` SHALL delegate all
Dynatrace Extensions API calls through `installer.ExtensionClient` (dtctl SDK) instead
of making inline HTTP requests via `pkg/extensions`. All extension operations use a
`sdkDTClient` embedding `*installer.ExtensionClient` (Bearer auth, `.apps.` domain).

#### Scenario: InstallAWS activates the da-aws extension before creating monitoring config

- **GIVEN** a user runs `dtwiz install aws` (or selects AWS from `dtwiz setup`)
- **WHEN** `InstallAWS` executes after fetching AWS account info
- **THEN** it calls `dtc.installExtension()` for `com.dynatrace.extension.da-aws`
- **AND** if freshly installed, waits for the extension to become active before continuing
- **AND** only after that does it call `findExistingMonitoringConfig`

#### Scenario: InstallAWS creates a monitoring config when none exists

- **GIVEN** `findExistingMonitoringConfig` returns no entry matching the AWS account
- **WHEN** `findExistingMonitoringConfig` returns an empty string
- **THEN** `createMonitoringConfig` is called with the AWS account ID, region, and latest extension version
- **AND** the returned objectId is stored as `pMonitoringConfigId` in the CloudFormation parameters

#### Scenario: InstallAWS reuses an existing monitoring config

- **GIVEN** `findExistingMonitoringConfig` returns an entry whose `value.aws.credentials[].accountId`
  matches the current AWS account
- **WHEN** `findExistingMonitoringConfig` returns that objectId
- **THEN** `createMonitoringConfig` is NOT called
- **AND** the existing objectId is passed to CloudFormation

#### Scenario: InstallAWS enables the monitoring config after CloudFormation deployment

- **GIVEN** `enableMonitoringConfig` is called with the monitoring config objectId
- **WHEN** the CloudFormation stack has been deployed successfully
- **THEN** it GETs the monitoring configuration
- **AND** sets `value.enabled = true` and `value.aws.credentials[i].enabled = true` for all credentials
- **AND** PUTs the modified configuration back
- **AND** data begins flowing from AWS into Dynatrace

### Requirement: AWS install commands do not require setupClient()

`cmd/install.go`, `cmd/uninstall.go`, and `cmd/setup.go` SHALL resolve credentials
for AWS commands using `getDtEnvironment()` and `validateCredentials()` only, without
calling `setupClient()` or `setupClientFromCreds()`. The platform token and environment
URL are passed directly to `awspkg.InstallAWS`.

#### Scenario: install aws command uses getDtEnvironment

- **GIVEN** a user runs `dtwiz install aws`
- **WHEN** `installAWSCmd.RunE` executes
- **THEN** it calls `getDtEnvironment()` and `validateCredentials()`
- **AND** passes `envURL` and `platformTok` directly to `awspkg.InstallAWS`
- **AND** does NOT call `setupClient()` or `setupClientFromCreds()`
