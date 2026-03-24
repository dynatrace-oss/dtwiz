## ADDED Requirements

### Requirement: Automatic Java agent JAR download
The system SHALL automatically download the latest OpenTelemetry Java agent JAR from the official GitHub releases to `~/opentelemetry/java/opentelemetry-javaagent.jar`.

#### Scenario: Successful download
- **WHEN** the user runs `dtwiz install otel-java` and network access is available
- **THEN** the system SHALL download the latest `opentelemetry-javaagent.jar` from `https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest` to `~/opentelemetry/java/`

#### Scenario: Agent JAR already exists
- **WHEN** `~/opentelemetry/java/opentelemetry-javaagent.jar` already exists
- **THEN** the system SHALL inform the user and prompt whether to re-download or use the existing JAR

#### Scenario: Download failure
- **WHEN** the download fails due to network error
- **THEN** the system SHALL print a clear error with the download URL so the user can download manually
