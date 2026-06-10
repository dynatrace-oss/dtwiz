## MODIFIED Requirements

### Requirement: Access token is optional; used as Classic API fallback

Access token is optional and **opt-in via the `--access-token` flag only**. It activates for Classic API calls solely when `--access-token` is passed explicitly on the command line. The `DT_ACCESS_TOKEN` environment variable SHALL NOT be read as an activation source, so a leftover env var can never switch Classic API calls onto access-token auth. When `--access-token` is absent, the platform token is used for Classic API calls. This opt-in path exists only until all Classic API endpoints accept platform tokens.

#### Scenario: Access token provided via flag

- **GIVEN** `--access-token dt0c01.****` is passed and differs from the platform token
- **WHEN** the user runs any command requiring Classic API access
- **THEN** the access token is used for Classic API calls and `--debug` logs `classic API auth: using explicit access token`

#### Scenario: DT_ACCESS_TOKEN set but flag absent

- **GIVEN** `DT_ACCESS_TOKEN` is exported but `--access-token` is NOT passed
- **WHEN** the user runs any command
- **THEN** the access token is ignored and the platform token is used for API calls

#### Scenario: No access token configured

- **GIVEN** Only `DT_PLATFORM_TOKEN` (or `--platform-token`) is set
- **WHEN** the user runs any install command (e.g. `dtwiz install otel --dry-run`)
- **THEN** the command completes using the platform token for Classic API calls

---

### Requirement: Status output labels access token as optional fallback

`dtwiz status` SHALL show the Access Token row only when `--access-token` is provided. The row SHALL NOT be triggered by the `DT_ACCESS_TOKEN` env var.

#### Scenario: Access token flag provided

- **GIVEN** `--access-token` is passed and `DT_ENVIRONMENT` resolves to a reachable environment
- **WHEN** the user runs `dtwiz status`
- **THEN** the output shows a `Platform Token` row and an `Access Token` row

#### Scenario: Flag absent (env var ignored)

- **GIVEN** `--access-token` is NOT passed, even if `DT_ACCESS_TOKEN` is exported
- **WHEN** the user runs `dtwiz status`
- **THEN** no Access Token row is shown and only the `Platform Token` row appears
