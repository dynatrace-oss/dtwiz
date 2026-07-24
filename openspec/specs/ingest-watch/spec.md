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

#### Scenario: Watch does not start after cancelled install

- **WHEN** the user selects `n` on the install confirmation prompt for any installer (oneagent, kubernetes, docker, otel, otel-collector, otel-python, otel-node, otel-java, aws, aws-lambda, demo)
- **THEN** the installer returns `ErrInstallCancelled` and exits cleanly
- **AND** the command layer treats `ErrInstallCancelled` as a non-error exit
- **AND** the system does not start ingest watch

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

- **WHEN** Dynatrace returns AWS\_\* entity types
- **THEN** the system displays section "Cloud" with total count, top 5 types by count with humanized names (strip AWS\_ prefix, lowercase, pluralize), and a link to the clouds app

#### Scenario: Kubernetes section with data

- **WHEN** Dynatrace returns K8S\_\* or CONTAINER entity types
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

## CHANGED Requirements

### Requirement: Watch command polls Dynatrace for ingested data

The system SHALL exit the watch after 10 minutes of continuous running and prompt the user to continue. If the user confirms, the timer resets and in-place rendering resumes. If the user declines or the session is non-TTY, the command exits cleanly.

#### Scenario: 10-minute timeout on TTY

- **GIVEN** the watch has been running for 10 minutes on a TTY terminal
- **WHEN** the timeout fires
- **THEN** the system prints "Continue watching? [Y/n]" inline without clearing the current display
- **AND** pressing y (or Enter) resets the timer and resumes in-place polling
- **AND** pressing n exits the command cleanly

#### Scenario: 10-minute timeout on non-TTY

- **GIVEN** the watch has been running for 10 minutes
- **AND** stdout is not a TTY (piped or redirected)
- **WHEN** the timeout fires
- **THEN** the command exits automatically without prompting

#### Scenario: In-place rendering continues after user presses y

- **GIVEN** the user pressed y at the continue prompt
- **WHEN** the next poll cycle completes
- **THEN** the display overwrites the previous frame in place (no fresh append below the prompt)

### Requirement: Logs query two-phase optimization

The system SHALL use a cheap probe query for Logs until the first log record is detected, then switch to the full summarize query to avoid expensive full scans on an empty dataset.

#### Scenario: Logs probe phase — no data yet

- **GIVEN** no log records have been ingested since the watch started
- **WHEN** the watch polls
- **THEN** the Logs section shows "waiting..."
- **AND** the system runs `fetch logs, from:X | limit 1` (not the full summarize)

#### Scenario: Logs probe phase — first log detected

- **GIVEN** the probe query returns at least one record
- **WHEN** the watch transitions to the metrics phase
- **THEN** the Logs section shows "Logs ingested"
- **AND** subsequent polls use `fetch logs, from:X | summarize count=count(), by:{loglevel}`

#### Scenario: Logs metrics phase — counts available

- **GIVEN** the system is in the Logs metrics phase
- **WHEN** the summarize query returns count > 0
- **THEN** the Logs section shows total count and per-level breakdown (info/warn/error) as before

### Requirement: Requests query two-phase optimization

The system SHALL use a cheap probe query for Requests until the first root span is detected, then switch to the full summarize query.

#### Scenario: Requests probe phase — no data yet

- **GIVEN** no spans have been ingested since the watch started
- **WHEN** the watch polls
- **THEN** the Requests section shows "waiting..."
- **AND** the system runs `fetch spans, from:X | filter request.is_root_span == true | limit 1`

#### Scenario: Requests probe phase — first span detected

- **GIVEN** the probe query returns at least one record
- **WHEN** the watch transitions to the metrics phase
- **THEN** the Requests section shows "Requests ingested"
- **AND** subsequent polls use `fetch spans, from:X | filter request.is_root_span == true | summarize failed=countIf(request.is_failed == true), success=countIf(request.is_failed != true)`

#### Scenario: Requests metrics phase — counts available

- **GIVEN** the system is in the Requests metrics phase
- **WHEN** the summarize query returns count > 0
- **THEN** the Requests section shows total count and success/failed breakdown as before
