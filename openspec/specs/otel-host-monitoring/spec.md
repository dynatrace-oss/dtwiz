# OTel Host Monitoring Spec

## Purpose

Define how `dtwiz install otel` configures host monitoring via the Dynatrace OpenTelemetry Host Monitoring extension.

## Requirements

### Requirement: Host monitoring is configured during OTel Collector install

`install otel` SHALL configure the managed OTel Collector to collect host-level signals in addition to application signals, using the Dynatrace Host Monitoring reference configuration.

#### Scenario: Host metrics pipeline added to managed collector

- **GIVEN** a user runs `install otel`
- **WHEN** dtwiz generates the managed collector configuration
- **THEN** the configuration SHALL include tiered `hostmetrics` receivers (10s, 5m, and 1h collection intervals)
- **AND** it SHALL include a metrics pipeline that routes those receivers through the `filter`, `resource_detection`, `transform`, `filter/delete-metrics`, and `cumulative_to_delta` processors to the Dynatrace `otlp_http` exporter

#### Scenario: Existing application pipelines are preserved

- **GIVEN** the managed collector previously carried only application (`otlp`) pipelines
- **WHEN** host monitoring is configured
- **THEN** the application traces, metrics, and logs pipelines fed by the `otlp` receiver SHALL remain functional
- **AND** the application metrics pipeline SHALL be distinct from the host metrics pipeline so the two do not share receivers

#### Scenario: Single shared exporter

- **WHEN** the combined configuration is generated
- **THEN** host and application pipelines SHALL export to the same Dynatrace `/api/v2/otlp` endpoint using the same authorization header

### Requirement: Platform-aware host signal selection

The generated configuration SHALL include host metrics on all supported platforms. The `logs` pipeline SHALL apply `resource_detection` on all platforms when host monitoring is enabled, so all logs are correlated with the host in Dynatrace. On Linux, the `journald` receiver SHALL additionally be included in the `logs` pipeline to collect system journal logs; on macOS and Windows, `journald` is omitted.

#### Scenario: Linux collects metrics and journald logs

- **GIVEN** the target platform is Linux
- **WHEN** the configuration is generated
- **THEN** it SHALL include the `journald` receiver in the `logs` pipeline alongside `otlp`
- **AND** the `logs` pipeline SHALL apply `resource_detection` so all logs are associated with the host

#### Scenario: macOS and Windows collect metrics only, no journald

- **GIVEN** the target platform is macOS or Windows
- **WHEN** the configuration is generated
- **THEN** it SHALL NOT include the `journald` receiver
- **AND** the `logs` pipeline SHALL still apply `resource_detection` so application logs are associated with the host

### Requirement: Preview and confirmation before applying

The combined configuration SHALL be shown to the user before it is written, consistent with the existing collector install preview.

#### Scenario: Combined config previewed with masked secrets

- **WHEN** `install otel` reaches the preview step
- **THEN** a summary of the configuration SHALL be printed inline with the ingest token masked: the receiver endpoint lines at the top and the service pipelines block at the bottom, with a note indicating how many lines are hidden and directing the user to `--debug` for the full output
- **AND** the user SHALL be asked a single confirmation prompt defaulting to yes before any file is written

#### Scenario: Dry-run makes no changes

- **GIVEN** `install otel --dry-run`
- **WHEN** the command runs
- **THEN** the combined configuration SHALL be shown
- **AND** no collector binary, configuration file, or process SHALL be created or modified

### Requirement: Privilege and platform limitations are surfaced

When host monitoring may be incomplete due to insufficient privileges or platform-specific gaps, dtwiz SHALL inform the user rather than imply full coverage. The notice is platform-specific and elevation-aware: on Linux and Windows it is suppressed when the process is already running with sufficient privileges, since no action is needed; on macOS it is always shown because the gaps (`system.processes.created`, `process.disk.io`) are permanent regardless of privilege level.

#### Scenario: Unprivileged notice on Linux

- **GIVEN** the target platform is Linux
- **AND** the process is not running with root or equivalent privileges
- **WHEN** `install otel` configures host monitoring
- **THEN** the output SHALL note that full host metrics and logs require elevated privileges

#### Scenario: Unprivileged notice on Windows

- **GIVEN** the target platform is Windows
- **AND** the process is not running with Administrator privileges
- **WHEN** `install otel` configures host monitoring
- **THEN** the output SHALL note that some per-process metrics require Administrator or Debug privilege

#### Scenario: macOS platform limitation notice

- **GIVEN** the target platform is macOS
- **WHEN** `install otel` configures host monitoring
- **THEN** the output SHALL note that `system.processes.created` and `process.disk.io` are unavailable on macOS regardless of privilege level

