# Go Runtime Detection

## ADDED Requirements

### Requirement: Go runtime detection

The system SHALL detect Go installations by looking up `go` on PATH.

#### Scenario: Go is available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `go` is found on PATH
- **THEN** the system reports the Go path and version and proceeds with project scanning

#### Scenario: Go is not available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `go` is not found on PATH
- **THEN** the system silently skips Go detection

### Requirement: Go project scanning

The system SHALL scan the filesystem for Go project markers (`go.mod`) starting from the current directory and common project locations.

#### Scenario: Go project detected

- **GIVEN** Go is available on the system
- **WHEN** a directory contains `go.mod`
- **THEN** the system adds it to the list of detected Go projects with marker `go.mod` and extracts the module name from the `module` directive

#### Scenario: No Go projects found

- **GIVEN** Go is available on the system
- **WHEN** no directories contain `go.mod`
- **THEN** the user is not prompted and Go instrumentation is skipped

### Requirement: Go instrumentation is SDK-based guidance

The system SHALL communicate that Go requires compile-time SDK integration and cannot be auto-instrumented at runtime like Python or Java. The plan SHALL provide the necessary OTel environment variables and instructions for adding the OpenTelemetry Go SDK to the project.

#### Scenario: Plan preview shows SDK guidance

- **GIVEN** the user selected a Go project
- **WHEN** the combined installation plan preview is shown
- **THEN** the Go section displays the project path, required `go get` packages, and is labeled "SDK integration (manual)"

#### Scenario: Post-install output guides SDK integration

- **GIVEN** the user confirmed the combined installation plan
- **WHEN** the Go instrumentation step executes
- **THEN** the output shows the `go get` commands for the OTel Go SDK, the environment variable export statements, and a clear note that the user must add SDK initialization code to their application
