# Watch Query Optimization

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
