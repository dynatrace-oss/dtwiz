# Lambda Uninstrumentation

## ADDED Requirements

### Requirement: Detect instrumented functions

The system SHALL list all Lambda functions in the current AWS region and filter to those that have a Dynatrace Lambda Layer attached (identified by any layer ARN containing `Dynatrace_OneAgent`). Dynatrace-managed functions (name contains `DynatraceApiClientFunction`) SHALL be excluded from the candidate list even if they carry a Dynatrace layer.

#### Scenario: Instrumented functions found

- **GIVEN** 10 Lambda functions exist in the current region
- **WHEN** 3 of them have a layer ARN containing `Dynatrace_OneAgent` (and none are Dynatrace-managed)
- **THEN** the uninstall preview shows only those 3 functions

#### Scenario: No instrumented functions found

- **GIVEN** no Lambda functions in the current region have a Dynatrace layer
- **WHEN** `UninstallAWSLambda` runs
- **THEN** it prints "No Lambda functions with Dynatrace instrumentation found" and exits without error

### Requirement: Clean removal of instrumentation

The system SHALL read each candidate function's live configuration, remove every Dynatrace Lambda Layer (ARN contains `Dynatrace_OneAgent`) from the layers list, and remove the Dynatrace-managed environment variables from the environment map: `AWS_LAMBDA_EXEC_WRAPPER`, `DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_BASE_URL`, `DT_CONNECTION_AUTH_TOKEN`, `DT_ENABLE_ESM_LOADERS`. All other layers and environment variables SHALL be preserved. The updated configuration is written back via `aws lambda update-function-configuration`.

#### Scenario: Remove layer and env vars

- **GIVEN** a function has layers `[custom-layer, Dynatrace_OneAgent_...]` and env vars `DATABASE_URL`, `DT_TENANT`, `DT_CONNECTION_BASE_URL`, `LOG_LEVEL`
- **WHEN** the system uninstruments the function
- **THEN** the updated layers are `[custom-layer]` and env vars are `DATABASE_URL`, `LOG_LEVEL`

#### Scenario: Node.js ESM loader flag removed

- **GIVEN** a Node.js function has env var `DT_ENABLE_ESM_LOADERS=true` alongside the other DT_* vars
- **WHEN** the system uninstruments the function
- **THEN** `DT_ENABLE_ESM_LOADERS` is removed along with the other DT_* vars

#### Scenario: Function with only DT env vars

- **GIVEN** a function's only env vars are the DT_* variables
- **WHEN** the system uninstruments the function
- **THEN** the environment variables map is empty (an empty `Variables` object, not null)

### Requirement: Uninstall preview, confirmation, and dry-run

The system SHALL show a preview of functions to be cleaned up (name, runtime, architecture) with a total count before applying changes. Under `--dry-run`, the preview is shown but no changes are applied and no confirmation prompt appears. Otherwise the system prompts `Apply?` and proceeds only on confirmation, returning `ErrInstallCancelled` on decline. The final summary reports successes and failures.

#### Scenario: Uninstall dry run

- **GIVEN** `--dry-run` is set
- **WHEN** `UninstallAWSLambda` runs
- **THEN** the preview is printed, no functions are modified, and no confirmation prompt appears

#### Scenario: Uninstall with confirmation

- **GIVEN** `--dry-run` is NOT set
- **WHEN** `UninstallAWSLambda` displays the preview
- **THEN** it prompts "Apply?" and proceeds only on confirmation; on decline it returns `ErrInstallCancelled`
