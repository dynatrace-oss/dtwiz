# Node.js OTel Uninstall

## MODIFIED Requirements

### Requirement: UninstallOtelCollector includes Node.js cleanup

The existing `UninstallOtelCollector()` function SHALL be extended to also detect and remove Node.js OTel instrumentation artifacts (`.otel/` directories and instrumented Node.js processes), in addition to the existing OTel Collector cleanup.

#### Scenario: Only collector artifacts exist

- **GIVEN** running OTel Collector processes and install directories exist
- **AND** no `.otel/` directories or instrumented Node.js processes exist
- **WHEN** `dtwiz uninstall otel` runs
- **THEN** the behavior is identical to the current implementation (no change)

#### Scenario: Only Node.js artifacts exist

- **GIVEN** no OTel Collector processes or directories exist
- **AND** `.otel/` directories with OTel packages exist
- **WHEN** `dtwiz uninstall otel` runs
- **THEN** the preview shows the Node.js instrumentation section with `.otel/` dirs to remove
- **AND** on confirmation, the `.otel/` directories are deleted

#### Scenario: Both collector and Node.js artifacts exist

- **GIVEN** both OTel Collector processes/dirs and Node.js `.otel/` dirs exist
- **WHEN** `dtwiz uninstall otel` runs
- **THEN** the preview shows both sections
- **AND** on confirmation, both are cleaned up

#### Scenario: Nothing to remove

- **GIVEN** no collector processes, no collector dirs, no `.otel/` dirs, no instrumented node processes
- **WHEN** `dtwiz uninstall otel` runs
- **THEN** it prints "Nothing to remove" and exits

## ADDED Requirements

### Requirement: Detect .otel/ directories

The system SHALL scan CWD and parent directories for `.otel/` directories that contain a `package.json` with `@opentelemetry` in its content.

#### Scenario: .otel/ directory found

- **GIVEN** the current directory contains a subfolder `.otel/package.json` with `@opentelemetry/auto-instrumentations-node` as a dependency
- **WHEN** `findNodeOtelDirs()` scans
- **THEN** the `.otel/` path is included in the results

#### Scenario: .otel/ directory without OTel packages ignored

- **GIVEN** a `.otel/` directory exists but its `package.json` does not contain `@opentelemetry`
- **WHEN** `findNodeOtelDirs()` scans
- **THEN** the directory is NOT included (it's not a dtwiz-created OTel dir)

#### Scenario: No .otel/ directories found

- **GIVEN** no `.otel/` directories exist in the scan area
- **WHEN** `findNodeOtelDirs()` scans
- **THEN** an empty list is returned

### Requirement: Detect instrumented Node.js processes

The system SHALL detect running `node` processes whose command line includes `@opentelemetry/auto-instrumentations-node/register`, `.otel/next-register.js`, or `.otel/nuxt-register.js`.

#### Scenario: Instrumented node process found

- **GIVEN** a `node` process is running with `--require @opentelemetry/auto-instrumentations-node/register` in its command
- **WHEN** `findInstrumentedNodeProcesses()` scans
- **THEN** the process PID is included in the results

#### Scenario: Framework wrapper process found

- **GIVEN** a `node` process is running with `otel/next-register.js` or `otel/nuxt-register.js` in its command
- **WHEN** `findInstrumentedNodeProcesses()` scans
- **THEN** the process PID is included in the results

#### Scenario: Regular node process not matched

- **GIVEN** a `node` process is running without OTel instrumentation flags
- **WHEN** `findInstrumentedNodeProcesses()` scans
- **THEN** it is NOT included

### Requirement: Preview and dry-run for Node.js cleanup

The Node.js cleanup section SHALL follow the same preview/dry-run/confirm pattern as the existing collector cleanup.

#### Scenario: Dry-run shows Node.js cleanup preview

- **GIVEN** `--dry-run` is set and `.otel/` directories exist
- **WHEN** `dtwiz uninstall otel --dry-run` runs
- **THEN** the preview shows `.otel/` dirs to remove and instrumented processes to kill
- **AND** prints "[dry-run] No changes made" without modifying anything

#### Scenario: Cleanup confirmed

- **GIVEN** `--dry-run` is NOT set
- **WHEN** the user confirms the uninstall
- **THEN** instrumented node processes are killed and `.otel/` directories are deleted
