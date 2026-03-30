# Go Runtime Detection

## ADDED Requirements

### Requirement: Go runtime detection

The system SHALL detect Go installations by looking up `go` on PATH and verifying the version via `go version`.

#### Scenario: Go is available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `go` is found on PATH and `go version` succeeds
- **THEN** the system reports the Go path and version and proceeds with project scanning

#### Scenario: Go is not available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `go` is not found on PATH
- **THEN** the system silently skips Go detection and returns nil

### Requirement: Go project scanning

The system SHALL scan the filesystem for Go project markers (`go.mod`) starting from the current directory and common project locations. SHALL use the shared `scanProjectDirs()` utility in `pkg/installer/otel_common.go` — NOT duplicate the scanning logic.

#### Scenario: Go project detected

- **GIVEN** Go is available on the system
- **WHEN** a directory contains `go.mod`
- **THEN** the system adds it to the list of detected Go projects with marker `go.mod` and extracts the module name from the `module` directive

#### Scenario: No Go projects found

- **GIVEN** Go is available on the system
- **WHEN** no directories contain `go.mod`
- **THEN** `DetectGoPlan` returns nil without prompting the user

### Requirement: Go instrumentation is SDK-based guidance

The system SHALL communicate that Go requires compile-time SDK integration and cannot be auto-instrumented at runtime like Python or Java. The plan SHALL provide the necessary OTel environment variables and instructions for adding the OpenTelemetry Go SDK to the project.

#### Scenario: PrintPlanSteps shows SDK guidance

- **GIVEN** a `GoInstrumentationPlan` was created for a selected project
- **WHEN** `PrintPlanSteps()` is called on a `GoInstrumentationPlan`
- **THEN** it prints the project path, required `go get` packages, and environment variables — and labels the step as "SDK integration (manual)"

#### Scenario: Execute prints setup instructions

- **GIVEN** the user confirmed the combined installation plan
- **WHEN** `Execute()` is called on a `GoInstrumentationPlan`
- **THEN** it prints the `go get` commands for the OTel Go SDK, generates the environment variable export statements, and clearly states the user must add SDK initialization code to their application

### Requirement: GoInstrumentationPlan struct

The system SHALL define a `GoInstrumentationPlan` struct with fields for the selected project (a `GoProject` containing the module name) and OTel environment variables. It SHALL implement `Runtime()`, `PrintPlanSteps()`, and `Execute()` methods satisfying the `InstrumentationPlan` interface. Follows the pattern established by `PythonInstrumentationPlan` in `pkg/installer/otel_python.go`. OTel environment variables SHALL be generated via the shared `generateBaseOtelEnvVars()` in `pkg/installer/otel_common.go` to ensure consistent URL-encoded header values across all runtimes.

#### Scenario: Struct fields populated

- **GIVEN** the user selected a Go project and detection completed
- **WHEN** `DetectGoPlan` returns a non-nil plan
- **THEN** the plan contains the project path, module name from `go.mod`, and pre-generated OTel environment variables
