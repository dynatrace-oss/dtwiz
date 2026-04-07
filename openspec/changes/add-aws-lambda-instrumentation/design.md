# Design

## Context

`dtwiz install aws` in `pkg/installer/aws.go` deploys a CloudFormation stack for AWS platform monitoring. It already detects the AWS account via `aws sts get-caller-identity`, creates a Dynatrace monitoring configuration via the Classic API, and deploys a CloudFormation template. The analyzer (`pkg/analyzer/detect_aws.go`) already detects Lambda functions and counts them. The codebase has no Lambda-specific installer.

The Dynatrace Lambda Layer is a per-function extension that provides distributed tracing and code-level visibility. It requires: (1) the correct layer ARN (varies by region, runtime, architecture), (2) environment variables on the function (`DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_BASE_URL`, `DT_CONNECTION_AUTH_TOKEN`, `AWS_LAMBDA_EXEC_WRAPPER`).

## Goals / Non-Goals

**Goals:**

- Implement `InstallAWSLambda()` that instruments all Lambda functions in the current AWS region with the Dynatrace Lambda Layer — zero manual configuration.
- Implement `UninstallAWSLambda()` that cleanly removes instrumentation from all functions.
- Run `InstallAWSLambda` concurrently from `InstallAWS` so users get platform + function-level monitoring in one command.
- Register `install aws-lambda` and `uninstall aws-lambda` as Cobra subcommands following existing patterns.
- Support `--dry-run` for both install and uninstall.
- Handle idempotency: re-running updates already-instrumented functions to the latest layer version.

**Non-Goals:**

- Multi-region instrumentation (only the current AWS CLI region).
- Adding `aws-lambda` as a separate recommendation in the `setup` menu or recommender.
- Creating/managing DT API tokens — reuses the existing `DT_ACCESS_TOKEN`.
- Supporting Lambda functions deployed as container images (layer attachment doesn't apply).

## Decisions

### 1. All logic in a single file: `aws_lambda.go`

All Lambda instrumentation code lives in `pkg/installer/aws_lambda.go`. This follows the project convention where each method has its own file (`aws.go`, `otel.go`, `kubernetes.go`, etc.). Exported functions: `InstallAWSLambda()` and `UninstallAWSLambda()`.

### 2. Layer ARN resolution via Dynatrace API

The correct layer ARN is resolved dynamically by calling:

```text
GET /api/v1/deployment/lambda/layer?arch={arm|x86}&techtype={runtime}&region={region}&withCollector=excluded
```

This endpoint requires the `InstallerDownload` token scope and returns:

```json
{
  "arns": [
    {
      "arch": "arm",
      "arn": "arn:aws:lambda:eu-central-1:657959507023:layer:Dynatrace_OneAgent_..._nodejs_arm:1",
      "region": "eu-central-1",
      "techType": "nodejs",
      "withCollector": "included"
    }
  ]
}
```

The installer caches layer ARNs by `(runtime, arch)` tuple during a single invocation to avoid redundant API calls.

**Alternative considered:** Hardcoding the Dynatrace AWS account ID and using `aws lambda list-layers` to discover layers. Rejected because the version/date component in the ARN changes with each Dynatrace release and listing would return all versions, not just the latest.

### 3. Connection info from Dynatrace API

The installer calls `/api/v1/deployment/installer/agent/connectioninfo` to obtain:

- `tenantUUID` → used as `DT_TENANT` env var
- Cluster ID → used as `DT_CLUSTER` env var (exact field to be determined at implementation time; may be embedded in communication endpoints or require parsing)

The `DT_CONNECTION_BASE_URL` is derived from the environment URL using the existing `ClassicAPIURL()` helper (strip `.apps.` if present).

The `DT_CONNECTION_AUTH_TOKEN` is the user's access token (reused as-is).

### 4. Runtime-to-techtype mapping

Each Lambda function's `Runtime` field (e.g., `nodejs18.x`, `python3.11`, `java17`) is mapped to the DT API's `techtype` parameter:

| AWS Runtime prefix | DT techtype |
|---------------------|-------------|
| `nodejs` | `nodejs` |
| `python` | `python` |
| `java` | `java` |
| `go` | `go` |
| `dotnet` | *skip with warning* |
| `provided` | *skip with warning* |

Functions with unsupported or custom runtimes are skipped with a warning message.

**Alternative considered:** Supporting `.NET` via the log-collection-only layer. Deferred — .NET Lambda support in Dynatrace is limited and the `techtype` parameter doesn't include `dotnet` for the layer endpoint.

### 5. Environment variable merging

`aws lambda update-function-configuration` replaces the entire `Environment.Variables` map. To avoid destroying existing env vars:

1. Call `aws lambda get-function-configuration` to read the current config.
2. Merge DT_* env vars into the existing map (add new keys, overwrite DT_* keys).
3. Replace the Dynatrace layer in the `Layers` list (identified by ARN containing `Dynatrace_OneAgent`), or append if not present.
4. Call `aws lambda update-function-configuration` with the merged config.

This is implemented using the AWS CLI with `--environment` and `--layers` flags, passing the full merged values.

### 6. Idempotency: update to latest layer

When a function already has a Dynatrace layer attached (detected by checking if any layer ARN contains `Dynatrace_OneAgent`):

- The existing layer is replaced with the latest version from the DT API.
- DT_* env vars are updated to current values.
- The preview table marks these functions as "update" vs "new".

### 7. Sequential processing

Functions are processed one at a time. This provides:

- Clear, sequential output (function name + status per line)
- No risk of hitting AWS API rate limits
- Simpler error handling (fail on one function, continue with the rest)

Each function's update is independent — a failure on one function logs an error and continues to the next.

### 8. Parallel execution from `install aws`

`InstallAWS()` spawns `InstallAWSLambda()` in a goroutine at the start of its flow. Both run concurrently:

```text
InstallAWS()
├─ go InstallAWSLambda(...)    ← concurrent
├─ Create DT monitoring config  ← existing flow
├─ Deploy CloudFormation stack   ← existing flow
└─ Wait for Lambda goroutine    ← sync.WaitGroup
```

Both the CloudFormation deployment and Lambda instrumentation have their own confirmation prompts and previews. Since they run concurrently, the Lambda flow runs independently — it performs its own validation, preview, confirmation, and execution. Error from the Lambda goroutine is logged but does not block the CloudFormation deployment.

**Note:** The parallel execution uses a separate goroutine with its own output. The terminal output may interleave. This is acceptable for the first iteration; output synchronization can be refined later if needed.

### 9. Uninstall flow

`UninstallAWSLambda()`:

1. Lists all Lambda functions in the current region.
2. Filters to functions that have a Dynatrace layer (ARN contains `Dynatrace_OneAgent`).
3. Shows a preview of functions to be cleaned up.
4. For each function: removes the Dynatrace layer from the layers list, removes DT_* env vars (`DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_BASE_URL`, `DT_CONNECTION_AUTH_TOKEN`, `AWS_LAMBDA_EXEC_WRAPPER`), preserves everything else.
5. Calls `aws lambda update-function-configuration` with the cleaned config.

### 10. Preview and dry-run

The preview shows a table:

```text
  Lambda functions to instrument:
  ──────────────────────────────────────────────────────────────────
  Function                Runtime      Arch    Action
  ──────────────────────────────────────────────────────────────────
  my-api-handler          nodejs18.x   arm64   new
  data-processor          python3.11   x86_64  new
  auth-service            java17       arm64   update
  legacy-func             dotnet6      x86_64  skip (unsupported)
  ──────────────────────────────────────────────────────────────────
  3 functions to instrument, 1 skipped
```

Under `--dry-run`, the preview is printed but no changes are applied and no confirmation prompt is shown.

### 11. Cobra command registration

Following existing patterns in `cmd/install.go` and `cmd/uninstall.go`:

- `installAWSLambdaCmd` registered under `installCmd` with `Use: "aws-lambda"` and `Args: cobra.NoArgs`.
- `uninstallAWSLambdaCmd` registered under `uninstallCmd` with the same pattern.
- Both use the parent's `--dry-run` persistent flag.

### 12. No recommender changes

`MethodAWSLambda` is NOT added to the recommender. The `setup` flow does not show it as a separate option. Lambda instrumentation is either:

- Triggered automatically by `install aws` (parallel).
- Run manually via `install aws-lambda`.

## Risks / Trade-offs

- **[Interleaved terminal output]** → Running Lambda instrumentation concurrently with CloudFormation deployment may produce interleaved output. Mitigation: acceptable for first iteration; can add output buffering later.
- **[DT_CLUSTER resolution uncertainty]** → The exact source of the numeric cluster ID in the connection info API response is unclear from docs. Mitigation: inspect the actual API response at implementation time; fall back to parsing communication endpoints if needed.
- **[Token scope requirement]** → The access token needs `InstallerDownload` scope for the layer ARN API, which may not be present on all user tokens. Mitigation: clear error message if the API returns 403.
- **[AWS API rate limits]** → Sequential processing of many functions (hundreds) could be slow. Mitigation: sequential processing avoids rate limit issues; concurrent processing can be added later if performance is a concern.
- **[Container image Lambda functions]** → Functions deployed as container images cannot have layers attached. Mitigation: detect `PackageType: Image` and skip with a warning.
- **[Function update conflicts]** → `update-function-configuration` may fail if the function is being updated by another process or is in a pending state. Mitigation: log the error and continue to the next function.
