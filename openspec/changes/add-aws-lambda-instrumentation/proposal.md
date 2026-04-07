# Proposal

## Why

`dtwiz install aws` deploys AWS platform monitoring (CloudFormation stack for logs, events, metrics) and includes `Lambda_essential` in the Dynatrace extension feature sets. This gives metric-level visibility into Lambda functions but provides no function-level instrumentation — no distributed traces, no code-level insights. Users must manually attach the Dynatrace Lambda Layer to each function, set environment variables, and resolve the correct layer ARN per runtime/architecture/region. This is error-prone and violates the project's "if we detect it, we enable it" philosophy.

## What Changes

- Add `dtwiz install aws-lambda` command that automatically instruments all Lambda functions in the current AWS region with the Dynatrace Lambda Layer.
- Add `dtwiz uninstall aws-lambda` command that removes the Dynatrace Lambda Layer and associated env vars from all instrumented functions.
- Modify `dtwiz install aws` to run `InstallAWSLambda` concurrently alongside the existing CloudFormation deployment — zero additional user interaction.
- The installer lists all Lambda functions, auto-detects each function's runtime and architecture, queries the Dynatrace API (`/api/v1/deployment/lambda/layer`) for the matching layer ARN, and attaches the layer with the required DT_* environment variables.
- Already-instrumented functions get their layer updated to the latest version.
- `aws-lambda` does NOT appear as a separate recommendation in the `setup` menu — it is only accessible via `dtwiz install aws-lambda` directly or triggered automatically from `dtwiz install aws`.

## Capabilities

### New Capabilities

- `lambda-instrumentation`: Attach the Dynatrace Lambda Layer to all Lambda functions in the current AWS region. Auto-detect runtime and architecture per function, resolve the correct layer ARN from the DT API, set required environment variables, and handle already-instrumented functions by updating to the latest layer version.
- `lambda-uninstrumentation`: Remove the Dynatrace Lambda Layer and DT_* environment variables from all instrumented functions, preserving all other function configuration.
- `aws-lambda-parallel-execution`: Run Lambda instrumentation concurrently when `install aws` is invoked, without requiring additional user interaction.

### Modified Capabilities

- `aws-platform-monitoring` (existing `install aws`): Modified to spawn `InstallAWSLambda` as a concurrent goroutine alongside the CloudFormation deployment.

## Impact

- **Code**: New file `pkg/installer/aws_lambda.go`; modified `pkg/installer/aws.go`, `cmd/install.go`, `cmd/uninstall.go`.
- **Dependencies**: No new Go module dependencies. Uses `aws lambda` CLI commands (already a prerequisite for `install aws`) and existing Dynatrace API client patterns from `aws.go`.
- **UX**: `install aws` now instruments Lambda functions automatically in parallel. Users see a combined summary. `install aws-lambda` is available standalone for users who only want Lambda instrumentation. Both support `--dry-run`.
- **Token scope**: The access token (`DT_ACCESS_TOKEN`) needs `InstallerDownload` scope for the layer ARN API. This is in addition to the scopes already required by `install aws`.
- **Non-regression**: Existing `install aws` CloudFormation flow must remain functional and unchanged. The Lambda instrumentation runs alongside it, not as a replacement.

## Rollback Plan

All new code lives in a single new file (`pkg/installer/aws_lambda.go`) and additive changes to existing files. To roll back:

1. **Revert `pkg/installer/aws.go`** — remove the goroutine that calls `InstallAWSLambda` from `InstallAWS`.
2. **Revert `cmd/install.go`** — remove the `installAWSLambdaCmd` subcommand registration.
3. **Revert `cmd/uninstall.go`** — remove the `uninstallAWSLambdaCmd` subcommand registration.
4. **Delete `pkg/installer/aws_lambda.go`** — all Lambda-specific logic is contained here.
5. No database, config, or external service changes are involved — rollback is purely code deletion and revert.
