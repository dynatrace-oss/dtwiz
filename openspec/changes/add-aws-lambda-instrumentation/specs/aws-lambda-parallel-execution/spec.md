# AWS Lambda Parallel Execution

## ADDED Requirements

### Requirement: Concurrent Lambda instrumentation from `install aws`

When `InstallAWS` is invoked, it SHALL spawn `InstallAWSLambda` in a separate goroutine at the start of its flow. The CloudFormation deployment and Lambda instrumentation run concurrently. `InstallAWS` waits for both to complete before returning.

#### Scenario: Both succeed

- **GIVEN** the user runs `dtwiz install aws`
- **WHEN** both the CloudFormation deployment and Lambda instrumentation complete successfully
- **THEN** the user sees output from both, and the command exits with success

#### Scenario: Lambda instrumentation fails, CloudFormation succeeds

- **GIVEN** the user runs `dtwiz install aws`
- **WHEN** the CloudFormation deployment succeeds but Lambda instrumentation fails
- **THEN** the CloudFormation result is not affected, the Lambda error is logged, and the command exits with success (platform monitoring is more important)

#### Scenario: Dry-run applies to both

- **GIVEN** the user runs `dtwiz install aws --dry-run`
- **WHEN** both flows run concurrently
- **THEN** both show their preview without applying any changes

## MODIFIED Requirements

### Requirement: `InstallAWS` spawns Lambda instrumentation

The existing `InstallAWS()` function in `pkg/installer/aws.go` SHALL be modified to start `InstallAWSLambda()` as a concurrent goroutine. The Lambda goroutine receives the same credentials (envURL, token, platformToken, dryRun). A `sync.WaitGroup` ensures `InstallAWS` does not return until both flows are complete. Errors from the Lambda goroutine are logged but do not cause `InstallAWS` to fail.

#### Scenario: Lambda goroutine error is non-fatal

- **GIVEN** `InstallAWSLambda` returns an error
- **WHEN** `InstallAWS` collects the result
- **THEN** the error is logged with a warning prefix, and `InstallAWS` returns nil (not the Lambda error)
