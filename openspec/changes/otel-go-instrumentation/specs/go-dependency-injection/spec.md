## ADDED Requirements

### Requirement: Add OTel Go SDK dependencies
The system SHALL add OTel Go SDK packages to the selected project via `go get`.

#### Scenario: Successful dependency injection
- **WHEN** the user confirms adding OTel dependencies to a Go project
- **THEN** the system SHALL run `go get` for `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`, `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`, and `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp`

#### Scenario: Preview before execution
- **WHEN** the system is about to add dependencies
- **THEN** it SHALL show the `go get` commands that will run and prompt `Apply? [Y/n]`

#### Scenario: Dry-run mode
- **WHEN** `--dry-run` is set
- **THEN** the system SHALL print the `go get` commands without executing

#### Scenario: Dependencies already present
- **WHEN** OTel packages are already in `go.mod`
- **THEN** the system SHALL inform the user and offer to update to latest versions
