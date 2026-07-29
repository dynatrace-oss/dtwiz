# AWS Monitor Install

## ADDED Requirements

### Requirement: Install command and entry point

The system SHALL expose `dtwiz install aws` as the CLI command for AWS CloudFormation
integration, accepting no positional arguments. The command SHALL resolve the Dynatrace
environment URL and platform token from standard sources (`--environment`/`DT_ENVIRONMENT`,
`--platform-token`/`DT_PLATFORM_TOKEN`) and honor the shared `--dry-run` flag.

#### Scenario: Install command registered

- **GIVEN** the CLI is built
- **WHEN** the user runs `dtwiz install aws`
- **THEN** the AWS installer runs against the resolved environment and platform token
- **AND** `--dry-run` shows the CloudFormation command preview without deploying

### Requirement: AWS CLI preflight

The system SHALL verify the AWS CLI is present on `PATH` before any Dynatrace or AWS API
call. If absent, it SHALL abort with an actionable message pointing to the AWS CLI install
documentation.

#### Scenario: AWS CLI missing

- **GIVEN** `aws` is not found on `PATH`
- **WHEN** the installer runs preflight
- **THEN** it aborts with a message pointing to the AWS CLI v2 install page

### Requirement: AWS account and region discovery

The system SHALL call `aws sts get-caller-identity` and `aws configure get region` to
determine the active AWS account ID and region before any other step. Both values are
surfaced in the output and used throughout the install workflow.

#### Scenario: Account and region resolved

- **GIVEN** the AWS CLI is configured and authenticated
- **WHEN** the installer fetches caller info
- **THEN** it prints the AWS account ID and region
- **AND** uses the region as the default for `LogsRegions` and `EventsRegions` in the CloudFormation parameters

### Requirement: Activate the `da-aws` Dynatrace extension

Before creating a monitoring configuration, the system SHALL install or activate the
`com.dynatrace.extension.da-aws` extension in the Dynatrace tenant. If the extension is
already installed the system SHALL treat that condition as success and continue. If freshly
installed, the system SHALL wait for the extension to become active before proceeding.

#### Scenario: Extension freshly installed

- **GIVEN** the `da-aws` extension is not yet installed in the tenant
- **WHEN** the installer calls the extension activation API
- **THEN** it waits for the extension to report `ACTIVE` before continuing to monitoring config creation
- **AND** prints `✓ Extension is active`

#### Scenario: Already-installed extension is accepted

- **GIVEN** the `da-aws` extension is already installed in the tenant
- **WHEN** the installer calls the extension activation API
- **THEN** it treats the already-installed response as success and continues immediately

### Requirement: Find or create monitoring configuration

The system SHALL search existing `da-aws` monitoring configurations for an entry whose
`value.aws.credentials[].accountId` matches the current AWS account ID. If one is found
it SHALL be reused; otherwise a new monitoring configuration SHALL be created using the
latest installed extension version.

#### Scenario: Existing monitoring config reused

- **GIVEN** a monitoring configuration already exists for the current AWS account
- **WHEN** the installer searches existing configurations
- **THEN** it reuses the existing objectId without creating a new configuration

#### Scenario: New monitoring config created

- **GIVEN** no monitoring configuration exists for the current AWS account
- **WHEN** the installer resolves the latest installed extension version
- **THEN** it creates a new monitoring configuration with the default AWS feature sets
- **AND** stores the returned objectId as `pMonitoringConfigId` in the CloudFormation parameters

### Requirement: CloudFormation deployment preview and confirmation

Before deploying, the system SHALL print the full `aws cloudformation deploy` command
with token values masked (first 10 characters followed by `***`). It SHALL then prompt
for a single `Apply?` confirmation (default yes). On `--dry-run` it prints the preview
and exits without prompting or deploying.

#### Scenario: Token values masked in preview

- **GIVEN** the deploy command includes `pDtApiToken` and `pDtIngestToken` values
- **WHEN** the preview is printed
- **THEN** each token value is shown as the first 10 characters followed by `***`

#### Scenario: Dry-run stops before deployment

- **GIVEN** `--dry-run` is set
- **WHEN** the installer reaches the confirmation step
- **THEN** it prints `[dry-run] No changes were made.` and exits without deploying

### Requirement: CloudFormation stack deployment and monitoring config enable

After confirmation, the system SHALL download the CloudFormation template from S3,
deploy the stack, and then enable the monitoring configuration. The deployment runs in a
background goroutine. After the stack is deployed, the system SHALL enable the monitoring
configuration by flipping `value.enabled` and all `value.aws.credentials[].enabled` to
`true` via a GET+PUT round-trip.

#### Scenario: Monitoring config enabled after deployment

- **GIVEN** the CloudFormation stack deployed successfully
- **WHEN** `enableMonitoringConfig` is called
- **THEN** it fetches the monitoring configuration
- **AND** sets `value.enabled = true` and all `value.aws.credentials[i].enabled = true`
- **AND** writes the updated configuration back
- **AND** data begins flowing from AWS into Dynatrace

#### Scenario: Deployment failure surfaced as error

- **GIVEN** `aws cloudformation deploy` exits with a non-zero status
- **WHEN** the background goroutine captures the error
- **THEN** the error is returned from `InstallAWS` after all concurrent work completes

### Requirement: Lambda instrumentation runs in parallel

`dtwiz install aws` SHALL invoke `InstallAWSLambda` on the main thread (after the
CloudFormation goroutine is started) to instrument all Lambda functions in the current
region with the Dynatrace Lambda Layer. Lambda instrumentation errors are non-fatal: they
are printed as a warning and the user is directed to retry with `dtwiz install aws-lambda`.

#### Scenario: Lambda failure is a warning

- **GIVEN** Lambda instrumentation encounters an error
- **WHEN** `InstallAWSLambda` returns
- **THEN** the error is printed as a warning
- **AND** the overall `install aws` flow continues and may still succeed

### Requirement: Ingest watch after install

After all concurrent work completes, the system SHALL tail newly ingested AWS data
(cloud-platform signals scoped to the current AWS account) starting from the recorded
install start time. When `startTime` is empty the watch is skipped.

#### Scenario: Watch scoped to current account

- **GIVEN** a non-empty `startTime` and a known `accountID`
- **WHEN** the watch starts
- **THEN** DQL queries are filtered to the current AWS account ID to reduce noise from other accounts
