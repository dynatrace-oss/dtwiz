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

The generated configuration SHALL include host metrics on all supported platforms. A `logs/host` pipeline collecting journald logs through the `resource_detection` processor SHALL be included on Linux only; on macOS and Windows no host log pipeline is added. Application logs reach Dynatrace on all platforms through the existing `logs` pipeline fed by the `otlp` receiver.

#### Scenario: Linux collects metrics and journald logs

- **GIVEN** the target platform is Linux
- **WHEN** the configuration is generated
- **THEN** it SHALL include the `journald` receiver and a `logs/host` pipeline forwarding journald logs to Dynatrace through the `resource_detection` processor
- **AND** the `logs` pipeline fed by the `otlp` receiver SHALL remain present for application logs

#### Scenario: macOS and Windows collect metrics only, no host logs pipeline

- **GIVEN** the target platform is macOS or Windows
- **WHEN** the configuration is generated
- **THEN** it SHALL NOT include the `journald` receiver or a `logs/host` pipeline
- **AND** the `logs` pipeline fed by the `otlp` receiver SHALL remain present for application logs

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

### Requirement: Host monitoring is gated behind the experimental flag until fully implemented and tested

`install otel` SHALL only generate host monitoring pipelines and show the privilege notice when the `--experimental` flag or `DTWIZ_EXPERIMENTAL=true` environment variable is enabled, following the same convention used for `install docker` and `update otel`. When the flag is not enabled, `install otel` SHALL behave exactly as it did before this change.

#### Scenario: Host monitoring disabled by default

- **GIVEN** `--experimental` is not set and `DTWIZ_EXPERIMENTAL` is not `true`
- **WHEN** `install otel` generates the managed collector configuration
- **THEN** the configuration SHALL contain only the application pipelines that existed before this change
- **AND** it SHALL NOT include `hostmetrics`, `journald`, or `health_check` receivers/extensions, host pipelines, or a privilege notice

#### Scenario: Host monitoring enabled via the experimental flag

- **GIVEN** `--experimental` is passed or `DTWIZ_EXPERIMENTAL=true` is set
- **WHEN** `install otel` runs
- **THEN** dtwiz SHALL configure host monitoring as described in the requirements above

#### Scenario: Discoverability notice when disabled

- **GIVEN** `--experimental` is not set and `DTWIZ_EXPERIMENTAL` is not `true`
- **WHEN** `install otel` runs
- **THEN** the output SHALL display an informational box directing the user to the OpenTelemetry Host Monitoring documentation to activate host monitoring manually
