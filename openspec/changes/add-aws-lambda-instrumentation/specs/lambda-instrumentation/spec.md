# Lambda Instrumentation

## ADDED Requirements

### Requirement: Lambda function discovery

The system SHALL list all Lambda functions in the current AWS region using `aws lambda list-functions` with pagination support. Functions with `PackageType: Image` (container images) SHALL be excluded since layers cannot be attached to them.

#### Scenario: Functions found in current region

- **GIVEN** the AWS CLI is configured with valid credentials and a default region
- **WHEN** `InstallAWSLambda` lists Lambda functions
- **THEN** all functions in the current region are returned with their runtime, architecture, and existing layers

#### Scenario: No functions found

- **GIVEN** the AWS CLI is configured with valid credentials
- **WHEN** no Lambda functions exist in the current region
- **THEN** the system prints "No Lambda functions found in region {region}" and exits without error

#### Scenario: Container image functions excluded

- **GIVEN** some Lambda functions use `PackageType: Image`
- **WHEN** the function list is built
- **THEN** container image functions are excluded from instrumentation and noted in the summary

### Requirement: Runtime-to-techtype mapping

The system SHALL map each function's AWS runtime to a Dynatrace `techtype` parameter: `nodejs*` -> `nodejs`, `python*` -> `python`, `java*` -> `java`, `go*` -> `go`. Functions with unsupported runtimes (`dotnet*`, `provided*`, or unknown) SHALL be skipped with a warning.

#### Scenario: Supported runtime mapped

- **GIVEN** a Lambda function has runtime `nodejs18.x`
- **WHEN** the system resolves the techtype
- **THEN** it maps to `nodejs` for the DT layer ARN API call

#### Scenario: Unsupported runtime skipped

- **GIVEN** a Lambda function has runtime `dotnet6`
- **WHEN** the system attempts to resolve the techtype
- **THEN** the function is skipped and a warning is printed: "Skipping {function-name}: unsupported runtime dotnet6"

### Requirement: Layer ARN resolution via Dynatrace API

The system SHALL resolve the correct Lambda layer ARN by calling `GET /api/v1/deployment/lambda/layer` on the Classic API URL with query parameters `arch`, `techtype`, `region`, and `withCollector=included`. Requires `InstallerDownload` token scope. Layer ARNs SHALL be cached by `(runtime, arch)` tuple within a single invocation to avoid redundant API calls.

#### Scenario: Layer ARN resolved successfully

- **GIVEN** a function with runtime `python3.11` and architecture `arm64` in region `eu-central-1`
- **WHEN** the system queries the DT layer API with `techtype=python&arch=arm&region=eu-central-1&withCollector=included`
- **THEN** it receives a valid ARN and caches it for subsequent functions with the same runtime and architecture

#### Scenario: Layer ARN cached for same runtime/arch

- **GIVEN** two functions both have runtime `nodejs18.x` and architecture `x86_64`
- **WHEN** the second function's layer ARN is resolved
- **THEN** the cached ARN from the first resolution is used without an additional API call

#### Scenario: API returns 403 (insufficient scope)

- **GIVEN** the access token does not have `InstallerDownload` scope
- **WHEN** the layer ARN API is called
- **THEN** the system prints a clear error: "Access token needs InstallerDownload scope for Lambda layer resolution" and exits

### Requirement: DT connection info retrieval

The system SHALL call `GET /api/v1/deployment/installer/agent/connectioninfo` to obtain the `tenantUUID` (used as `DT_TENANT`) and the cluster ID (used as `DT_CLUSTER`). The `DT_CONNECTION_BASE_URL` is derived from the environment URL using `ClassicAPIURL()`. The `DT_CONNECTION_AUTH_TOKEN` is the user's access token.

#### Scenario: Connection info retrieved

- **GIVEN** a valid access token with `InstallerDownload` scope
- **WHEN** the connection info API is called
- **THEN** `tenantUUID` and cluster ID are extracted and used for the function environment variables

### Requirement: Environment variable merging

The system SHALL read each function's current configuration via `aws lambda get-function-configuration`, merge DT_* env vars into the existing environment variables (preserving all non-DT vars), and write the merged config back. The following env vars SHALL be set: `AWS_LAMBDA_EXEC_WRAPPER=/opt/dynatrace`, `DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_BASE_URL`, `DT_CONNECTION_AUTH_TOKEN`.

#### Scenario: Function with existing env vars

- **GIVEN** a Lambda function has existing environment variables `DATABASE_URL=postgres://...` and `LOG_LEVEL=info`
- **WHEN** the system instruments the function
- **THEN** the updated configuration contains `DATABASE_URL`, `LOG_LEVEL`, AND the five DT_* env vars

#### Scenario: Function with no existing env vars

- **GIVEN** a Lambda function has no environment variables
- **WHEN** the system instruments the function
- **THEN** the updated configuration contains only the five DT_* env vars

### Requirement: Layer attachment

The system SHALL add the Dynatrace Lambda Layer to the function's layer list. If a Dynatrace layer is already present (ARN contains `Dynatrace_OneAgent`), it SHALL be replaced with the latest version. Other layers SHALL be preserved.

#### Scenario: New instrumentation

- **GIVEN** a function has layers `[arn:aws:lambda:...:layer:my-custom-layer:3]`
- **WHEN** the system instruments the function
- **THEN** the updated layers list is `[arn:aws:lambda:...:layer:my-custom-layer:3, arn:aws:lambda:...:layer:Dynatrace_OneAgent_...:1]`

#### Scenario: Update existing instrumentation

- **GIVEN** a function already has a layer with ARN containing `Dynatrace_OneAgent` (older version)
- **WHEN** the system instruments the function
- **THEN** the old Dynatrace layer is replaced with the latest version; other layers are unchanged

### Requirement: Sequential processing with error resilience

The system SHALL process functions one at a time. If instrumentation fails for a single function (e.g., function is in a pending state), the error is logged and processing continues with the next function. The final summary reports successes and failures.

#### Scenario: One function fails, others succeed

- **GIVEN** 5 Lambda functions to instrument
- **WHEN** function 3 fails with "ResourceConflictException" (function being updated)
- **THEN** functions 1, 2, 4, 5 are instrumented successfully, function 3's error is logged, and the summary shows "4 succeeded, 1 failed"

### Requirement: Preview and dry-run

The system SHALL display a preview table showing: function name, runtime, architecture, and action (new/update/skip). Under `--dry-run`, the preview is displayed but no changes are applied and no confirmation prompt is shown.

#### Scenario: Dry run preview

- **GIVEN** `--dry-run` is set
- **WHEN** `InstallAWSLambda` runs
- **THEN** the preview table is printed, no functions are modified, and no confirmation prompt appears

#### Scenario: Normal run with confirmation

- **GIVEN** `--dry-run` is NOT set
- **WHEN** `InstallAWSLambda` displays the preview
- **THEN** it prompts "Apply? [Y/n]" and proceeds only on confirmation
