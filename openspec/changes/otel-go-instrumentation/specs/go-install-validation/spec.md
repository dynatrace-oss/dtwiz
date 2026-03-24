## ADDED Requirements

### Requirement: Pre-flight validation for Go installer
The `InstallOtelGo()` function SHALL validate prerequisites before proceeding.

#### Scenario: Go not in PATH
- **WHEN** `go` is not found in PATH
- **THEN** the installer SHALL exit with a clear error message indicating Go is required

#### Scenario: No go.mod in selected project
- **WHEN** the selected project directory does not contain a `go.mod` file
- **THEN** the installer SHALL exit with a clear error message indicating a Go module is required

#### Scenario: All prerequisites met
- **WHEN** `go` is in PATH and the project has a `go.mod`
- **THEN** the installer SHALL proceed with the installation flow
