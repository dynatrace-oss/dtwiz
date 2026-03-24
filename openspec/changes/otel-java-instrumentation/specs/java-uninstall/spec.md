## ADDED Requirements

### Requirement: Uninstall OTel Java instrumentation
The system SHALL provide a `dtwiz uninstall otel-java` command that stops instrumented Java processes and removes the agent JAR.

#### Scenario: Instrumented processes found
- **WHEN** Java processes with `-javaagent:.*opentelemetry-javaagent.jar` in their command are running
- **THEN** the system SHALL list them, prompt for confirmation, stop them, and remove `~/opentelemetry/java/`

#### Scenario: No instrumented processes found
- **WHEN** no instrumented Java processes are running but the JAR directory exists
- **THEN** the system SHALL offer to remove the JAR directory

#### Scenario: Dry-run mode
- **WHEN** `--dry-run` is set
- **THEN** the system SHALL list processes and directories that would be affected without taking action

#### Scenario: Confirmation prompt
- **WHEN** cleanup actions are identified
- **THEN** the system SHALL show a preview and prompt `Apply? [Y/n]` before proceeding
