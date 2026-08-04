# Lambda Uninstrumentation

## ADDED Requirements

### Requirement: Detect instrumented functions

The system SHALL discover all Lambda functions in the current AWS region and select those that have a Dynatrace layer attached. Dynatrace-managed functions SHALL be excluded from the candidate list even if they carry a Dynatrace layer.

#### Scenario: Instrumented functions found

- **GIVEN** ten Lambda functions exist in the current region and three of them have a Dynatrace layer (none Dynatrace-managed)
- **WHEN** the uninstaller runs
- **THEN** the preview shows only those three functions

#### Scenario: No instrumented functions found

- **GIVEN** no functions in the current region have a Dynatrace layer
- **WHEN** the uninstaller runs
- **THEN** it reports that no instrumented functions were found and exits without error

### Requirement: Clean removal of instrumentation

The system SHALL remove every Dynatrace layer and every Dynatrace-managed environment variable (`AWS_LAMBDA_EXEC_WRAPPER`, `DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_BASE_URL`, `DT_CONNECTION_AUTH_TOKEN`, `DT_ENABLE_ESM_LOADERS`) from each candidate function, preserving all other layers and environment variables.

#### Scenario: Remove layer and env vars

- **GIVEN** a function has a custom layer plus a Dynatrace layer, and a mix of unrelated and Dynatrace environment variables
- **WHEN** the system uninstruments the function
- **THEN** only the custom layer and the unrelated environment variables remain

#### Scenario: Node.js ESM loader flag removed

- **GIVEN** a Node.js function has `DT_ENABLE_ESM_LOADERS=true` alongside the other Dynatrace variables
- **WHEN** the system uninstruments the function
- **THEN** `DT_ENABLE_ESM_LOADERS` is removed along with the other Dynatrace variables

#### Scenario: Function with only DT env vars

- **GIVEN** a function's only environment variables are the Dynatrace variables
- **WHEN** the system uninstruments the function
- **THEN** the function is left with no environment variables

### Requirement: Uninstall preview, confirmation, and dry-run

The system SHALL show a preview of the functions to be cleaned up (name, runtime, architecture) with a total count before applying changes. Under `--dry-run`, the preview is shown but no changes are applied and no confirmation is requested. Otherwise the system SHALL request confirmation and proceed only on confirmation, cancelling cleanly on decline. The final summary SHALL report successes and failures.

#### Scenario: Uninstall dry run

- **GIVEN** `--dry-run` is set
- **WHEN** the uninstaller runs
- **THEN** the preview is shown, no functions are modified, and no confirmation is requested

#### Scenario: Uninstall with confirmation

- **GIVEN** `--dry-run` is not set
- **WHEN** the preview is shown
- **THEN** the system requests confirmation and proceeds only on confirmation; on decline it cancels cleanly
