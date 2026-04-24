# Spec: Java Process Detection

## ADDED Requirements

### Requirement: Running Java process discovery as enrichment

The installer SHALL detect running Java processes and use them to enrich the project selection menu and to stop any running instance before relaunch. A running process is NOT required to proceed with instrumentation.

#### Scenario: Java processes matched to projects

- **GIVEN** one or more Java processes are running on the system
- **WHEN** the installer scans for running processes using `ps ax` and matches them to detected projects
- **THEN** each matched project in the selection menu SHALL display the associated PID(s) (e.g., `← PIDs: 1234`)
- **AND** the plan preview SHALL explicitly list the process(es) that will be stopped, including PID and description
- **AND** matched processes SHALL only be stopped after the user has confirmed the plan

#### Scenario: JPS enrichment available

- **GIVEN** `jps` is available in PATH (JDK installed)
- **WHEN** the installer detects Java processes
- **THEN** process entries SHALL be enriched with the main class or JAR name from `jps` output, stored in the `Description` field of `DetectedProcess`
- **AND** the stop-step summary SHALL display `Description` when non-empty, falling back to `Command`
- **AND** processes detected by `ps` but not present in `jps` output SHALL still be included

#### Scenario: JPS not available

- **WHEN** `jps` is not found in PATH (JRE-only installation)
- **THEN** the installer SHALL rely solely on `ps ax` for process detection without error or warning

#### Scenario: No Java processes running

- **WHEN** no running Java processes are detected
- **THEN** the project selection menu SHALL still be shown (with no PID annotations)
- **AND** the installer SHALL proceed normally using the project-based entrypoint detection path
- **AND** the installer SHALL NOT print "No running Java processes detected" as an error or terminal condition

### Requirement: Windows process detection

On Windows, the installer SHALL detect Java processes using OS-appropriate mechanisms.

#### Scenario: Windows process listing

- **GIVEN** the installer is running on Windows
- **WHEN** Java processes are scanned
- **THEN** the existing `detectProcesses` Windows implementation SHALL be used to find processes with `java` in the command line
- **AND** the PID annotation in the project menu SHALL work the same as on Unix/macOS

### Requirement: Debug logging for process detection

The process detection pipeline SHALL emit debug-level log lines so users running with `--debug` can trace what was found, what was enriched, and what was matched to a project.

#### Scenario: Raw process scan result

- **WHEN** `detectJavaProcesses` completes
- **THEN** a debug line SHALL be emitted: `"detected java processes" count=<n>`

#### Scenario: JPS enrichment applied

- **WHEN** `enrichProcessesWithJPS` successfully enriches a process entry
- **THEN** a debug line SHALL be emitted: `"jps enrichment" pid=<pid> description=<description>`

#### Scenario: JPS not available

- **WHEN** `jps` is not found in PATH
- **THEN** a debug line SHALL be emitted: `"jps not found, skipping enrichment"`

#### Scenario: Process matched to project

- **WHEN** a running process is matched to a detected project
- **THEN** a debug line SHALL be emitted: `"matched process to project" pid=<pid> project=<path>`

#### Scenario: No processes matched

- **WHEN** no running processes are matched to any detected project
- **THEN** a debug line SHALL be emitted: `"no running java processes matched to any project"`
