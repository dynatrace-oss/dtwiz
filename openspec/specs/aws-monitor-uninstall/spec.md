# AWS Monitor Uninstall

## Purpose

Define the `dtwiz uninstall aws` command that removes the Dynatrace AWS CloudFormation integration.

## Requirements

### Requirement: Uninstall command and entry point

The system SHALL expose `dtwiz uninstall aws` as the CLI command for removing the
Dynatrace AWS CloudFormation integration, accepting no positional arguments. The command
SHALL resolve the Dynatrace environment URL and platform token from standard sources
(`--environment`/`DT_ENVIRONMENT`, `--platform-token`/`DT_PLATFORM_TOKEN`) and honor the
shared `--dry-run` flag.

#### Scenario: Uninstall command registered

- **GIVEN** the CLI is built
- **WHEN** the user runs `dtwiz uninstall aws`
- **THEN** the AWS uninstaller runs against the resolved environment and platform token
- **AND** `--dry-run` previews the steps without executing them

### Requirement: AWS CLI preflight

The system SHALL verify the AWS CLI is present on `PATH` before any API call. If absent,
it SHALL abort with an actionable message pointing to the AWS CLI install documentation.

#### Scenario: AWS CLI missing

- **GIVEN** `aws` is not found on `PATH`
- **WHEN** the uninstaller runs preflight
- **THEN** it aborts with a message pointing to the AWS CLI v2 install page

### Requirement: AWS account and region discovery

The system SHALL call `aws sts get-caller-identity` and `aws configure get region` to
determine the active AWS account ID and region. Both values are surfaced in the output
and used to scope the CloudFormation stack deletion and monitoring config lookup.

#### Scenario: Account and region resolved successfully

- **GIVEN** the AWS CLI is configured with valid credentials
- **WHEN** the uninstaller runs
- **THEN** it prints the resolved AWS account ID and region before proceeding

### Requirement: Find existing monitoring configuration

Before prompting for confirmation, the system SHALL search for a `da-aws` monitoring
configuration matching the current AWS account. If one is found, its objectId is shown
in the confirmation summary and will be deleted in step 2. If none is found, step 2 is
skipped.

#### Scenario: Monitoring config found

- **GIVEN** a `da-aws` monitoring configuration exists for the current AWS account
- **WHEN** the uninstaller looks up configurations
- **THEN** it prints the objectId and includes it in the confirmation summary as step 2

#### Scenario: No monitoring config found

- **GIVEN** no `da-aws` monitoring configuration exists for the current AWS account
- **WHEN** the uninstaller looks up configurations
- **THEN** it prints that no configuration was found for the account
- **AND** skips the Dynatrace cleanup step without error

### Requirement: Confirmation preview before destructive steps

The system SHALL print the numbered list of steps that will be performed — always
including the CloudFormation stack deletion, and conditionally including the monitoring
config deletion — and prompt for a single `Apply?` confirmation (default yes). On
`--dry-run` it prints the preview and exits without executing any step.

#### Scenario: Dry-run stops before deletion

- **GIVEN** `--dry-run` is set
- **WHEN** the uninstaller reaches the confirmation step
- **THEN** it prints `[dry-run] No changes were made.` and exits without deleting anything

### Requirement: CloudFormation stack deletion with wait

Step 1 SHALL delete the CloudFormation stack named `dynatrace-data-acquisition` in the
current region and wait for full deletion before proceeding to step 2.

#### Scenario: Stack deleted and waited

- **GIVEN** the user confirmed
- **WHEN** step 1 executes
- **THEN** it calls `aws cloudformation delete-stack --stack-name dynatrace-data-acquisition --region <region>`
- **AND** calls `aws cloudformation wait stack-delete-complete` before returning

### Requirement: Monitoring configuration deletion

Step 2 SHALL delete the `da-aws` monitoring configuration identified in the preflight
lookup, using the dtctl SDK. If no configuration was found the step is skipped.

#### Scenario: Monitoring config deleted

- **GIVEN** a monitoring configuration objectId was found
- **WHEN** step 2 executes
- **THEN** it calls `DeleteMonitoringConfiguration` for the `da-aws` extension with the objectId
- **AND** prints the objectId upon successful deletion
