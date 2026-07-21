# Finer Grained Error Messages

## MODIFIED Requirements

### Requirement: Platform token DQL validation is a hard requirement

At startup, all commands SHALL validate the platform token via DQL query. Failure exits with an error regardless of whether an access token is set. The error message SHALL distinguish authentication failure (401) from insufficient permissions (403).

#### Scenario: Platform token DQL validation fails with invalid or expired token

- **GIVEN** `DT_PLATFORM_TOKEN` is set to an invalid or expired token
- **WHEN** the user runs any command
- **THEN** the command exits with `✗ Platform token: authentication failed`

#### Scenario: Platform token DQL validation fails with insufficient permissions

- **GIVEN** `DT_PLATFORM_TOKEN` is set to a valid token that lacks the required DQL scope
- **WHEN** the user runs any command
- **THEN** the command exits with `✗ Platform token: insufficient permissions`

## ADDED Requirements

### Requirement: Access token validation distinguishes authentication failure from insufficient permissions

When an access token is configured, `dtwiz status` SHALL validate it via the Classic API token lookup. The error message SHALL distinguish authentication failure (401) from insufficient permissions (403).

#### Scenario: Access token validation fails with invalid or expired token

- **GIVEN** an access token is configured via `--access-token`
- **AND** the token is invalid or expired
- **WHEN** the user runs `dtwiz status`
- **THEN** the access token row shows `✗ Access token: authentication failed`

#### Scenario: Access token validation fails with insufficient permissions

- **GIVEN** an access token is configured via `--access-token`
- **AND** the token is valid but lacks the required scope
- **WHEN** the user runs `dtwiz status`
- **THEN** the access token row shows `✗ Access token: insufficient permissions`
