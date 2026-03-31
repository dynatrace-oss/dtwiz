# Node.js Runtime Detection

## ADDED Requirements

### Requirement: Node.js runtime detection

The system SHALL detect Node.js installations by looking up `node` on PATH and verifying the version via `node --version`.

#### Scenario: Node.js is available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `node` is found on PATH
- **THEN** the system proceeds with project scanning

#### Scenario: Node.js is not available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `node` is not found on PATH
- **THEN** the system silently skips Node.js detection

### Requirement: Node.js project scanning

The system SHALL scan the filesystem for Node.js project markers (`package.json`) starting from the current directory and common project locations, excluding `node_modules` directories.

#### Scenario: Node.js project detected

- **GIVEN** Node.js is available on the system
- **WHEN** a directory contains `package.json` and is not inside a `node_modules` directory
- **THEN** the system adds it to the list of detected Node.js projects with marker `package.json`

#### Scenario: No Node.js projects found

- **GIVEN** Node.js is available on the system
- **WHEN** no directories contain `package.json` outside of `node_modules`
- **THEN** the user is not prompted and Node.js instrumentation is skipped

### Requirement: Node.js process detection

The system SHALL detect running `node` processes and attempt to match them to discovered projects by working directory. Detection is best-effort — it may fail on processes owned by other users or on systems with restricted permissions.

#### Scenario: Running Node process matched to project

- **GIVEN** one or more Node.js projects have been detected on the filesystem
- **WHEN** a running `node` process has a CWD matching a detected project directory
- **THEN** the project listing shows the associated PIDs

### Requirement: Node.js entrypoint detection

The system SHALL infer entrypoints from `package.json` fields (`main`, `scripts.start`) or common filenames. Recognized file extensions include `.js`, `.mjs`, `.cjs`, `.ts`, `.mts`, and `.cts`. Convention fallbacks SHALL check both `.js` and `.ts` variants for each base name (`index`, `app`, `server`).

#### Scenario: Entrypoint found in package.json

- **GIVEN** the user selected a Node.js project
- **WHEN** `package.json` has a `main` field or `scripts.start` field
- **THEN** the system uses that as the entrypoint

#### Scenario: Entrypoint found by convention

- **GIVEN** the user selected a Node.js project
- **WHEN** `package.json` does not specify an entrypoint but `index.js`, `index.ts`, `app.js`, `app.ts`, `server.js`, or `server.ts` exists in the project root
- **THEN** the system uses the first matching file as the entrypoint

#### Scenario: No entrypoint detected

- **GIVEN** the user selected a Node.js project
- **WHEN** no entrypoint can be inferred from `package.json` or conventional filenames
- **THEN** the system skips the project with a message explaining what was looked for and instructs the user to add an entrypoint and re-run

### Requirement: Node.js instrumentation output

The system SHALL guide the user through adding OTel auto-instrumentation to their Node.js application. It SHALL display the npm packages to install, the required environment variables, and the instrumented run command using `--require`. The user is responsible for running these commands.

#### Scenario: Plan preview shows project and run command

- **GIVEN** the user selected a Node.js project
- **WHEN** the combined installation plan preview is shown
- **THEN** the Node.js section displays the project path, the npm packages to install, and the instrumented run command

#### Scenario: Post-install output guides SDK setup

- **GIVEN** the user confirmed the combined installation plan
- **WHEN** the Node.js instrumentation step executes
- **THEN** the output shows the `npm install` command, the environment variable export statements, and the run command with `--require @opentelemetry/auto-instrumentations-node/register`
