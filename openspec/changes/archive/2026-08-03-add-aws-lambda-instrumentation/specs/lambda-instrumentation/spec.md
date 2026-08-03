# Lambda Instrumentation

## ADDED Requirements

### Requirement: Lambda function discovery

The system SHALL discover all Lambda functions in the current AWS region. Functions deployed as container images SHALL be excluded, since a layer cannot be attached to them.

#### Scenario: Functions found in current region

- **GIVEN** the AWS CLI is configured with valid credentials and a resolvable region
- **WHEN** the installer discovers Lambda functions
- **THEN** all Zip-packaged functions in the current region are returned with their runtime, architecture, layers, and environment variables

#### Scenario: No functions found

- **GIVEN** the AWS CLI is configured with valid credentials
- **WHEN** no Lambda functions exist in the current region
- **THEN** the system reports that no functions were found and exits without error

#### Scenario: Container image functions excluded

- **GIVEN** some Lambda functions are deployed as container images
- **WHEN** the function list is built
- **THEN** those functions are excluded and never appear in the preview or instrumentation loop

### Requirement: Supported runtimes

The system SHALL instrument functions whose runtime is Node.js, Python, Java, or Go. Functions with any other runtime (e.g. .NET, custom/`provided`) SHALL be skipped and shown in the preview with the reason `skip (unsupported runtime)`.

#### Scenario: Supported runtime instrumented

- **GIVEN** a Lambda function has a Node.js runtime
- **WHEN** the system classifies the function
- **THEN** its action is `new` or `update` and the correct Dynatrace layer for that runtime is used

#### Scenario: Unsupported runtime skipped

- **GIVEN** a Lambda function has a .NET runtime
- **WHEN** the system classifies the function
- **THEN** its action is `skip`, it is rendered in the preview as `skip (unsupported runtime)`, and it is not modified

### Requirement: Skip Dynatrace-managed functions

The system SHALL never instrument Dynatrace-managed Lambda functions. Such functions SHALL be classified as `skip` (reason: `Dynatrace internal`) and SHALL be excluded from the uninstrumentation candidate list.

#### Scenario: Dynatrace-internal function skipped

- **GIVEN** a Dynatrace-managed function
- **WHEN** the system classifies functions for instrumentation
- **THEN** its action is `skip` with reason `Dynatrace internal` and it is never modified

### Requirement: Layer resolution

The system SHALL resolve the correct Dynatrace Lambda layer for each function's runtime, architecture, and region from Dynatrace. When the credentials lack the scope required to resolve a layer, or no layer exists for the requested runtime/architecture/region, the system SHALL return a clear, actionable error.

#### Scenario: Layer resolved successfully

- **GIVEN** a function with a supported runtime and known architecture in the current region
- **WHEN** the system resolves its layer
- **THEN** it obtains the correct layer for that runtime, architecture, and region

#### Scenario: Insufficient token scope

- **GIVEN** the token lacks the scope required to resolve a Lambda layer
- **WHEN** the system attempts to resolve a layer
- **THEN** it returns a clear error stating which scope is required

#### Scenario: No layer available

- **GIVEN** no Dynatrace layer exists for the requested runtime, architecture, and region
- **WHEN** the system attempts to resolve a layer
- **THEN** it returns an error naming the runtime, architecture, and region for which no layer was found

### Requirement: Dynatrace connection details

The system SHALL obtain the Dynatrace connection details needed by an instrumented function (tenant, cluster, connection base URL, and connection auth token) from Dynatrace. When the token lacks the scope required to obtain any of these, the system SHALL return a clear, actionable error.

#### Scenario: Connection details retrieved

- **GIVEN** a token with the required scopes
- **WHEN** the system gathers connection details
- **THEN** `DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_BASE_URL`, and `DT_CONNECTION_AUTH_TOKEN` are all populated for the function environment variables

#### Scenario: Missing scope

- **GIVEN** the token lacks a scope required to obtain a connection detail
- **WHEN** the system attempts to gather connection details
- **THEN** it returns a clear error stating which scope is required

### Requirement: Environment variable configuration

The system SHALL set the Dynatrace environment variables on each instrumented function while preserving all of the function's existing (non-Dynatrace) environment variables. The variables set on every instrumented function are `AWS_LAMBDA_EXEC_WRAPPER`, `DT_TENANT`, `DT_CLUSTER`, `DT_CONNECTION_BASE_URL`, and `DT_CONNECTION_AUTH_TOKEN`. For Node.js functions, `DT_ENABLE_ESM_LOADERS=true` SHALL additionally be set.

#### Scenario: Function with existing env vars

- **GIVEN** a non-Node.js function has existing, unrelated environment variables
- **WHEN** the system instruments the function
- **THEN** the updated configuration retains all existing variables AND adds the five base Dynatrace variables

#### Scenario: Node.js function gets ESM loader flag

- **GIVEN** a Node.js function
- **WHEN** the system instruments the function
- **THEN** the updated configuration additionally contains `DT_ENABLE_ESM_LOADERS=true`

#### Scenario: Function with no existing env vars

- **GIVEN** a non-Node.js function with no environment variables
- **WHEN** the system instruments the function
- **THEN** the updated configuration contains only the five base Dynatrace variables

### Requirement: Layer attachment

The system SHALL attach the Dynatrace Lambda layer to the function, preserving the function's other layers. If a Dynatrace layer is already attached, it SHALL be replaced in place with the resolved version.

#### Scenario: New instrumentation

- **GIVEN** a function has one custom layer and no Dynatrace layer
- **WHEN** the system instruments the function
- **THEN** the custom layer is preserved and the Dynatrace layer is added

#### Scenario: Update existing instrumentation

- **GIVEN** a function already has an older Dynatrace layer
- **WHEN** the system instruments the function
- **THEN** the Dynatrace layer is replaced in place with the resolved version and other layers are unchanged

### Requirement: Error resilience

The system SHALL continue instrumenting the remaining functions when instrumentation of a single function fails. The failure SHALL be reported, and the final summary SHALL report the number of successes and failures.

#### Scenario: One function fails, others succeed

- **GIVEN** five Lambda functions to instrument
- **WHEN** one of them fails (e.g. it is being updated by another process)
- **THEN** the other four are instrumented successfully, the failing function's error is reported, and the summary shows four succeeded and one failed

### Requirement: Preview, confirmation, and dry-run

The system SHALL display a preview of the functions it will act on, showing function name, runtime, architecture, and action (`new`/`update`/`skip (reason)`), followed by counts of functions to instrument and functions skipped. When no functions are actionable, the system SHALL report that there is nothing to instrument and exit. Under `--dry-run`, the preview is shown but no changes are applied and no confirmation is requested. When run standalone, the system SHALL request confirmation before applying and cancel cleanly on decline.

#### Scenario: Dry run preview

- **GIVEN** `--dry-run` is set
- **WHEN** the installer runs
- **THEN** the preview is shown, no functions are modified, and no confirmation is requested

#### Scenario: Normal run with confirmation

- **GIVEN** `--dry-run` is not set and the installer runs standalone
- **WHEN** the preview is shown
- **THEN** the system requests confirmation and proceeds only on confirmation; on decline it cancels cleanly

#### Scenario: Nothing actionable

- **GIVEN** every discovered function is classified as `skip`
- **WHEN** the preview is rendered
- **THEN** the system reports there is nothing to instrument and exits without requesting confirmation
