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

### Requirement: dtwiz never writes to a third-party collector's configuration

`install otel` SHALL only ever write to its own managed collector configuration. It SHALL NOT modify, merge into, or otherwise write to the configuration of any other OTel Collector running on the host, regardless of the outcome of conflict detection.

#### Scenario: Combined config in the managed collector

- **GIVEN** no other OTel Collector is running on the host
- **WHEN** `install otel` completes
- **THEN** exactly one dtwiz-managed collector SHALL run, carrying both host and application pipelines

#### Scenario: dtwiz's own collector coexists with a foreign collector

- **GIVEN** a non-dtwiz OTel Collector is already running on the host
- **WHEN** `install otel` deploys host monitoring
- **THEN** dtwiz SHALL deploy or update only its own managed collector
- **AND** it SHALL NOT read the foreign collector's configuration for any purpose other than the conflict check described below, and SHALL NOT write to it under any circumstance
- **AND** it SHALL select non-conflicting ports, including the health check port, so both collectors run and start up successfully at the same time

### Requirement: Same-tenant host monitoring conflicts are detected and surfaced before proceeding

Before deploying host monitoring, `install otel` SHALL check whether another running OTel Collector already collects host metrics for the same Dynatrace tenant, using only read access to that collector's configuration file. Detection SHALL never trigger a write to the foreign configuration.

#### Scenario: No conflict when the foreign collector has no host metrics

- **GIVEN** another OTel Collector is running on the host
- **AND** its configuration does not define a `hostmetrics` receiver
- **WHEN** `install otel` deploys host monitoring
- **THEN** dtwiz SHALL proceed without warning the user of a conflict

#### Scenario: No conflict when the foreign collector targets a different tenant

- **GIVEN** another OTel Collector is running on the host with a `hostmetrics` receiver already configured
- **AND** its exporter targets a different Dynatrace tenant than the one `install otel` is configuring
- **WHEN** `install otel` deploys host monitoring
- **THEN** dtwiz SHALL proceed without warning the user of a conflict

#### Scenario: Conflict detected for the same tenant

- **GIVEN** another OTel Collector is running on the host with a `hostmetrics` receiver already configured
- **AND** its exporter targets the same Dynatrace tenant that `install otel` is configuring
- **WHEN** `install otel` deploys host monitoring
- **THEN** dtwiz SHALL warn the user that host metrics may be duplicated
- **AND** it SHALL let the user choose to skip host monitoring for this run or proceed anyway with dtwiz's own collector
- **AND** it SHALL mention `update otel` as the existing command for consolidating onto the foreign collector directly, without attempting that consolidation itself

#### Scenario: Tenant cannot be determined

- **GIVEN** another OTel Collector is running on the host with a `hostmetrics` receiver already configured
- **AND** its exporter endpoint cannot be resolved to a tenant from the static configuration file (for example, because it references an environment variable)
- **WHEN** `install otel` deploys host monitoring
- **THEN** dtwiz SHALL note that the conflict check was inconclusive
- **AND** it SHALL proceed with its own managed collector by default, without a blocking prompt

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
