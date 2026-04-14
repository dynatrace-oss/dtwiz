# Node.js OTel Instrumentation

## ADDED Requirements

### Requirement: .otel/ directory creation

The system SHALL create a `.otel/` directory inside the project root containing a generated `package.json` with OTel dependencies. The app's `package.json` SHALL never be modified.

#### Scenario: .otel/ directory created

- **GIVEN** a Node.js project at `/path/to/myapp`
- **WHEN** the installer creates the OTel directory
- **THEN** `/path/to/myapp/.otel/` is created
- **AND** `/path/to/myapp/.otel/package.json` exists with `@opentelemetry/auto-instrumentations-node` and all OTel exporter packages as dependencies

#### Scenario: .otel/ already exists

- **GIVEN** a `.otel/` directory already exists from a previous install
- **WHEN** the installer runs
- **THEN** the existing `.otel/` is reused (package.json is overwritten, `npm install` runs again)

#### Scenario: App package.json untouched

- **GIVEN** the project has an existing `package.json` with application dependencies
- **WHEN** the installer completes
- **THEN** the app's `package.json` is byte-identical to before

### Requirement: OTel package installation

The system SHALL run `npm install` inside the `.otel/` directory to install OTel packages. npm is always used regardless of the project's own package manager.

#### Scenario: npm install succeeds

- **GIVEN** the `.otel/package.json` has been written
- **WHEN** `npm install` is run inside `.otel/`
- **THEN** `.otel/node_modules/` contains the OTel packages and the command exits successfully

#### Scenario: npm install fails

- **GIVEN** npm is not on PATH or the install fails
- **WHEN** the installer attempts `npm install`
- **THEN** a clear error is shown with the exact command that failed

#### Scenario: npm prerequisite check

- **GIVEN** the user's system does not have `npm` on PATH
- **WHEN** `InstallOtelNode()` starts
- **THEN** the installer exits with a clear error indicating npm is required

### Requirement: Node.js env var generation

The system SHALL generate OTEL\_\* environment variables for Node.js including the `OTEL_NODE_RESOURCE_DETECTORS=all` recommended by Dynatrace.

#### Scenario: Env vars include resource detectors

- **WHEN** `generateOtelNodeEnvVars()` is called
- **THEN** the returned map includes `OTEL_NODE_RESOURCE_DETECTORS=all` in addition to all base OTEL\_\* vars

#### Scenario: Env vars include standard OTEL vars

- **WHEN** `generateOtelNodeEnvVars()` is called with apiURL, token, and serviceName
- **THEN** the returned map includes `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_PROTOCOL`, and all exporter configs

### Requirement: register.js generation for Next.js

For Next.js projects only, the system SHALL generate a `.otel/register.js` file that sets `process.env.OTEL_*` variables and requires `@opentelemetry/auto-instrumentations-node/register`.

#### Scenario: register.js generated for Next.js

- **GIVEN** a project is identified as Next.js
- **WHEN** the `.otel/` directory is created
- **THEN** `.otel/register.js` is written
- **AND** it contains `process.env.OTEL_SERVICE_NAME = ...` for each OTEL\_\* var
- **AND** it contains `require('@opentelemetry/auto-instrumentations-node/register')`

#### Scenario: register.js NOT generated for regular projects

- **GIVEN** a project is NOT Next.js
- **WHEN** the `.otel/` directory is created
- **THEN** `.otel/register.js` is NOT written

### Requirement: Regular Node.js app launch

For non-Next.js projects, the system SHALL launch the app using `node --require @opentelemetry/auto-instrumentations-node/register <entrypoint>` with CWD set to `.otel/` so that `require()` resolves the OTel module from `.otel/node_modules/`.

#### Scenario: Regular app launched with auto-instrumentation

- **GIVEN** a regular Node.js project with entrypoint `server.js`
- **WHEN** `Execute()` launches the process
- **THEN** the command is `node --require @opentelemetry/auto-instrumentations-node/register ../server.js`
- **AND** CWD is set to the `.otel/` directory
- **AND** OTEL\_\* env vars are set on the process

#### Scenario: Process managed by ManagedProcess

- **GIVEN** the instrumented app has been launched
- **WHEN** the process is running
- **THEN** it is tracked via `StartManagedProcess()` with log file capture
- **AND** `PrintProcessSummary()` reports port detection or crash status

### Requirement: Next.js app launch

For Next.js projects, the system SHALL launch the app using `node --require ./.otel/register.js ./node_modules/next/dist/bin/next start` with CWD set to the project root.

#### Scenario: Next.js app launched with register.js

- **GIVEN** a Next.js project
- **WHEN** `Execute()` launches the process
- **THEN** the command is `node --require ./.otel/register.js ./node_modules/next/dist/bin/next start`
- **AND** CWD is set to the project root (not `.otel/`)
- **AND** `register.js` has already set the OTEL\_\* env vars programmatically

### Requirement: Dynatrace service verification

After launching the instrumented process, the system SHALL poll Dynatrace Smartscape until the service appears, using the existing `waitForServices()` infrastructure.

#### Scenario: Service appears in Dynatrace

- **GIVEN** the instrumented process is running and sending telemetry
- **WHEN** `waitForServices()` polls Smartscape
- **THEN** it detects the service and prints a confirmation with a link to the Dynatrace UI

#### Scenario: All processes crashed

- **GIVEN** all launched processes have crashed during the settle window
- **WHEN** `PrintProcessSummary()` returns no alive services
- **THEN** the "Waiting for traffic" prompt is NOT shown
- **AND** the user sees "No services are running — check the logs above for errors."

### Requirement: Preview and dry-run

`InstallOtelNode()` SHALL show a plan preview before executing. Under `--dry-run`, the preview is displayed but no changes are applied.

#### Scenario: Dry-run shows preview

- **GIVEN** `--dry-run` is set
- **WHEN** `InstallOtelNode()` runs
- **THEN** it prints the API URL, service name, packages, env vars, and returns without creating `.otel/` or launching processes

#### Scenario: Normal run with confirmation

- **GIVEN** `--dry-run` is NOT set and a project is detected
- **WHEN** the plan steps are shown
- **THEN** the user is prompted to confirm before execution proceeds

### Requirement: Stop running processes

Before installation, the system SHALL stop any running Node.js processes associated with the selected project.

#### Scenario: Running processes stopped

- **GIVEN** the selected project has running Node.js processes
- **WHEN** `Execute()` begins
- **THEN** those processes are stopped via `stopProcesses()` before `.otel/` creation
