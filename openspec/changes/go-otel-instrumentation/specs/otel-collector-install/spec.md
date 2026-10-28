# Spec Delta

## MODIFIED Requirements

### Requirement: Go instrumentation guidance uses the local collector endpoint

Go services launched by dtwiz, and any Go setup instructions that dtwiz prints (environment variables the developer should set), SHALL use the local collector as the OTLP endpoint, not Dynatrace.

#### Scenario: Go project guidance shows local collector endpoint

- **GIVEN** `dtwiz install otel` selects a Go project that falls back to manual steps
- **WHEN** the environment variable guidance is printed
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is shown as `http://127.0.0.1:<collectorHTTPPort>`
- **THEN** no Dynatrace token is shown in `OTEL_EXPORTER_OTLP_HEADERS`

#### Scenario: Launched Go service uses the local collector

- **GIVEN** `dtwiz install otel` builds and launches a Go service
- **WHEN** the service starts
- **THEN** its `OTEL_EXPORTER_OTLP_ENDPOINT` is `http://127.0.0.1:<collectorHTTPPort>`
- **THEN** no Dynatrace token is present in its environment
