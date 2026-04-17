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

### Requirement: Framework bootstrap scripts generation

For Next.js and Nuxt projects, the system SHALL generate framework-specific bootstrap scripts in `.otel/`. These scripts set `process.env.OTEL_*` variables and require the auto-instrumentation register module before delegating to the framework.

#### Scenario: next-register.js generated for Next.js

- **GIVEN** a project is identified as Next.js
- **WHEN** the `.otel/` directory is created
- **THEN** `.otel/next-register.js` is written (CommonJS)
- **AND** it sets `process.env` variables for all OTEL\_\* config
- **AND** it requires `@opentelemetry/auto-instrumentations-node/register`
- **AND** it delegates to `next/dist/bin/next` (Next.js CLI)

#### Scenario: nuxt-otel-bootstrap.mjs generated for Nuxt

- **GIVEN** a project is identified as Nuxt
- **WHEN** the `.otel/` directory is created
- **THEN** `.otel/nuxt-otel-bootstrap.mjs` is written (ES Module)
- **AND** it uses `node:module.register()` to install ESM loader hooks (import-in-the-middle)
- **AND** it requires `@opentelemetry/auto-instrumentations-node/register` via `createRequire()`
- **AND** it does NOT delegate to the Nuxt CLI (the CLI spawns child processes that lose registration)

#### Scenario: No bootstrap script for regular projects

- **GIVEN** a project is NOT Next.js or Nuxt
- **WHEN** the `.otel/` directory is created
- **THEN** no bootstrap scripts are written

### Requirement: Entrypoint detection

The system SHALL use the existing `detectNodeEntrypoints` function to resolve the entrypoint for regular Node.js projects. The detection priority is: `package.json` `"main"` field → `scripts.start` file reference → other `scripts` entries with `node <file>` patterns → conventional filenames at project root (`index`, `app`, `server` with `.js`, `.ts`, `.mjs`, `.cjs`, `.mts`, `.cts` extensions). If no entrypoint is found and the project is not a known framework (Next.js, Nuxt), the project is skipped.

#### Scenario: Entrypoint detected via package.json main

- **GIVEN** a project with `"main": "server.js"` in `package.json` and `server.js` exists on disk
- **WHEN** `detectNodeEntrypoints` runs
- **THEN** it returns `["server.js"]`

#### Scenario: Entrypoint detected via scripts.start

- **GIVEN** a project with `"scripts": {"start": "node app.js"}` and `app.js` exists on disk
- **AND** no `"main"` field is set
- **WHEN** `detectNodeEntrypoints` runs
- **THEN** it returns `["app.js"]`

#### Scenario: Entrypoint detected via conventional filename

- **GIVEN** a project with no `"main"` field and no scripts referencing source files
- **AND** `index.js` exists in the project root
- **WHEN** `detectNodeEntrypoints` runs
- **THEN** it returns `["index.js"]`

#### Scenario: No entrypoint found and not a framework

- **GIVEN** a project with no `"main"`, no scripts referencing source files, and no conventional files
- **AND** the project is not Next.js or Nuxt
- **WHEN** `buildNodeInstrumentationPlan` runs
- **THEN** the project is skipped with a diagnostic message

### Requirement: Regular Node.js app launch

For non-Next.js projects, the system SHALL launch the app using `node --require @opentelemetry/auto-instrumentations-node/register <entrypoint>` with CWD set to `.otel/` so that `require()` resolves the OTel module from `.otel/node_modules/`.

#### Scenario: Regular app launched with auto-instrumentation

- **GIVEN** a regular Node.js project where `detectNodeEntrypoints` resolved entrypoint `server.js`
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

For Next.js projects, the system SHALL launch the app using `node .otel/next-register.js start` with CWD set to the project root.

#### Scenario: Next.js app launched via wrapper

- **GIVEN** a Next.js project and `.otel/next-register.js` has been written
- **WHEN** `Execute()` launches the process
- **THEN** the command is `node .otel/next-register.js start`
- **AND** CWD is set to the project root (not `.otel/`)
- **AND** OTEL\_\* env vars are set on the process
- **AND** the process is tracked with log file capture

### Requirement: Nuxt app launch

For Nuxt projects, the system SHALL launch the Nitro server directly using `node --import .otel/nuxt-otel-bootstrap.mjs .output/server/index.mjs` with CWD set to the project root. The Nuxt CLI is not used because it spawns child processes that lose OTel registration.

#### Scenario: Nuxt build output required

- **GIVEN** a Nuxt project is selected
- **WHEN** `Execute()` prepares to launch
- **THEN** it checks for `.output/server/index.mjs` (built Nitro output)
- **AND** if not found, it prints an error message: "Nuxt build output not found. Run 'npx nuxt build' first, then re-run dtwiz."
- **AND** it exits without launching

#### Scenario: Nuxt app launched via ESM bootstrap

- **GIVEN** a Nuxt project with `.output/server/index.mjs` available
- **AND** `.otel/nuxt-otel-bootstrap.mjs` has been written
- **WHEN** `Execute()` launches the process
- **THEN** the command is `node --import .otel/nuxt-otel-bootstrap.mjs .output/server/index.mjs`
- **AND** CWD is set to the project root
- **AND** OTEL\_\* env vars are set on the process
- **AND** the ESM bootstrap uses `module.register()` to install import-in-the-middle hooks before any code loads
- **AND** the process is tracked with log file capture

### Requirement: Dynatrace service verification

After launching the instrumented process, the system SHALL poll Dynatrace Smartscape until the service appears, using the existing `waitForServices()` infrastructure.

#### Scenario: Service appears in Dynatrace

- **GIVEN** the instrumented process is running and sending telemetry
- **WHEN** `PrintProcessSummary()` detects ports and `waitForServices()` polls Smartscape
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
