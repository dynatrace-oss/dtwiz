## ADDED Requirements

### Requirement: Uninstall OTel Node.js instrumentation
The system SHALL provide a `dtwiz uninstall otel-node` command that stops instrumented Node processes and removes OTel packages.

#### Scenario: Instrumented processes found
- **WHEN** Node processes with `@opentelemetry/auto-instrumentations-node` in their command are running
- **THEN** the system SHALL list them, prompt for confirmation, stop them, and run the package manager uninstall for OTel packages

#### Scenario: No instrumented processes found
- **WHEN** no instrumented Node processes are running
- **THEN** the system SHALL inform the user

#### Scenario: Dry-run mode
- **WHEN** `--dry-run` is set
- **THEN** the system SHALL list processes and packages that would be affected without taking action

#### Scenario: Confirmation prompt
- **WHEN** cleanup actions are identified
- **THEN** the system SHALL show a preview and prompt `Apply? [Y/n]` before proceeding
