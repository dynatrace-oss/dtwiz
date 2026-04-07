# Lambda Uninstrumentation

## ADDED Requirements

### Requirement: Detect instrumented functions

The system SHALL list all Lambda functions in the current AWS region and filter to those that have a Dynatrace Lambda Layer attached (identified by any layer ARN containing `Dynatrace_OneAgent`).

#### Scenario: Instrumented functions found

- **GIVEN** 10 Lambda functions exist in the current region
- **WHEN** 3 of them have a layer ARN containing `Dynatrace_OneAgent`
- **THEN** the uninstall preview shows only those 3 functions

#### Scenario: No instrumented functions found

- **GIVEN** no Lambda functions in the current region have a Dynatrace layer
- **WHEN** `UninstallAWSLambda` runs
- **THEN** it prints "No Lambda functions with Dynatrace instrumentation found" and exits without error

### Requirement: Clean removal of instrumentation

The system SHALL remove the Dynatrace Lambda Layer from the function's layers list and remove the DT_* environment variables (`DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_BASE_URL`, `DT_CONNECTION_AUTH_TOKEN`, `AWS_LAMBDA_EXEC_WRAPPER`). All other layers and environment variables SHALL be preserved.

#### Scenario: Remove layer and env vars

- **GIVEN** a function has layers `[custom-layer, Dynatrace_OneAgent_...]` and env vars `DATABASE_URL`, `DT_TENANT`, `DT_CONNECTION_BASE_URL`, `LOG_LEVEL`
- **WHEN** the system uninstruments the function
- **THEN** the updated layers are `[custom-layer]` and env vars are `DATABASE_URL`, `LOG_LEVEL`

#### Scenario: Function with only DT env vars

- **GIVEN** a function's only env vars are the DT_* variables
- **WHEN** the system uninstruments the function
- **THEN** the environment variables map is empty (not null — to avoid AWS API errors)

### Requirement: Uninstall preview and dry-run

The system SHALL show a preview of functions to be cleaned up before applying changes. Under `--dry-run`, the preview is shown but no changes are applied.

#### Scenario: Uninstall dry run

- **GIVEN** `--dry-run` is set
- **WHEN** `UninstallAWSLambda` runs
- **THEN** the preview is printed, no functions are modified, and no confirmation prompt appears

#### Scenario: Uninstall with confirmation

- **GIVEN** `--dry-run` is NOT set
- **WHEN** `UninstallAWSLambda` displays the preview
- **THEN** it prompts "Apply? [Y/n]" and proceeds only on confirmation
