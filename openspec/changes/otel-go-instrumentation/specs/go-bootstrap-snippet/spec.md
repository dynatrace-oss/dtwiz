## ADDED Requirements

### Requirement: Generate OTel Go bootstrap snippet
The system SHALL generate a copy-paste-ready Go code snippet that initializes the OTel SDK with OTLP HTTP exporters configured for Dynatrace.

#### Scenario: Snippet generation
- **WHEN** dependencies have been added to the project
- **THEN** the system SHALL print a Go code snippet that initializes TracerProvider, MeterProvider, and LoggerProvider with OTLP HTTP exporters, reads configuration from OTEL_* env vars, and includes a `defer shutdown()` pattern

#### Scenario: OTEL environment variables
- **WHEN** the snippet is generated
- **THEN** the system SHALL also print the required OTEL_* environment variables (`OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`) with correct values for the user's Dynatrace environment

#### Scenario: Placement instructions
- **WHEN** the snippet is printed
- **THEN** the system SHALL include clear instructions on where to add the snippet (in `main()`, before application logic starts) and how to import the required packages
