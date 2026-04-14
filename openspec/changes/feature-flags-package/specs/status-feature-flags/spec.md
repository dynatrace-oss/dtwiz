# Status Feature Flags

## ADDED Requirements

### Requirement: Active feature flags displayed in status output

`dtwiz status` SHALL display a "Feature Flags" section when any feature flag is enabled. The section SHALL list each enabled flag with its env var name and source. When no flags are enabled, the section SHALL be omitted entirely.

#### Scenario: One flag enabled via env var

- **GIVEN** `DTWIZ_ALL_RUNTIMES=true` is set in the environment
- **WHEN** the user runs `dtwiz status`
- **THEN** the output includes a "Feature Flags" section showing `DTWIZ_ALL_RUNTIMES  enabled (env)`

#### Scenario: Flag enabled via CLI flag

- **GIVEN** `--all-runtimes` is passed on the command line
- **WHEN** the user runs `dtwiz status --all-runtimes`
- **THEN** the output includes a "Feature Flags" section showing `DTWIZ_ALL_RUNTIMES  enabled (cli)`

#### Scenario: No flags enabled

- **GIVEN** no feature flag env vars are set and no feature flag CLI flags are passed
- **WHEN** the user runs `dtwiz status`
- **THEN** no "Feature Flags" section appears in the output
