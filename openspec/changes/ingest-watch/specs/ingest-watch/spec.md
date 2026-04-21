# Spec: Ingest Watch

## ADDED Requirements

### Requirement: Watch command polls Dynatrace for ingested data

The system SHALL provide a `dtwiz watch` command that polls the Dynatrace DQL API every 5 seconds and displays a live-updating terminal summary of newly ingested data since the watch started.

#### Scenario: User runs watch command standalone

- **WHEN** user runs `dtwiz watch` with valid environment and platform token
- **THEN** the system polls Dynatrace every 5 seconds and displays counts and details for Services, Cloud, Kubernetes, Relationships, Logs, Requests, and Exceptions

#### Scenario: Watch starts after successful install

- **WHEN** an installer (oneagent, kubernetes, docker, otel, aws) completes successfully
- **THEN** the system automatically starts the ingest watch to show data flowing in

#### Scenario: Missing platform token

- **WHEN** user runs `dtwiz watch` without a platform token configured
- **THEN** the system prints an error message and exits without polling

### Requirement: Live in-place terminal rendering

The system SHALL use ANSI cursor movement to update the display in-place without scrolling, refreshing every 5 seconds with an elapsed time counter.

#### Scenario: TTY terminal output

- **WHEN** stdout is a TTY
- **THEN** the system uses cursor-up ANSI sequences to overwrite previous output each cycle

#### Scenario: Non-TTY output (piped or redirected)

- **WHEN** stdout is not a TTY
- **THEN** the system falls back to append-only output without ANSI cursor movement

### Requirement: Seven data sections with deep links

The system SHALL display seven data sections, each showing counts, details, and a deep link to the relevant Dynatrace app once data arrives.

#### Scenario: Services section with data

- **WHEN** Dynatrace returns service entities
- **THEN** the system displays section "Services" with count, up to 5 service names, "+N more" if needed, and a link to the services explorer

#### Scenario: Cloud section with data

- **WHEN** Dynatrace returns AWS_* entity types
- **THEN** the system displays section "Cloud" with total count, top 5 types by count with humanized names (strip AWS_ prefix, lowercase, pluralize), and a link to the clouds app

#### Scenario: Kubernetes section with data

- **WHEN** Dynatrace returns K8S_* or CONTAINER entity types
- **THEN** the system displays section "Kubernetes" with total count, top 5 types by count with humanized names, and a link to the kubernetes app

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

### Requirement: QuickStart link always visible

The system SHALL always display a prominent QuickStart link at the bottom of the output, visually separated with horizontal rules and rendered in the CLI highlight color (magenta bold).

#### Scenario: QuickStart link rendering

- **WHEN** the watch display renders
- **THEN** the bottom shows a magenta bold section with "See all your data and findings in Dynatrace QuickStart" preceded by a pointing finger emoji and a link to the QuickStart app

### Requirement: Load generation hint

The system SHALL display a hint telling users to generate load on their system to see data appear.

#### Scenario: Hint displayed below header

- **WHEN** the watch display renders
- **THEN** a dim/gray line reads "Generate some load on your system to see data appear." below the header

### Requirement: Elapsed time display

The system SHALL display elapsed time since the watch started in the header line.

#### Scenario: Elapsed time formatting

- **WHEN** the watch has been running for some time
- **THEN** the header shows "Watching for new data in Dynatrace... (elapsed: Xm Ys)"
