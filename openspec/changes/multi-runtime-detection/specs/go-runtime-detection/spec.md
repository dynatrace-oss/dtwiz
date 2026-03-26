## ADDED Requirements

### Requirement: Go runtime detection
The system SHALL detect Go installations by looking up `go` on PATH and verifying the version via `go version`.

#### Scenario: Go is available
- **WHEN** `go` is found on PATH and `go version` succeeds
- **THEN** the system reports the Go path and version and proceeds with project scanning

#### Scenario: Go is not available
- **WHEN** `go` is not found on PATH
- **THEN** the system silently skips Go detection and returns nil

### Requirement: Go project scanning
The system SHALL scan the filesystem for Go project markers (`go.mod`) starting from the current directory and common project locations.

#### Scenario: Go project detected
- **WHEN** a directory contains `go.mod`
- **THEN** the system adds it to the list of detected Go projects with marker `go.mod` and extracts the module name from the `module` directive

#### Scenario: No Go projects found
- **WHEN** no directories contain `go.mod`
- **THEN** `DetectGoPlan` returns nil without prompting the user

### Requirement: Go project selection prompt
The system SHALL present discovered Go projects and prompt the user to select one or skip.

#### Scenario: User selects a project
- **WHEN** the user enters a valid project number
- **THEN** the system creates a `GoInstrumentationPlan` for that project

#### Scenario: User skips
- **WHEN** the user presses Enter without selecting
- **THEN** `DetectGoPlan` returns nil

### Requirement: Go instrumentation is SDK-based guidance
The system SHALL communicate that Go requires compile-time SDK integration and cannot be auto-instrumented at runtime like Python or Java. The plan SHALL provide the necessary OTel environment variables and instructions for adding the OpenTelemetry Go SDK to the project.

#### Scenario: PrintPlanSteps shows SDK guidance
- **WHEN** `PrintPlanSteps()` is called on a `GoInstrumentationPlan`
- **THEN** it prints the project path, required `go get` packages, and environment variables — and labels the step as "SDK integration (manual)"

#### Scenario: Execute prints setup instructions
- **WHEN** `Execute()` is called on a `GoInstrumentationPlan`
- **THEN** it prints the `go get` commands for the OTel Go SDK, generates the environment variable export statements, and clearly states the user must add SDK initialization code to their application

### Requirement: GoInstrumentationPlan struct
The system SHALL define a `GoInstrumentationPlan` struct with fields for the selected project, module name, OTel environment variables, `EnvURL`, and `PlatformToken`. It SHALL implement `PrintPlanSteps()` and `Execute()` methods.

#### Scenario: Struct fields populated
- **WHEN** `DetectGoPlan` returns a non-nil plan
- **THEN** the plan contains the project path, module name from `go.mod`, and pre-generated OTel environment variables
