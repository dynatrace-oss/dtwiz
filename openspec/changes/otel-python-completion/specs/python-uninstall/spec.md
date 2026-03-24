## ADDED Requirements

### Requirement: Uninstall OTel Python instrumentation
The system SHALL provide a `dtwiz uninstall otel-python` command that stops instrumented Python processes and removes OTel Python packages from the project's virtualenv.

#### Scenario: Instrumented processes found and stopped
- **WHEN** the user runs `dtwiz uninstall otel-python` and Python processes launched via `opentelemetry-instrument` are running
- **THEN** the system SHALL list the processes, prompt for confirmation, stop them, and uninstall `opentelemetry-distro` and `opentelemetry-exporter-otlp` from the process's virtualenv

#### Scenario: No instrumented processes found
- **WHEN** the user runs `dtwiz uninstall otel-python` and no instrumented Python processes are running
- **THEN** the system SHALL inform the user that no instrumented processes were found

#### Scenario: Dry-run mode
- **WHEN** `--dry-run` is set
- **THEN** the system SHALL list the processes and packages that would be stopped/removed without taking action

#### Scenario: Confirmation prompt
- **WHEN** instrumented processes are found
- **THEN** the system SHALL show a preview of processes to stop and packages to remove, and prompt `Apply? [Y/n]` before proceeding
