## ADDED Requirements

### Requirement: Install and launch Node.js with OTel instrumentation
The system SHALL install OTel Node packages and launch the application with `--require @opentelemetry/auto-instrumentations-node/register` and OTEL_* environment variables.

#### Scenario: Successful instrumentation
- **WHEN** the user confirms instrumentation of a selected Node.js project
- **THEN** the system SHALL install OTel packages using the project's package manager, stop any running process for that project, launch with `node --require @opentelemetry/auto-instrumentations-node/register <entrypoint>`, and verify traces appear in Dynatrace

#### Scenario: Package manager detection
- **WHEN** installing OTel packages
- **THEN** the system SHALL detect the package manager from lockfile (`package-lock.json` → npm, `yarn.lock` → yarn, `pnpm-lock.yaml` → pnpm, fallback → npm)

#### Scenario: Preview before execution
- **WHEN** the system is about to instrument and launch
- **THEN** it SHALL show the install command, launch command, and OTEL_* env vars, and prompt `Apply? [Y/n]`

#### Scenario: Dry-run mode
- **WHEN** `--dry-run` is set
- **THEN** the system SHALL print the planned commands without executing

#### Scenario: OTEL environment variables
- **WHEN** configuring the instrumented process
- **THEN** the system SHALL set `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`, `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`, `OTEL_LOGS_EXPORTER`, and `OTEL_NODE_RESOURCE_DETECTORS=all`
