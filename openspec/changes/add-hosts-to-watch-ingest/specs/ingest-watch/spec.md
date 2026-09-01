# Spec: Ingest Watch

## MODIFIED Requirements

### Requirement: Watch command polls Dynatrace for ingested data

The system SHALL provide a `dtwiz watch` command that polls the Dynatrace DQL API every 5 seconds and displays a live-updating terminal summary of newly ingested data since the watch started.

#### Scenario: User runs watch command standalone

- **WHEN** user runs `dtwiz watch` with valid environment and platform token
- **THEN** the system polls Dynatrace every 5 seconds and displays counts and details for Services, Hosts, Kubernetes, Cloud, Relationships, Logs, Requests, and Exceptions

#### Scenario: Watch starts after successful install

- **WHEN** an installer (oneagent, kubernetes, docker, otel, aws) completes successfully
- **THEN** the system automatically starts the ingest watch to show data flowing in

#### Scenario: Watch does not start after cancelled install

- **WHEN** the user selects `n` on the install confirmation prompt for any installer (oneagent, kubernetes, docker, otel, otel-collector, otel-python, otel-node, otel-java, aws, aws-lambda, demo)
- **THEN** the installer returns `ErrInstallCancelled` and exits cleanly
- **AND** the command layer treats `ErrInstallCancelled` as a non-error exit
- **AND** the system does not start ingest watch

#### Scenario: Missing platform token

- **WHEN** user runs `dtwiz watch` without a platform token configured
- **THEN** the system prints an error message and exits without polling

### Requirement: Seven data sections with deep links

The system SHALL display eight data sections, each showing counts, details, and a deep link to the relevant Dynatrace app once data arrives.

#### Scenario: Services section with data

- **WHEN** Dynatrace returns service entities
- **THEN** the system displays section "Services" with count, up to 5 service names, "+N more" if needed, and a link to the services explorer

#### Scenario: Hosts section with data

- **WHEN** Dynatrace returns regular host or OpenTelemetry host entities
- **THEN** the system displays section "Hosts" with a combined host count
- **AND** regular host and OpenTelemetry host entries are listed together under "Hosts"
- **AND** each listed host has a link to that host's detail page

#### Scenario: Kubernetes section with data

- **WHEN** Dynatrace returns K8S\_\* or CONTAINER entity types
- **THEN** the system displays section "Kubernetes" with total count, top 5 types by count with humanized names, and a link to the kubernetes app

#### Scenario: Cloud section with data

- **WHEN** Dynatrace returns AWS\_\*, AZURE\_\*, or GCP\_\* entity types
- **THEN** the system displays section "Cloud" with total count, top 5 types by count with humanized names, and a link to the clouds app

#### Scenario: Section order

- **WHEN** the watch display renders
- **THEN** the sections appear in this order: Services, Hosts, Kubernetes, Cloud, Relationships, Logs, Requests, Exceptions

#### Scenario: Relationships section with data

- **WHEN** Dynatrace returns smartscape edges
- **THEN** the system displays "Relationships" with a count and a link to the smartscape view

#### Scenario: Logs section with data

- **WHEN** Dynatrace returns log records
- **THEN** the system displays "Logs" with total count, breakdown by log level (info/warn/error), and a link to the logs app

#### Scenario: Requests section with data

- **WHEN** Dynatrace returns span records for root spans
- **THEN** the system displays "Requests" with total count, successful vs failed breakdown, and a link to distributed tracing

#### Scenario: Exceptions section with data

- **WHEN** Dynatrace returns spans with exception events
- **THEN** the system displays "Exceptions" with count and a link to the exceptions explorer

#### Scenario: Section with no data yet

- **WHEN** a data section has no results
- **THEN** the system displays the section name with "waiting..." in dim/gray text and no link
