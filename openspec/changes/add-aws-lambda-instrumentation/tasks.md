# Tasks

## 1. Core Lambda instrumentation installer

Implement the main `InstallAWSLambda()` function and all supporting helpers in a new file. This is the bulk of the work.

**Files:** `pkg/installer/aws_lambda.go` (create)

- [x] 1.1 Create `aws_lambda.go` with `InstallAWSLambda(envURL, token, platformToken string, dryRun bool) error` entry point
- [x] 1.2 Implement `getAWSRegion()` — run `aws configure get region` to get the current AWS region
- [x] 1.3 Implement `listLambdaFunctions(region string)` — call `aws lambda list-functions` with pagination (`--no-paginate` or manual `--starting-token`), parse JSON output, return slice of `lambdaFunction` structs (name, runtime, architecture, layers, env vars, packageType). Exclude `PackageType: Image` functions.
- [x] 1.4 Implement `mapRuntimeToTechtype(runtime string) (string, bool)` — map AWS runtime prefix to DT techtype (`nodejs` -> `nodejs`, `python` -> `python`, `java` -> `java`, `go` -> `go`). Return false for unsupported runtimes.
- [x] 1.5 Implement `getDTConnectionInfo(envURL, token string)` — call `GET /api/v1/deployment/installer/agent/connectioninfo` on the Classic API URL with `Api-Token` auth. Parse response for `tenantUUID` and cluster ID. Return a `dtConnectionInfo` struct.
- [x] 1.6 Implement `getLambdaLayerARN(envURL, token, techtype, arch, region string)` — call `GET /api/v1/deployment/lambda/layer` with query params. Parse response. Return the ARN string. Handle 403 with clear error about `InstallerDownload` scope.
- [x] 1.7 Add layer ARN cache: `map[string]string` keyed by `"{techtype}-{arch}"` to avoid redundant API calls
- [x] 1.8 Implement `classifyFunction(fn lambdaFunction)` — return action: `"new"`, `"update"`, or `"skip"` based on whether a Dynatrace layer is already present and whether the runtime is supported
- [x] 1.9 Implement `printPreviewTable(functions []lambdaFunction, actions []string)` — render the preview table showing function name, runtime, arch, and action
- [x] 1.10 Implement `instrumentFunction(fn lambdaFunction, layerARN string, connInfo dtConnectionInfo, envURL, token string)` — read current config via `aws lambda get-function-configuration`, merge DT_* env vars, update layers list (replace existing DT layer or append), call `aws lambda update-function-configuration`
- [x] 1.11 Wire the full `InstallAWSLambda` flow: validate -> list functions -> resolve layer ARNs -> preview -> confirm -> instrument sequentially -> summary
- [x] 1.12 Add tests: `mapRuntimeToTechtype` mapping, `classifyFunction` logic (new/update/skip), env var merging preserves existing vars, layer replacement logic

## 2. Lambda uninstrumentation

Implement `UninstallAWSLambda()` in the same file. Depends on task 1 (reuses helpers).

**Files:** `pkg/installer/aws_lambda.go` (modify)

- [x] 2.1 Implement `UninstallAWSLambda(dryRun bool) error` — list functions, filter to instrumented ones, preview, confirm, remove layer + DT_* env vars
- [x] 2.2 Implement `isInstrumented(fn lambdaFunction) bool` — check if any layer ARN contains `Dynatrace_OneAgent`
- [x] 2.3 Implement `uninstrumentFunction(fn lambdaFunction)` — remove DT layer from layers list, remove DT_* env vars from env map, call `aws lambda update-function-configuration`
- [x] 2.4 Add tests: `isInstrumented` detection, env var removal preserves non-DT vars, empty env map after removal

## 3. Cobra command registration

Register `install aws-lambda` and `uninstall aws-lambda` subcommands following existing patterns.

**Files:** `cmd/install.go` (modify), `cmd/uninstall.go` (modify)

- [x] 3.1 Add `installAWSLambdaCmd` in `cmd/install.go` — `Use: "aws-lambda"`, `Short: "Install Dynatrace Lambda Layer on all functions"`, `Args: cobra.NoArgs`. RunE: resolve creds via `getDtEnvironment()`, call `validateCredentials()`, call `installer.InstallAWSLambda(envURL, token, platformToken, installDryRun)`
- [x] 3.2 Register `installAWSLambdaCmd` under `installCmd` in `init()`
- [x] 3.3 Add `uninstallAWSLambdaCmd` in `cmd/uninstall.go` — `Use: "aws-lambda"`, `Short: "Remove Dynatrace Lambda Layer from all functions"`, `Args: cobra.NoArgs`. RunE: call `installer.UninstallAWSLambda(uninstallDryRun)`
- [x] 3.4 Register `uninstallAWSLambdaCmd` under `uninstallCmd` in `init()`

## 4. Parallel execution from `install aws`

Modify `InstallAWS` to spawn `InstallAWSLambda` concurrently. Depends on task 1.

**Files:** `pkg/installer/aws.go` (modify)

- [x] 4.1 Add `sync.WaitGroup` and error channel to `InstallAWS()`
- [x] 4.2 Spawn `InstallAWSLambda()` in a goroutine before the CloudFormation deployment
- [x] 4.3 After CloudFormation completes, wait for the Lambda goroutine via `wg.Wait()`
- [x] 4.4 Log Lambda goroutine errors as warnings (non-fatal to the overall `install aws` flow)
- [x] 4.5 Ensure `--dry-run` is passed through to the Lambda goroutine

## 5. Integration testing and verification

End-to-end verification of all flows.

**Files:** `pkg/installer/aws_lambda_test.go` (create or extend)

- [x] 5.1 Run `make test` — all existing tests must pass
- [x] 5.2 Run `make lint` — no new lint issues
- [ ] 5.3 Manual verification: `dtwiz install aws-lambda --dry-run` shows preview without changes
- [ ] 5.4 Manual verification: `dtwiz install aws --dry-run` shows both CloudFormation and Lambda previews
- [ ] 5.5 Manual verification: `dtwiz uninstall aws-lambda --dry-run` shows cleanup preview
