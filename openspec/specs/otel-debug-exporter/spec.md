# otel-debug-exporter

## Purpose

Include a `debug` exporter in the generated OTel Collector config when dtwiz is run with `--debug`, so operators can observe live telemetry flowing through the collector pipelines during troubleshooting.

## Requirements

### Requirement: Debug exporter included in generated collector config when debug mode is active

When the OTel Collector config is generated (for install or update) and debug mode is active, the generated YAML SHALL include a `debug` exporter configured with `verbosity: normal`. Every pipeline in the config (traces, metrics, logs and their host-monitoring variants) SHALL list `debug` as an exporter alongside `otlp_http`.

#### Scenario: Install with --debug includes debug exporter

- **GIVEN** the user runs `dtwiz install otel` with the `--debug` flag
- **WHEN** the collector config is generated and written to disk
- **THEN** the config contains a `debug` exporter with `verbosity: normal`
- **THEN** all pipelines list `debug` as an exporter

#### Scenario: Install without --debug excludes debug exporter

- **GIVEN** the user runs `dtwiz install otel` without the `--debug` flag
- **WHEN** the collector config is generated and written to disk
- **THEN** the config does not contain a `debug` exporter
- **THEN** all pipeline exporter lists contain only `otlp_http`

#### Scenario: Update with --debug includes debug exporter

- **GIVEN** the user runs `dtwiz update otel` with the `--debug` flag
- **WHEN** the collector config is regenerated and written to disk
- **THEN** the config contains a `debug` exporter with `verbosity: normal`
- **THEN** all pipelines list `debug` as an exporter

#### Scenario: Update without --debug removes debug exporter from previous debug install

- **GIVEN** the collector config previously contained a `debug` exporter (installed with `--debug`)
- **WHEN** the user runs `dtwiz update otel` without the `--debug` flag
- **THEN** the regenerated config does not contain a `debug` exporter

#### Scenario: Host monitoring mode includes debug exporter in all host pipelines

- **GIVEN** the user runs `dtwiz install otel` with `--debug` and host monitoring is enabled
- **WHEN** the collector config is generated
- **THEN** all four pipelines (traces, metrics/apps, metrics/host, logs) list `debug` as an exporter
