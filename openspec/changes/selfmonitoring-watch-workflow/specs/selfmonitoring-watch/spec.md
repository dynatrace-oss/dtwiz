# Spec: Self-Monitoring Watch Workflow

## NEW Requirements

### Requirement: dtwiz watch emits one post-session self-monitoring span

The system SHALL emit exactly one self-monitoring event per `dtwiz watch` invocation, covering the full session from start to when the session ends (user exit or timeout).

#### Scenario: First data is received during watch

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** the first poll cycle after `dtwiz watch` starts returns at least one signal with data
- **THEN** the system sends one self-monitoring event immediately (without waiting for the user to exit) with `st=com`, `watch.exit=first_data`, a non-empty `watch.sig` listing the signals seen, and `watch.t_<signal>` fields for each signal with the milliseconds to first data; the watch display continues uninterrupted

#### Scenario: Watch session ends with no data — user exits

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** the user runs `dtwiz watch` and presses Enter before any signal receives data
- **THEN** no self-monitoring event is sent — drop-off is visible by the absence of a span for that execution

#### Scenario: Watch session times out with no data

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** the watch session reaches the 10-minute timeout and the user declines to continue (or the session is non-TTY), and no signal data was seen
- **THEN** the system sends one self-monitoring event with `st=com`, `watch.exit=timeout`, and `watch.sig=""`

#### Scenario: Watch session times out after data was already received

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured and first data was already received (event already fired with `watch.exit=first_data`)
- **WHEN** the watch session subsequently reaches the 10-minute timeout
- **THEN** no second event is sent — the span was already emitted at first-data time

#### Scenario: Feature flag disabled

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is not enabled
- **WHEN** the user runs `dtwiz watch` and exits
- **THEN** no self-monitoring event is sent

#### Scenario: No invocation event is fired for watch

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the user runs `dtwiz watch`
- **THEN** no `st=inv` event is sent — watch emits only the single post-session span

#### Scenario: Event flush does not block the user noticeably

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled
- **WHEN** the watch session ends and the self-monitoring event is being flushed
- **THEN** the system waits at most 500ms for the flush before exiting; if the flush does not complete within 500ms the event is silently dropped

## MODIFIED Requirements

### Requirement: Self-monitoring events carry a meaningful title

The system SHALL set the event title to `"dtwiz <cmd>"` or `"dtwiz <cmd> <sub>"` (using the normalized command and subcommand identifiers) for all self-monitoring events.

#### Scenario: Top-level command event

- **GIVEN** a self-monitoring event is sent for a top-level command (e.g. `watch`, `status`)
- **WHEN** the event is ingested
- **THEN** the event title is `"dtwiz <cmd>"` (e.g. `"dtwiz wch"`)

#### Scenario: Subcommand event

- **GIVEN** a self-monitoring event is sent for a subcommand (e.g. `install otel`)
- **WHEN** the event is ingested
- **THEN** the event title is `"dtwiz <cmd> <sub>"` (e.g. `"dtwiz ins otel"`)
