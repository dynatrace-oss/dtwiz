# AWS Lambda Parallel Execution

## Purpose

Define how `dtwiz install aws` instruments Lambda functions in parallel with the CloudFormation deployment so users get both platform and function-level monitoring from a single command.

## Requirements

### Requirement: `install aws` also instruments Lambda functions

When `install aws` proceeds past its confirmation gate, it SHALL run the CloudFormation deployment and Lambda instrumentation concurrently, wait for both to finish, and apply Lambda instrumentation without a second confirmation. Under `--dry-run`, neither the CloudFormation deployment nor Lambda instrumentation runs.

#### Scenario: Both succeed

- **GIVEN** the user runs `dtwiz install aws`
- **WHEN** both the CloudFormation deployment and Lambda instrumentation complete successfully
- **THEN** the user sees output from both and the command exits successfully

#### Scenario: Dry-run does not reach Lambda

- **GIVEN** the user runs `dtwiz install aws --dry-run`
- **WHEN** the command reaches its confirmation gate
- **THEN** it shows the CloudFormation preview and returns without deploying CloudFormation or instrumenting Lambda functions; no Lambda preview is shown and nothing is modified

### Requirement: Lambda errors are non-fatal within `install aws`

A failure during Lambda instrumentation SHALL NOT affect the CloudFormation deployment or the result returned by `install aws`. The system SHALL report the failure as a warning with a hint to retry via `dtwiz install aws-lambda`.

#### Scenario: Lambda instrumentation fails, CloudFormation succeeds

- **GIVEN** the user runs `dtwiz install aws`
- **WHEN** the CloudFormation deployment succeeds but Lambda instrumentation fails
- **THEN** the CloudFormation result is unaffected, a warning with a retry hint is shown, and the command exits successfully
