# K8s Uninstall Resilience

## Purpose

Define the resilience behavior for `dtwiz uninstall kubernetes` so that all uninstall steps execute even when prior steps fail.

## Requirements

### Requirement: All uninstall steps execute regardless of prior step failure

The uninstall sequence SHALL run all 4 steps (delete CRs, wait for pods, helm uninstall, delete namespace) even when an earlier step fails. Step failures SHALL be printed inline as they occur. If one or more steps fail, the command SHALL exit with a non-zero status and a summary error message.

#### Scenario: Helm uninstall fails but namespace is still deleted

- **WHEN** step 3 (helm uninstall) fails because the release does not exist
- **THEN** step 4 (delete namespace) still executes
- **THEN** if the namespace exists it is removed
- **THEN** the command exits with an error referencing "one or more steps failed"

#### Scenario: Step 1 fails but subsequent steps still run

- **WHEN** step 1 (delete DynaKube CRs) fails
- **THEN** step 3 (helm uninstall) still executes
- **THEN** step 4 (delete namespace) still executes

#### Scenario: EdgeConnect deletion runs even when DynaKube deletion fails

- **WHEN** step 1 (delete DynaKube CRs) fails
- **THEN** EdgeConnect CR deletion is still attempted

#### Scenario: All steps succeed

- **WHEN** all 4 steps complete without error
- **THEN** the command exits with status 0
- **THEN** output contains "uninstalled successfully"

#### Scenario: Multiple steps fail

- **WHEN** more than one step fails
- **THEN** each failure is printed inline as it occurs
- **THEN** the command exits with a single summary error
- **THEN** output contains "Uninstall completed with errors"

### Requirement: Usage block suppressed on runtime errors

The CLI SHALL NOT print the command usage block when a command fails due to a runtime error (e.g. a subprocess exits non-zero). The usage block SHALL still appear for invalid invocations such as unknown flags or unrecognised subcommands.

#### Scenario: Runtime error does not show usage

- **WHEN** `dtwiz uninstall kubernetes` is invoked with valid flags and a step fails at runtime
- **THEN** the error message is printed
- **THEN** the usage block is NOT printed

#### Scenario: Unknown flag still shows usage

- **WHEN** `dtwiz uninstall kubernetes --unknown-flag` is invoked
- **THEN** the usage block IS printed

#### Scenario: Unknown flag on other commands still shows usage

- **WHEN** `dtwiz setup --unknown-flag` is invoked
- **THEN** the usage block IS printed

### Requirement: Each error is printed exactly once

The CLI SHALL print each runtime error exactly once. Cobra's built-in error output SHALL be the single source; no additional manual print SHALL duplicate it.

#### Scenario: Single error print on failure

- **WHEN** any command's `RunE` returns an error
- **THEN** the error appears in stderr exactly once
