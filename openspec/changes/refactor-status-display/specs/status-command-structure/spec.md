# Status Command Structure

## ADDED Requirements

### Requirement: Credential status printed via shared helper

`dtwiz status` SHALL print Access Token and Platform Token status using a single shared code path. For each token the output SHALL follow this logic:

- Token not set → `✗ not set (use --<cli-name> or <ENV_VAR>)` in `ColorError`
- Token set, no environment URL → `✓ configured (skipped validation — no environment URL)` in `ColorOK`
- Token set, environment URL present, validation fails → `✗ <error>` in `ColorError`
- Token set, environment URL present, validation passes → `✓ valid (<url>)` in `ColorOK`, where `<url>` is the token-appropriate URL (classic API URL for Access Token, Apps URL for Platform Token)

#### Scenario: Access Token valid

- **GIVEN** `DT_ACCESS_TOKEN` is set and `DT_ENVIRONMENT` resolves to a reachable environment
- **WHEN** the user runs `dtwiz status`
- **THEN** the Access Token line shows `✓ valid (https://<env-id>.live.dynatrace.com/api/v2/)` in green bold

#### Scenario: Platform Token valid

- **GIVEN** `DT_PLATFORM_TOKEN` is set and `DT_ENVIRONMENT` resolves to a reachable environment
- **WHEN** the user runs `dtwiz status`
- **THEN** the Platform Token line shows `✓ valid (https://<env-id>.apps.dynatrace.com/)` in green bold

#### Scenario: Token not set

- **GIVEN** `DT_ACCESS_TOKEN` is not set
- **WHEN** the user runs `dtwiz status`
- **THEN** the Access Token line shows `✗ not set (use --access-token or DT_ACCESS_TOKEN)` in red bold

#### Scenario: Token set but no environment

- **GIVEN** `DT_ACCESS_TOKEN` is set but `DT_ENVIRONMENT` is not set
- **WHEN** the user runs `dtwiz status`
- **THEN** the Access Token line shows `✓ configured (skipped validation — no environment URL)` in green bold

### Requirement: System analysis error exits non-zero

`dtwiz status` SHALL return an error (exit non-zero) when `analyzer.AnalyzeSystem()` fails. The error message SHALL be printed as `✗ system analysis failed: <error>` in `ColorError` before returning.

#### Scenario: System analysis fails

- **GIVEN** `analyzer.AnalyzeSystem()` returns an error
- **WHEN** the user runs `dtwiz status`
- **THEN** the output shows `✗ system analysis failed: <error>` in red bold and the process exits with a non-zero exit code

### Requirement: Output sections use display package helpers

All section headings in `dtwiz status` (Connection Status, System Analysis, Feature Flags) SHALL be printed via `display.Header()` followed by `display.PrintSectionDivider()`. All credential status lines SHALL be printed via `display.PrintStatusLine()`.

#### Scenario: Connection Status section heading

- **GIVEN** the user runs `dtwiz status`
- **WHEN** the Connection Status section renders
- **THEN** it begins with the text "Connection Status" indented by two spaces, in magenta bold, followed by a `─` separator line
