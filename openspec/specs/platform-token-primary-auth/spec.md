# Platform Token Primary Authentication

## Purpose

Dynatrace is deprecating access tokens. New tenants can no longer create them. Platform tokens (`dt0s16.*`) are the replacement and already work for all ingest endpoints. Classic API endpoints are being updated to accept platform tokens as well.

This change makes platform token the required primary credential. Access token (`dt0c01.*`) is retained as an optional fallback for Classic API calls that do not yet accept platform tokens. Once Classic API coverage is complete and verified, access token support will be removed entirely.

## Requirements

### Requirement: Platform token is required

All dtwiz commands SHALL require `DT_PLATFORM_TOKEN` to be set. If not set, the command SHALL exit with an error.

#### Scenario: Platform token missing

- **GIVEN** `DT_PLATFORM_TOKEN` is not set
- **WHEN** the user runs any dtwiz command
- **THEN** the command exits with `no Dynatrace platform token configured`

---

### Requirement: Access token is optional; used as Classic API fallback

Access token is optional. At startup, dtwiz SHALL probe the Classic API with the platform token. If rejected (401/403), the access token SHALL be used instead for Classic API calls. This fallback exists only until all Classic API endpoints accept platform tokens.

#### Scenario: Platform token accepted by Classic API

- **GIVEN** `DT_PLATFORM_TOKEN` is set (and optionally `DT_ACCESS_TOKEN`)
- **WHEN** the Classic API probe returns a non-401/403 response
- **THEN** the platform token is used for Classic API calls and `--debug` logs `classic API auth: platform token accepted`

#### Scenario: Platform token rejected, access token available

- **GIVEN** Both `DT_PLATFORM_TOKEN` and `DT_ACCESS_TOKEN` are set
- **WHEN** the Classic API probe returns 401 or 403
- **THEN** the access token is used for Classic API calls and `--debug` logs `classic API auth: using access token as fallback`

#### Scenario: Platform token rejected, no access token

- **GIVEN** Only `DT_PLATFORM_TOKEN` is set
- **WHEN** the Classic API probe returns 401 or 403
- **THEN** the platform token is used as best effort and `--debug` logs `classic API auth: no access token configured, proceeding with platform token`

#### Scenario: Only platform token set, command succeeds

- **GIVEN** Only `DT_PLATFORM_TOKEN` is set
- **WHEN** the user runs any install command (e.g. `dtwiz install otel --dry-run`)
- **THEN** the command completes successfully

---

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

---

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

---

### Requirement: Status output labels access token as optional fallback

`dtwiz status` SHALL label the access token row as `"Access Token (fallback)"` to communicate its optional, transitional role.

#### Scenario: Both tokens set

- **GIVEN** Both `DT_PLATFORM_TOKEN` and `DT_ACCESS_TOKEN` are set
- **WHEN** the user runs `dtwiz status`
- **THEN** the output shows `Platform Token` and `Access Token (fallback)` as two separate rows

#### Scenario: Only platform token set

- **GIVEN** Only `DT_PLATFORM_TOKEN` is set
- **WHEN** the user runs `dtwiz status`
- **THEN** `Access Token (fallback)` shows as `✗ not set` without causing an error
