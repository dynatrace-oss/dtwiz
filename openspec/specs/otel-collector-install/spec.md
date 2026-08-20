# Spec: Route App Telemetry Through Local OTel Collector on Install

## Overview

When `dtwiz install otel` (bundled flow) or any standalone `install otel-java / otel-node / otel-python`
command instruments an application, the instrumented app SHALL send its telemetry to the locally running
OTel Collector rather than directly to the Dynatrace API endpoint. The collector is the single egress
point that holds Dynatrace credentials and handles forwarding.

---

## Requirements

### Requirement: Instrumented apps target the local OTel Collector

When `dtwiz` sets up auto-instrumentation for any supported runtime, the generated environment for the
instrumented process SHALL set `OTEL_EXPORTER_OTLP_ENDPOINT` to the local collector's HTTP OTLP endpoint
(`http://127.0.0.1:<port>`). No Dynatrace API URL SHALL appear in the app's OTLP endpoint variable.

#### Scenario: Java app instrumented via `install otel` (bundled)

- **GIVEN** `dtwiz install otel` is run and a Java project is selected
- **WHEN** the instrumented process is launched
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to `http://127.0.0.1:<collectorHTTPPort>`

#### Scenario: Node.js app instrumented via `install otel` (bundled)

- **GIVEN** `dtwiz install otel` is run and a Node.js project is selected
- **WHEN** the instrumented process is launched
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to `http://127.0.0.1:<collectorHTTPPort>`

#### Scenario: Python app instrumented via `install otel` (bundled)

- **GIVEN** `dtwiz install otel` is run and a Python project is selected
- **WHEN** the instrumented process is launched
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to `http://127.0.0.1:<collectorHTTPPort>`

#### Scenario: Java app instrumented via `install otel-java` (standalone)

- **GIVEN** `dtwiz install otel-java` is run and a Java project is selected
- **WHEN** the instrumented process is launched
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to `http://127.0.0.1:<collectorHTTPPort>`

#### Scenario: Node.js app instrumented via `install otel-node` (standalone)

- **GIVEN** `dtwiz install otel-node` is run and a Node.js project is selected
- **WHEN** the instrumented process is launched
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to `http://127.0.0.1:<collectorHTTPPort>`

#### Scenario: Python app instrumented via `install otel-python` (standalone)

- **GIVEN** `dtwiz install otel-python` is run and a Python project is selected
- **WHEN** the instrumented process is launched
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to `http://127.0.0.1:<collectorHTTPPort>`

---

### Requirement: No Dynatrace authorization token in app process environment

When `dtwiz` sets up auto-instrumentation, the generated environment for the instrumented process
SHALL NOT include `OTEL_EXPORTER_OTLP_HEADERS` containing a Dynatrace token. The collector is
responsible for authenticating to Dynatrace; the app process has no need for that credential.

#### Scenario: Instrumented app has no OTLP authorization header

- **GIVEN** `dtwiz install otel` (or any standalone variant) instruments an application
- **WHEN** the instrumented process is launched
- **THEN** `OTEL_EXPORTER_OTLP_HEADERS` is not present in the process environment
- **THEN** no Dynatrace API token appears in any OTLP-related environment variable

---

### Requirement: The OTLP HTTP port used by the app matches the port the collector is actually configured on

The port in `OTEL_EXPORTER_OTLP_ENDPOINT` SHALL match the OTLP HTTP receiver port the collector is
configured to use. The port is resolved from the most accurate source available at the time of
instrumentation:

- **Bundled flow** (`install otel`): the port determined during collector plan preparation is used directly.
- **Standalone flows** (`install otel-java/node/python`): the port is read from the installed collector
  config file. If the config cannot be read or the port cannot be parsed, port 4318 is used as a fallback.

#### Scenario: Bundled flow — collector configured on a non-default port

- **GIVEN** port 4318 is occupied at install time, so the collector is configured to use port 4320
- **WHEN** `dtwiz install otel` instruments an application
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to `http://127.0.0.1:4320`

#### Scenario: Standalone flow — collector installed on a non-default port

- **GIVEN** a collector is installed with OTLP HTTP on port 4320 (as recorded in its config file)
- **WHEN** `dtwiz install otel-java` instruments an application
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to `http://127.0.0.1:4320`

#### Scenario: Standalone flow — no collector config found, fallback to 4318

- **GIVEN** no dtwiz-managed collector config file exists on disk
- **WHEN** `dtwiz install otel-java` instruments an application
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to `http://127.0.0.1:4318`

#### Scenario: Standalone flow — no collector listening on the resolved port

- **GIVEN** the resolved collector port (from config or fallback) has nothing listening on it
- **WHEN** `dtwiz install otel-java` (or `otel-node` / `otel-python`) completes instrumentation
- **THEN** the install succeeds (env vars are written, startup script is updated)
- **THEN** a clear warning is printed: the collector is not reachable on that port and telemetry will not reach Dynatrace until a collector is started
- **THEN** the process exits with a non-error exit code

---

### Requirement: Go instrumentation guidance uses the local collector endpoint

When `dtwiz` prints Go SDK setup instructions (environment variables the developer should set),
the displayed `OTEL_EXPORTER_OTLP_ENDPOINT` SHALL point to the local collector, not to Dynatrace.

#### Scenario: Go project guidance shows local collector endpoint

- **GIVEN** `dtwiz install otel` selects a Go project
- **WHEN** the environment variable guidance is printed
- **THEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is shown as `http://127.0.0.1:<collectorHTTPPort>`
- **THEN** no Dynatrace token is shown in `OTEL_EXPORTER_OTLP_HEADERS`

---

### Requirement: The OTel Collector config is unchanged by this feature

The collector config continues to hold the Dynatrace OTLP endpoint and authorization token.
No changes are made to how the collector forwards data to Dynatrace.

#### Scenario: Collector config still targets Dynatrace

- **GIVEN** `dtwiz install otel` runs and instruments an application
- **WHEN** the collector config is written
- **THEN** the collector's `otlp_http` exporter endpoint points to the Dynatrace API URL
- **THEN** the collector's `headers.Authorization` contains the Dynatrace token
