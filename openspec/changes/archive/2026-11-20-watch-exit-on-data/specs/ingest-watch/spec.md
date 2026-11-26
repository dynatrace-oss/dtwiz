# Spec Delta

## ADDED Requirements

### Requirement: Standalone watch can exit on first data

The `dtwiz watch` command SHALL accept an `--exit-on-data` flag. With the flag, the watch SHALL end on its own after the first poll that finds data in any of the eight data sections, and SHALL exit with code `0`. Without the flag, the watch SHALL keep running as before.

#### Scenario: Data arrives while watching with the flag

- **GIVEN** the user runs `dtwiz watch --exit-on-data`
- **AND** no data section had data on earlier polls
- **WHEN** a poll finds data in at least one data section
- **THEN** the system renders that poll's frame, including every section and the QuickStart link
- **AND** the frame does not show "Press Enter to continue or keep watching..."
- **AND** the command exits with code `0` without waiting for input

#### Scenario: Data already present on the first poll

- **GIVEN** the user runs `dtwiz watch --exit-on-data --from "now()-1h"`
- **AND** Dynatrace holds data for that time range
- **WHEN** the first poll completes
- **THEN** the system renders that frame and exits with code `0` without waiting for the next poll

#### Scenario: Flag on a non-TTY

- **GIVEN** stdout is not a TTY (piped or redirected)
- **AND** the user runs `dtwiz watch --exit-on-data`
- **WHEN** a poll finds data in at least one data section
- **THEN** the system prints that frame in the existing append-only format and exits with code `0`

#### Scenario: Enter still ends the watch early

- **GIVEN** the user runs `dtwiz watch --exit-on-data` on a TTY and no data has arrived yet
- **WHEN** the user presses Enter
- **THEN** the watch ends and the command exits with code `0`, as it does without the flag

#### Scenario: No flag

- **GIVEN** the user runs `dtwiz watch` without `--exit-on-data`
- **WHEN** a poll finds data in at least one data section
- **THEN** the watch keeps polling every 5 seconds until the user presses Enter or the 10-minute timeout ends it

### Requirement: Standalone watch with exit-on-data ends at the timeout without prompting

With `--exit-on-data`, when no data section has data by the 10-minute mark, `dtwiz watch` SHALL end without showing the "Continue watching? [Y/n]" prompt, on TTY and non-TTY output alike, and SHALL exit with code `0`.

#### Scenario: No data within 10 minutes on a TTY

- **GIVEN** the user runs `dtwiz watch --exit-on-data` on a TTY
- **AND** no data section has received data
- **WHEN** the watch reaches the 10-minute mark
- **THEN** the system does not print "Continue watching? [Y/n]" and does not read input
- **AND** the command exits with code `0`

#### Scenario: No data within 10 minutes on a non-TTY

- **GIVEN** the user runs `dtwiz watch --exit-on-data` with stdout not a TTY
- **AND** no data section has received data
- **WHEN** the watch reaches the 10-minute mark
- **THEN** the command exits with code `0`, as it does without the flag

#### Scenario: Flag combined with --from

- **GIVEN** the user runs `dtwiz watch --exit-on-data --from 2026-04-21T14:30:05Z`
- **WHEN** the watch polls
- **THEN** all queries use `2026-04-21T14:30:05Z` as their start time, as `--from` defines, and the exit-on-data behaviour applies unchanged

### Requirement: Post-install watches do not exit on first data

The watch that starts after a successful `install`, `update` or `setup` SHALL NOT end on its own when data arrives, whether or not `--yes` was passed. `--yes` SHALL only skip confirmation prompts.

#### Scenario: Install with --yes

- **GIVEN** the user runs `dtwiz install oneagent --yes`
- **AND** the install completes successfully
- **WHEN** the post-install watch finds data in a data section
- **THEN** the watch keeps polling until the user presses Enter or the 10-minute timeout ends it

#### Scenario: Setup

- **GIVEN** the user runs `dtwiz setup` and an install completes successfully
- **WHEN** the post-install watch finds data in a data section
- **THEN** the watch keeps polling until the user presses Enter or the 10-minute timeout ends it
