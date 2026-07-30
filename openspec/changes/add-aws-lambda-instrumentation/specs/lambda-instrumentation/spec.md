# Lambda Instrumentation

## ADDED Requirements

### Requirement: Lambda function discovery

The system SHALL list all Lambda functions in the current AWS region using `aws lambda list-functions` (JSON output) with `NextMarker`-based pagination. The current region is resolved via `GetAWSCallerInfo()` (dtctl SDK / AWS caller identity), not `aws configure get region`. Functions with `PackageType: Image` (container images) SHALL be excluded since layers cannot be attached to them.

#### Scenario: Functions found in current region

- **GIVEN** the AWS CLI is configured with valid credentials and a resolvable region
- **WHEN** `InstallAWSLambda` lists Lambda functions
- **THEN** all Zip-packaged functions in the current region are returned with their runtime, architecture, existing layers, and existing environment variables

#### Scenario: No functions found

- **GIVEN** the AWS CLI is configured with valid credentials
- **WHEN** no Lambda functions exist in the current region
- **THEN** the system prints "No Lambda functions found in region {region}" and exits without error

#### Scenario: Container image functions excluded

- **GIVEN** some Lambda functions use `PackageType: Image`
- **WHEN** the function list is built
- **THEN** container image functions are silently excluded from the list and never appear in the preview or instrumentation loop

### Requirement: Runtime-to-techtype mapping

The system SHALL map each function's AWS runtime prefix to a Dynatrace `techtype` parameter: `nodejs*` -> `nodejs`, `python*` -> `python`, `java*` -> `java`, `go*` -> `go`. Functions whose runtime matches none of these prefixes (`dotnet*`, `provided*`, or unknown) SHALL be classified as `skip` and shown in the preview table with the reason `skip (unsupported runtime)`.

> Note: the pre-execution info box currently advertises Node.js and Python support only, while the mapping also covers Java and Go. This is a known cosmetic inconsistency in the info box text, not in the mapping behavior.

#### Scenario: Supported runtime mapped

- **GIVEN** a Lambda function has runtime `nodejs18.x`
- **WHEN** the system resolves the techtype
- **THEN** it maps to `nodejs` for the DT layer ARN API call

#### Scenario: Unsupported runtime skipped

- **GIVEN** a Lambda function has runtime `dotnet6`
- **WHEN** the system classifies the function
- **THEN** the function's action is `skip`, it is rendered in the preview as `skip (unsupported runtime)`, and it is not modified

### Requirement: Skip Dynatrace-managed functions

The system SHALL never instrument Dynatrace-managed Lambda functions. Any function whose name contains `DynatraceApiClientFunction` SHALL be classified as `skip` (reason: `Dynatrace internal`) during instrumentation and SHALL be excluded from the uninstrumentation candidate list.

#### Scenario: Dynatrace-internal function skipped

- **GIVEN** a function named `dynatrace-...-DynatraceApiClientFunction-...`
- **WHEN** the system classifies functions for instrumentation
- **THEN** the function's action is `skip` with reason `Dynatrace internal` and it is never modified

### Requirement: Layer ARN resolution via Dynatrace API

The system SHALL resolve the correct Lambda layer ARN by calling `GET /api/v1/deployment/lambda/layer` on the Classic API URL (derived via `APIURL()`) with query parameters `arch`, `techtype`, `region`, and `withCollector=excluded`. Architecture is translated for the API: `arm64` -> `arm`, everything else -> `x86`. Requires `InstallerDownload` token scope. Resolved ARNs SHALL be cached by `"{techtype}-{arch}"` key within a single invocation to avoid redundant API calls. The first ARN in the response `arns` array is used.

#### Scenario: Layer ARN resolved successfully

- **GIVEN** a function with runtime `python3.11` and architecture `arm64` in region `eu-central-1`
- **WHEN** the system queries the DT layer API with `techtype=python&arch=arm&region=eu-central-1&withCollector=excluded`
- **THEN** it receives a valid ARN and caches it under key `python-arm64` for subsequent functions with the same techtype and architecture

#### Scenario: Layer ARN cached for same techtype/arch

- **GIVEN** two functions both have runtime `nodejs18.x` and architecture `x86_64`
- **WHEN** the second function's layer ARN is resolved
- **THEN** the cached ARN from the first resolution is used without an additional API call

#### Scenario: API returns 403 (insufficient scope)

- **GIVEN** the access token does not have `InstallerDownload` scope
- **WHEN** the layer ARN API is called
- **THEN** the system returns a clear error: "access token needs InstallerDownload scope for Lambda layer resolution (HTTP 403)"

#### Scenario: No ARN found

- **GIVEN** the layer API returns an empty `arns` array for the requested techtype/arch/region
- **WHEN** the system attempts to resolve the ARN
- **THEN** it returns an error "no Lambda layer ARN found for techtype={techtype} arch={arch} region={region}"

### Requirement: DT connection info retrieval

The system SHALL assemble the Dynatrace connection details used for the function environment variables from three Classic API calls:

- `GET /api/v1/deployment/installer/agent/connectioninfo` -> `tenantUUID` (used as `DT_TENANT`).
- `GET /api/v1/config/clusterid` -> numeric `clusterId` (used as `DT_CLUSTER`).
- `GET /api/v2/agentConnectionToken` -> agent connection token `dt0a01.*` (used as `DT_CONNECTION_AUTH_TOKEN`). Requires the `environment-api:agent-connection-tokens:read` scope.

The `DT_CONNECTION_BASE_URL` is the Classic API URL derived from the environment URL via `APIURL()`. All three calls authenticate with the platform token via `AuthHeader()`.

#### Scenario: Connection info retrieved

- **GIVEN** a valid platform token with the required scopes
- **WHEN** the connection info, cluster ID, and agent connection token endpoints are called
- **THEN** `DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_AUTH_TOKEN`, and `DT_CONNECTION_BASE_URL` are all populated for the function environment variables

#### Scenario: Agent connection token scope missing

- **GIVEN** the platform token lacks `environment-api:agent-connection-tokens:read`
- **WHEN** the agent connection token endpoint returns 403
- **THEN** the system returns a clear error: "platform token needs environment-api:agent-connection-tokens:read scope (HTTP 403)"

### Requirement: Environment variable merging

The system SHALL read each function's current configuration via `aws lambda get-function-configuration`, merge the Dynatrace env vars into the existing environment variables (preserving all non-DT vars), and write the merged config back via `aws lambda update-function-configuration`. The following env vars SHALL be set on every instrumented function: `AWS_LAMBDA_EXEC_WRAPPER=/opt/dynatrace`, `DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_BASE_URL`, `DT_CONNECTION_AUTH_TOKEN`. Additionally, for Node.js runtimes only, `DT_ENABLE_ESM_LOADERS=true` SHALL be set.

#### Scenario: Function with existing env vars

- **GIVEN** a Lambda function has existing environment variables `DATABASE_URL=postgres://...` and `LOG_LEVEL=info`
- **WHEN** the system instruments the function (non-Node.js runtime)
- **THEN** the updated configuration contains `DATABASE_URL`, `LOG_LEVEL`, AND the five base DT_* env vars

#### Scenario: Node.js function gets ESM loader flag

- **GIVEN** a Lambda function has runtime `nodejs20.x`
- **WHEN** the system instruments the function
- **THEN** the updated configuration additionally contains `DT_ENABLE_ESM_LOADERS=true`

#### Scenario: Function with no existing env vars

- **GIVEN** a non-Node.js Lambda function has no environment variables
- **WHEN** the system instruments the function
- **THEN** the updated configuration contains only the five base DT_* env vars

### Requirement: Layer attachment

The system SHALL add the Dynatrace Lambda Layer to the function's layer list. If a Dynatrace layer is already present (any ARN containing `Dynatrace_OneAgent`), it SHALL be replaced in place with the latest version. Other layers SHALL be preserved. Layers are re-read from the live function configuration at instrumentation time.

#### Scenario: New instrumentation

- **GIVEN** a function has layers `[arn:aws:lambda:...:layer:my-custom-layer:3]`
- **WHEN** the system instruments the function
- **THEN** the updated layers list is `[arn:aws:lambda:...:layer:my-custom-layer:3, arn:aws:lambda:...:layer:Dynatrace_OneAgent_...:1]`

#### Scenario: Update existing instrumentation

- **GIVEN** a function already has a layer with ARN containing `Dynatrace_OneAgent` (older version)
- **WHEN** the system instruments the function
- **THEN** the old Dynatrace layer is replaced in place with the latest version; other layers are unchanged

### Requirement: Sequential processing with error resilience

The system SHALL process functions one at a time. If instrumentation fails for a single function (e.g., function is in a pending state), the error is printed inline and processing continues with the next function. The final summary reports successes and failures.

#### Scenario: One function fails, others succeed

- **GIVEN** 5 Lambda functions to instrument
- **WHEN** function 3 fails with "ResourceConflictException" (function being updated)
- **THEN** functions 1, 2, 4, 5 are instrumented successfully, function 3's error is printed, and the summary shows "4 succeeded, 1 failed"

### Requirement: Preview, confirmation, and dry-run

The system SHALL display a preview table showing: function name, runtime, architecture, and action (`new`/`update`/`skip (reason)`), followed by a count of functions to instrument and functions skipped. When there are no actionable functions, the system prints "No functions to instrument." and exits. Under `--dry-run`, the preview is displayed but no changes are applied and no confirmation prompt is shown. When invoked standalone (confirm enabled), the system prompts `Apply?` and proceeds only on confirmation, returning `ErrInstallCancelled` on decline.

#### Scenario: Dry run preview

- **GIVEN** `--dry-run` is set
- **WHEN** `InstallAWSLambda` runs
- **THEN** the preview table is printed, no functions are modified, and no confirmation prompt appears

#### Scenario: Normal run with confirmation

- **GIVEN** `--dry-run` is NOT set and the installer is invoked standalone
- **WHEN** `InstallAWSLambda` displays the preview
- **THEN** it prompts "Apply?" and proceeds only on confirmation; on decline it returns `ErrInstallCancelled`

#### Scenario: Nothing actionable

- **GIVEN** every discovered function is classified as `skip`
- **WHEN** the preview is rendered
- **THEN** the system prints "No functions to instrument." and exits without prompting
