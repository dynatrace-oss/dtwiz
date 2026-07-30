# AWS Lambda Parallel Execution

## ADDED Requirements

### Requirement: Concurrent Lambda instrumentation from `install aws`

When `install aws` proceeds past its confirmation gate, it SHALL run the CloudFormation deployment (plus monitoring-config enablement) in a background goroutine and run `InstallAWSLambda` on the main thread concurrently. The CloudFormation goroutine reports progress through a status channel while Lambda instrumentation produces its output on the main thread. `install aws` waits for the CloudFormation goroutine to finish (via `sync.WaitGroup`) before returning.

Lambda instrumentation only runs on a real install: the dry-run branch short-circuits at the `ShouldProceed(dryRun, ...)` gate before either the CloudFormation deployment or Lambda instrumentation is reached.

#### Scenario: Both succeed

- **GIVEN** the user runs `dtwiz install aws`
- **WHEN** both the CloudFormation deployment and Lambda instrumentation complete successfully
- **THEN** the user sees output from both, and the command exits with success

#### Scenario: Lambda instrumentation fails, CloudFormation succeeds

- **GIVEN** the user runs `dtwiz install aws`
- **WHEN** the CloudFormation deployment succeeds but Lambda instrumentation fails
- **THEN** the CloudFormation result is not affected, a warning is printed ("Lambda instrumentation encountered an error: ..." with a retry hint), and the command exits with success

#### Scenario: Dry-run does not reach Lambda

- **GIVEN** the user runs `dtwiz install aws --dry-run`
- **WHEN** the command reaches the `ShouldProceed` gate
- **THEN** it prints the CloudFormation command preview and returns before deploying CloudFormation or running Lambda instrumentation; no Lambda preview is shown and nothing is modified

## MODIFIED Requirements

### Requirement: `install aws` runs Lambda instrumentation

The AWS install flow in `pkg/installer/aws/install.go` SHALL invoke `InstallAWSLambda(envURL, token, dryRun=false, confirm=false)` on the main thread, after the CloudFormation deploy goroutine has been started. Passing `confirm=false` means Lambda instrumentation applies immediately after its own preview without a second confirmation prompt (the user already confirmed at the `install aws` gate). Passing `dryRun=false` is safe because the dry-run path returns earlier and never reaches this call. Errors from `InstallAWSLambda` are non-fatal: they are printed as a warning with a retry hint and do not change the value returned by the install flow, which reflects the CloudFormation deploy result.

#### Scenario: Lambda error is non-fatal

- **GIVEN** `InstallAWSLambda` returns an error during `install aws`
- **WHEN** the install flow collects the result
- **THEN** the error is printed as a warning ("You can retry with: dtwiz install aws-lambda") and the flow returns the CloudFormation deploy result (nil when the deploy succeeded), not the Lambda error
