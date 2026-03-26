# Node.js Runtime Detection

## ADDED Requirements

### Requirement: Node.js runtime detection

The system SHALL detect Node.js installations by looking up `node` on PATH and verifying the version via `node --version`.

#### Scenario: Node.js is available

- **GIVEN** the user selected Node.js from the runtime selection menu
- **WHEN** `node` is found on PATH and `node --version` succeeds
- **THEN** the system reports the Node.js path and version and proceeds with project scanning

#### Scenario: Node.js is not available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `node` is not found on PATH
- **THEN** the system silently skips Node.js detection and returns nil

### Requirement: Node.js project scanning

The system SHALL scan the filesystem for Node.js project markers (`package.json`) starting from the current directory and common project locations, excluding `node_modules` directories. Follows the same scanning pattern established by `DetectPythonPlan` in `pkg/installer/otel_python.go`.

#### Scenario: Node.js project detected

- **GIVEN** Node.js is available on the system
- **WHEN** a directory contains `package.json` and is not inside a `node_modules` directory
- **THEN** the system adds it to the list of detected Node.js projects with marker `package.json`

#### Scenario: No Node.js projects found

- **GIVEN** Node.js is available on the system
- **WHEN** no directories contain `package.json` outside of `node_modules`
- **THEN** `DetectNodePlan` returns nil without prompting the user

### Requirement: Node.js process detection

The system SHALL detect running `node` processes and attempt to match them to discovered projects by working directory.

#### Scenario: Running Node process matched to project

- **GIVEN** one or more Node.js projects have been detected on the filesystem
- **WHEN** a running `node` process has a CWD matching a detected project directory
- **THEN** the project listing shows the associated PIDs

### Requirement: Node.js project selection prompt

The system SHALL present discovered Node.js projects and prompt the user to select one or skip.

#### Scenario: User selects a project

- **GIVEN** one or more Node.js projects are listed
- **WHEN** the user enters a valid project number
- **THEN** the system creates a `NodeInstrumentationPlan` for that project

#### Scenario: User skips

- **GIVEN** one or more Node.js projects are listed
- **WHEN** the user presses Enter without selecting
- **THEN** `DetectNodePlan` returns nil

### Requirement: Node.js entrypoint detection

The system SHALL infer entrypoints from `package.json` fields (`main`, `scripts.start`) or common filenames (`index.js`, `app.js`, `server.js`).

#### Scenario: Entrypoint found in package.json

- **GIVEN** the user selected a Node.js project
- **WHEN** `package.json` has a `main` field or `scripts.start` field
- **THEN** the system uses that as the entrypoint

#### Scenario: Entrypoint found by convention

- **GIVEN** the user selected a Node.js project
- **WHEN** `package.json` does not specify an entrypoint but `index.js`, `app.js`, or `server.js` exists in the project root
- **THEN** the system uses the first matching file as the entrypoint

#### Scenario: No entrypoint detected

- **GIVEN** the user selected a Node.js project
- **WHEN** no entrypoint can be inferred from `package.json` or conventional filenames
- **THEN** the system prompts the user to enter the entrypoint file manually

### Requirement: NodeInstrumentationPlan struct

The system SHALL define a `NodeInstrumentationPlan` struct with fields for the selected project, entrypoints, OTel environment variables, `EnvURL`, and `PlatformToken`. It SHALL implement `PrintPlanSteps()` and `Execute()` methods. Follows the pattern established by `PythonInstrumentationPlan` in `pkg/installer/otel_python.go`.

#### Scenario: PrintPlanSteps displays plan

- **GIVEN** a `NodeInstrumentationPlan` was created for a selected project
- **WHEN** `PrintPlanSteps()` is called
- **THEN** it prints the project path, npm packages to install, and the instrumented run command

#### Scenario: Execute performs instrumentation

- **GIVEN** the user confirmed the combined installation plan
- **WHEN** `Execute()` is called
- **THEN** it installs `@opentelemetry/auto-instrumentations-node` and related packages via npm, configures environment variables, and launches the entrypoint with `--require @opentelemetry/auto-instrumentations-node/register`
