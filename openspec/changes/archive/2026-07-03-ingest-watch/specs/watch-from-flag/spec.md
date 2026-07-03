# Spec: Watch --from Flag

## ADDED Requirements

### Requirement: Configurable query start time via --from flag

The system SHALL accept a `--from` flag on `dtwiz watch` that sets the lower bound of all DQL queries. The flag accepts either an RFC3339 timestamp or a DQL relative expression (e.g. `now()-1h`). When omitted, the start time defaults to the dtwiz process start time so the watch only shows data ingested after the CLI launched.

#### Scenario: --from with RFC3339 timestamp

- **GIVEN** the user runs `dtwiz watch --from 2026-04-21T14:30:05Z`
- **WHEN** the watch command initializes
- **THEN** all DQL queries use `2026-04-21T14:30:05Z` as the `from` clause

#### Scenario: --from with DQL relative expression

- **GIVEN** the user runs `dtwiz watch --from "now()-1h"`
- **WHEN** the watch command initializes
- **THEN** all DQL queries use `now()-1h` as the `from` clause

#### Scenario: Default start time when --from is omitted

- **GIVEN** the user runs `dtwiz watch` without `--from`
- **WHEN** the watch command initializes
- **THEN** all DQL queries use the dtwiz process start time (formatted as RFC3339 UTC, e.g. `2026-04-21T14:30:05Z`) as the `from` clause

#### Scenario: Installer-triggered watch inherits process start time

- **GIVEN** an installer completes successfully and triggers `WatchIngest()` automatically
- **WHEN** no explicit `--from` is provided
- **THEN** the watch queries use the process start time so only data from the current session is shown, not historical data
