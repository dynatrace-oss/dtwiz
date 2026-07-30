## ADDED Requirements

### Requirement: Host monitoring is configured during OTel Collector install

`install otel` SHALL configure the managed OTel Collector to collect host-level signals in addition to application signals, using the Dynatrace Host Monitoring reference configuration. Host monitoring SHALL be enabled by default without an additional flag.

#### Scenario: Host metrics pipeline added to managed collector

- **GIVEN** a user runs `install otel`
- **WHEN** dtwiz generates the managed collector configuration
- **THEN** the configuration SHALL include tiered `hostmetrics` receivers (10s, 5m, and 1h collection intervals)
- **AND** it SHALL include a metrics pipeline that routes those receivers through the `filter`, `resource_detection`, `transform`, `filter/delete-metrics`, and `cumulativetodelta` processors to the Dynatrace `otlp_http` exporter

#### Scenario: Existing application pipelines are preserved

- **GIVEN** the managed collector previously carried only application (`otlp`) pipelines
- **WHEN** host monitoring is configured
- **THEN** the application traces, metrics, and logs pipelines fed by the `otlp` receiver SHALL remain functional
- **AND** the application metrics pipeline SHALL be distinct from the host metrics pipeline so the two do not share receivers

#### Scenario: Single shared exporter

- **WHEN** the combined configuration is generated
- **THEN** host and application pipelines SHALL export to the same Dynatrace `/api/v2/otlp` endpoint using the same authorization header

### Requirement: Platform-aware host signal selection

The generated configuration SHALL include host log collection only on platforms where the required receiver is available, and SHALL collect host metrics on all supported platforms.

#### Scenario: Linux collects metrics and journald logs

- **GIVEN** the target platform is Linux
- **WHEN** the configuration is generated
- **THEN** it SHALL include the `journald` receiver and a logs pipeline that forwards host logs to Dynatrace through the `resource_detection` processor

#### Scenario: macOS and Windows collect metrics only

- **GIVEN** the target platform is macOS or Windows
- **WHEN** the configuration is generated
- **THEN** it SHALL include host metrics collection
- **AND** it SHALL NOT include the `journald` receiver, a host logs pipeline, or any other host-log receiver in its place

### Requirement: Preview and confirmation before applying

The combined configuration SHALL be shown to the user before it is written, consistent with the existing collector install preview.

#### Scenario: Combined config previewed with masked secrets

- **WHEN** `install otel` reaches the preview step
- **THEN** the full combined configuration SHALL be printed inline with the ingest token masked
- **AND** the user SHALL be asked a single confirmation prompt defaulting to yes before any file is written

#### Scenario: Dry-run makes no changes

- **GIVEN** `install otel --dry-run`
- **WHEN** the command runs
- **THEN** the combined configuration SHALL be shown
- **AND** no collector binary, configuration file, or process SHALL be created or modified

### Requirement: Privilege and platform limitations are surfaced

When host monitoring may be incomplete due to insufficient privileges or platform-specific gaps, dtwiz SHALL inform the user rather than imply full coverage. The privilege mechanism differs by platform, so the notice SHALL NOT be phrased as a Linux-only concern.

#### Scenario: Unprivileged host monitoring notice

- **GIVEN** host monitoring requires elevated privileges for some scrapers or `journald` on the current platform
- **WHEN** `install otel` configures host monitoring
- **THEN** the output SHALL note that some host metrics or logs may require elevated privileges to be collected, worded appropriately for the current platform

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
- **WHEN** `install otel` completes
- **THEN** the output SHALL include a one-line notice that host monitoring can be enabled with `--experimental` or `DTWIZ_EXPERIMENTAL=true`
