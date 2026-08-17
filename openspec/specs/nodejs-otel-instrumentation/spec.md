# Node.js OTel Instrumentation

## Purpose

Define how `dtwiz install otel` sets up Node.js OTel auto-instrumentation including package installation, `.otel/` directory, and startup script.

## Requirements

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

For Next.js and Nuxt projects, the system SHALL generate framework-specific bootstrap scripts in `.otel/`. OTEL_\* env vars SHALL NOT be embedded in scripts; they are passed via `cmd.Env` at process launch time to avoid writing secrets to disk. Bootstrap scripts SHALL be written with `0600` permissions.

#### Scenario: next-otel-bootstrap.js generated for Next.js

- **GIVEN** a project is identified as Next.js
- **WHEN** the `.otel/` directory is created
- **THEN** `.otel/next-otel-bootstrap.js` is written (CommonJS) with `0600` permissions
- **AND** it requires `@opentelemetry/auto-instrumentations-node/register`
- **AND** it delegates to `next/dist/bin/next` (Next.js CLI)
- **AND** it does NOT embed OTEL\_\* env vars (they are passed via process environment at launch time)

#### Scenario: nuxt-otel-bootstrap.mjs generated for Nuxt

- **GIVEN** a project is identified as Nuxt
- **WHEN** the `.otel/` directory is created
- **THEN** `.otel/nuxt-otel-bootstrap.mjs` is written (ES Module) with `0600` permissions
- **AND** it uses `node:module.register()` to install ESM loader hooks (import-in-the-middle)
- **AND** it requires `@opentelemetry/auto-instrumentations-node/register` via `createRequire()`
- **AND** it does NOT delegate to the Nuxt CLI (the CLI spawns child processes that lose registration)

#### Scenario: No bootstrap script for regular projects

- **GIVEN** a project is NOT Next.js or Nuxt
- **WHEN** the `.otel/` directory is created
- **THEN** no bootstrap scripts are written

### Requirement: Entrypoint detection

The system SHALL use `detectNodeEntrypoints` to resolve the entrypoint for regular Node.js projects. Detection priority: `package.json` `"main"` field, `scripts.start` file reference, other scripts with `node <file>` patterns, or conventional filenames (`index`, `app`, `server` with `.js`/`.ts`/`.mjs`/`.cjs`/`.mts`/`.cts`). If none is found and the project is not Next.js or Nuxt, it is skipped.

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
- **THEN** the project is skipped with a message: "This project can't be auto-instrumented."
- **AND** a link to the Dynatrace manual instrumentation docs is shown: "See Instrument your JavaScript application on Node.js with OpenTelemetry to instrument it manually."
- **AND** the user is immediately prompted "Select another project? [Y/n]"
- **AND** if the user confirms, the project list is shown again for re-selection
- **AND** if the user declines, the flow exits without the "No Node.js projects detected" fallback message

This behavior applies in both the standalone `dtwiz install otel-node` flow (via `DetectNodePlan`) and the combined `dtwiz install otel` flow (via `InstallOtelCollectorWithProject`).

### Requirement: Auto-install project dependencies

`Execute()` SHALL install the project's own `node_modules/` as the **first** step, before any framework build. This ensures that `npm run build` (triggered for Next.js/Nuxt when build output is missing) has dependencies available. The installer calls `installNodeProjectDeps()`, which runs `npm ci` when `package-lock.json` is present, or `npm install` otherwise. When `node_modules/` already exists the call is a silent no-op.

#### Scenario: node_modules/ missing for regular, Next.js, or Nuxt app — auto-installed

- **GIVEN** a Node.js project (regular, Next.js, or Nuxt) is selected
- **AND** the project directory does not contain a `node_modules/` subdirectory
- **WHEN** `Execute()` prepares to launch
- **THEN** it prints "Installing project dependencies..."
- **AND** `installNodeProjectDeps()` runs `npm ci` (if `package-lock.json` exists) or `npm install`
- **AND** on success it prints "done." and execution continues normally

#### Scenario: node_modules/ missing and install fails

- **GIVEN** any Node.js project (regular, Next.js, or Nuxt) is selected
- **AND** the project directory does not contain a `node_modules/` subdirectory
- **AND** `npm install` / `npm ci` exits non-zero
- **WHEN** `Execute()` calls `installNodeProjectDeps()` as its first step
- **THEN** it prints "failed." with the npm error output
- **AND** it exits without running any framework build, creating `.otel/`, or launching any process

#### Scenario: node_modules/ already present — no-op

- **GIVEN** any Node.js project (regular, Next.js, or Nuxt) is selected
- **AND** the project directory already contains a `node_modules/` subdirectory
- **WHEN** `Execute()` calls `installNodeProjectDeps()`
- **THEN** the function returns immediately without running npm
- **AND** execution continues normally

### Requirement: Regular Node.js app launch

For non-Next.js projects, the system SHALL launch the app using `node --require @opentelemetry/auto-instrumentations-node/register <entrypoint>` with CWD set to `.otel/` so that `require()` resolves the OTel module from `.otel/node_modules/`. The entrypoint path SHALL use forward slashes (e.g. `../server.js`) to ensure compatibility on Windows.

#### Scenario: Regular app launched with auto-instrumentation

- **GIVEN** a regular Node.js project where `detectNodeEntrypoints` resolved entrypoint `server.js`
- **WHEN** `Execute()` launches the process
- **THEN** the command is `node --require @opentelemetry/auto-instrumentations-node/register ../server.js`
- **AND** the entrypoint path uses forward slashes regardless of the host OS
- **AND** CWD is set to the `.otel/` directory
- **AND** OTEL\_\* env vars are set on the process

#### Scenario: Process managed by ManagedProcess

- **GIVEN** the instrumented app has been launched
- **WHEN** the process is running
- **THEN** it is tracked via `StartManagedProcess()` with log file capture
- **AND** `PrintProcessSummary()` reports port detection or crash status

### Requirement: Auto-build Next.js project when `.next/` is missing

For Next.js projects, `next start` requires a production build in `.next/`. If `.next/` is absent when `Execute()` runs, the system SHALL automatically run `npm run build` via `runBuildScript()` before proceeding, using the same pattern as Nuxt. If the build script is absent or the build fails, the installer exits with a clear error. After a successful build, the presence of `.next/` is re-verified before continuing.

#### Scenario: .next/ missing — auto-build triggered

- **GIVEN** a Next.js project is selected
- **AND** `.next/` does not exist
- **AND** `package.json` contains a `"build"` script
- **WHEN** `Execute()` prepares to launch
- **THEN** it prints "Building Next.js project (npm run build)..."
- **AND** runs `npm run build` via `runBuildScript()`
- **AND** on success prints "done." and continues normally

#### Scenario: .next/ missing — auto-build fails (no build script)

- **GIVEN** a Next.js project is selected
- **AND** `.next/` does not exist
- **AND** `package.json` has no `"build"` script
- **WHEN** `Execute()` calls `runBuildScript()`
- **THEN** it prints "failed." with an error indicating to add a build script
- **AND** exits without creating `.otel/` or launching any process

#### Scenario: .next/ already exists — build skipped

- **GIVEN** a Next.js project is selected
- **AND** `.next/` already exists
- **WHEN** `Execute()` prepares to launch
- **THEN** `runBuildScript()` is NOT called and execution proceeds directly

For Next.js projects, the system SHALL launch the app using `node .otel/next-otel-bootstrap.js start` with CWD set to the project root.

#### Scenario: Next.js app launched via wrapper

- **GIVEN** a Next.js project and `.otel/next-otel-bootstrap.js` has been written
- **WHEN** `Execute()` launches the process
- **THEN** the command is `node .otel/next-otel-bootstrap.js start`
- **AND** CWD is set to the project root (not `.otel/`)
- **AND** OTEL\_\* env vars are set on the process
- **AND** the process is tracked with log file capture

### Requirement: Nuxt app launch

For Nuxt projects, the system SHALL launch the Nitro server directly using `node --import .otel/nuxt-otel-bootstrap.mjs .output/server/index.mjs` with CWD set to the project root. The Nuxt CLI is not used because it spawns child processes that lose OTel registration.

#### Scenario: Nuxt build output required — auto-build triggered

- **GIVEN** a Nuxt project is selected
- **AND** `.output/server/index.mjs` does not exist
- **AND** `package.json` contains a `"build"` script
- **WHEN** `Execute()` prepares to launch
- **THEN** it prints "Building Nuxt project (npm run build)..."
- **AND** runs `npm run build` via `buildNuxtProject()`
- **AND** on success prints "done." and continues normally

#### Scenario: Nuxt auto-build fails — no build script

- **GIVEN** a Nuxt project is selected
- **AND** `.output/server/index.mjs` does not exist
- **AND** `package.json` has no `"build"` script
- **WHEN** `Execute()` calls `buildNuxtProject()`
- **THEN** it prints "failed." with an error indicating to add a build script or run `npx nuxt build` manually
- **AND** exits without creating `.otel/` or launching any process

#### Scenario: Nuxt auto-build fails — npm run build exits non-zero

- **GIVEN** a Nuxt project is selected
- **AND** `.output/server/index.mjs` does not exist
- **AND** `package.json` contains a `"build"` script
- **AND** `npm run build` exits with a non-zero code
- **WHEN** `Execute()` calls `buildNuxtProject()`
- **THEN** it prints "failed." with the npm error output
- **AND** exits without creating `.otel/` or launching any process

#### Scenario: Nuxt build output already exists — build skipped

- **GIVEN** a Nuxt project is selected
- **AND** `.output/server/index.mjs` already exists
- **WHEN** `Execute()` prepares to launch
- **THEN** `buildNuxtProject()` is NOT called and execution proceeds directly to the launch step

#### Scenario: Nuxt app launched via ESM bootstrap

- **GIVEN** a Nuxt project with `.output/server/index.mjs` available
- **AND** `.otel/nuxt-otel-bootstrap.mjs` has been written
- **WHEN** `Execute()` launches the process
- **THEN** the command is `node --import .otel/nuxt-otel-bootstrap.mjs .output/server/index.mjs`
- **AND** CWD is set to the project root
- **AND** OTEL\_\* env vars are set on the process
- **AND** the ESM bootstrap uses `module.register()` to install import-in-the-middle hooks before any code loads
- **AND** the process is tracked with log file capture

### Requirement: Process health check

After launching the instrumented process, the system SHALL use `PrintProcessSummary()` to verify processes are alive and detect listening ports.

#### Scenario: Processes started successfully

- **GIVEN** the instrumented process is running and sending telemetry
- **WHEN** `PrintProcessSummary()` checks process status
- **THEN** it reports alive services with detected ports
- **AND** the user sees "Waiting for traffic — send requests to your services to generate traces and metrics."

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
