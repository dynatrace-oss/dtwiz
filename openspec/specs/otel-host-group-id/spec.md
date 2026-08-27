# OTel Host Group ID Spec

## Purpose

Define how the managed OTel Collector sets the `dt.host_group.id` resource attribute for host-monitoring telemetry.

## Requirements

### Requirement: Collector config sets dt.host_group.id on host monitoring telemetry

The generated OTel Collector configuration SHALL include a `resource/add-host-group-id` processor in the regular `dtwiz install otel` flow. This processor SHALL upsert the `dt.host_group.id` resource attribute to the hostname of the machine on which `dtwiz` is run, and SHALL be applied to all generated pipelines (`metrics/apps`, `metrics/host`, `traces`, `logs`).

#### Scenario: dt.host_group.id is set in standard install

- **GIVEN** a user runs `dtwiz install otel` on a machine with hostname `my-machine`
- **WHEN** the OTel Collector configuration is generated
- **THEN** the config contains a `resource/add-host-group-id` processor with `dt.host_group.id` set to `my-machine`
- **THEN** all pipelines (`traces`, `metrics/apps`, `metrics/host`, `logs`) reference `resource/add-host-group-id`

#### Scenario: existing dt.host_group.id is overwritten

- **GIVEN** an application sends telemetry with `dt.host_group.id` already set to some value
- **WHEN** that telemetry passes through the collector
- **THEN** `dt.host_group.id` is overwritten with the machine hostname (upsert semantics)

#### Scenario: hostname resolution failure

- **GIVEN** the machine hostname cannot be resolved at install time
- **WHEN** the OTel Collector configuration is generated
- **THEN** `dt.host_group.id` is set to an empty string and config generation succeeds without error
