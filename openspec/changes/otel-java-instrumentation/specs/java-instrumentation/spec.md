## ADDED Requirements

### Requirement: Restart Java process with instrumentation
The system SHALL stop the selected Java process and restart it with the `-javaagent` flag and OTEL_* environment variables configured for Dynatrace export.

#### Scenario: Successful instrumentation
- **WHEN** the user confirms instrumentation of a selected Java process
- **THEN** the system SHALL stop the process, restart it with `-javaagent:~/opentelemetry/java/opentelemetry-javaagent.jar` and OTEL_* env vars, and verify traces appear in Dynatrace

#### Scenario: Preview before execution
- **WHEN** the system is about to restart a Java process
- **THEN** it SHALL show the full command that will be executed (including `-javaagent` flag and env vars) and prompt `Apply? [Y/n]`

#### Scenario: Dry-run mode
- **WHEN** `--dry-run` is set
- **THEN** the system SHALL print the instrumented command and env vars without executing

#### Scenario: OTEL environment variables
- **WHEN** configuring the instrumented process
- **THEN** the system SHALL set `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`, `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`, and `OTEL_LOGS_EXPORTER`
