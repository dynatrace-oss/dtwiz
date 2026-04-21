# Spec: Status Extensions

## ADDED Requirements

### Requirement: Centralized HTTP client with Classic and Platform sub-clients

The system SHALL provide a `NewHTTPClient()` constructor in `cmd/` that returns a `Client` containing a `ClassicClient` (Classic API, `Api-Token` auth) and a `PlatformClient` (Platform/Apps API, `Bearer` auth). Both sub-clients SHALL share the same retry policy, timeout, and debug-logging behaviour.

#### Scenario: Client construction with all credentials present

- **GIVEN** `DT_ENVIRONMENT`, `DT_ACCESS_TOKEN`, and `DT_PLATFORM_TOKEN` are all set
- **WHEN** `NewHTTPClient()` is called
- **THEN** it returns a `Client` with `Classic` pointing at the Classic API base URL and `Platform` pointing at the Apps URL, each pre-configured with the correct `Authorization` header

#### Scenario: Client construction fails when environment URL is missing

- **GIVEN** `DT_ENVIRONMENT` is not set and `--environment` is not provided
- **WHEN** `NewHTTPClient()` is called
- **THEN** it returns an error with a message explaining how to set the environment URL

#### Scenario: Client construction fails when access token is missing

- **GIVEN** `DT_ACCESS_TOKEN` is not set and `--access-token` is not provided
- **WHEN** `NewHTTPClient()` is called
- **THEN** it returns an error with a message explaining how to set the access token

#### Scenario: Client construction fails when platform token is missing

- **GIVEN** `DT_PLATFORM_TOKEN` is not set and `--platform-token` is not provided
- **WHEN** `NewHTTPClient()` is called
- **THEN** it returns an error with a message explaining how to set the platform token

### Requirement: Retry on transient errors

The HTTP client SHALL automatically retry requests that fail with a network error, HTTP 429 (rate limited), or any HTTP 5xx response. It SHALL not retry on 4xx responses other than 429.

#### Scenario: Request retried on 429

- **GIVEN** the Dynatrace API returns HTTP 429 on the first attempt
- **WHEN** the retry condition is evaluated
- **THEN** the request is retried up to 3 times with exponential backoff between 1 s and 10 s

#### Scenario: No retry on 4xx client error

- **GIVEN** the Dynatrace API returns HTTP 403 (forbidden)
- **WHEN** the retry condition is evaluated
- **THEN** the request is not retried and the 403 response is returned immediately

### Requirement: Request/response debug logging

When verbosity is enabled (`-v` or `--debug`), the client SHALL log request and response details to stderr. Sensitive headers SHALL be redacted. Response bodies SHALL be capped at 2048 bytes to prevent large or sensitive payloads from flooding stderr.

#### Scenario: Verbose mode logs request method and URL

- **GIVEN** the user passes `-v`
- **WHEN** any HTTP request is made via the client
- **THEN** the method and full URL are printed to stderr before the request is sent

#### Scenario: Debug mode additionally logs headers and response body

- **GIVEN** the user passes `--debug`
- **WHEN** any HTTP request is made via the client
- **THEN** request headers and the first 2048 bytes of the response body are printed to stderr; `Authorization`, `x-api-key`, `cookie`, and `set-cookie` header values are replaced with `[REDACTED]`

#### Scenario: Response body truncated when larger than 2048 bytes

- **GIVEN** the user passes `--debug` and the response body exceeds 2048 bytes
- **WHEN** the response is received
- **THEN** only the first 2048 bytes are printed, followed by `[... truncated]` and the total byte count

### Requirement: --extensions flag on dtwiz status

The system SHALL add an `--extensions` flag to `dtwiz status` that, when set, probes both the Classic Extensions API and the Platform Extensions API and displays reachability and installed count for each.

#### Scenario: Both APIs reachable

- **GIVEN** `--extensions` is set and both credentials are valid
- **WHEN** `dtwiz status --extensions` runs
- **THEN** the output shows an "Extensions API" section with two lines:
  - `Classic Extensions: ✓ reachable (N extensions)`
  - `Platform Extensions: ✓ reachable (N packages)`

#### Scenario: Classic API returns HTTP error

- **GIVEN** `--extensions` is set and `GET /api/v2/extensions` returns HTTP 4xx or 5xx
- **WHEN** `dtwiz status --extensions` runs
- **THEN** the Classic Extensions line shows `✗ HTTP <status-code>` in red

#### Scenario: Platform API returns HTTP error

- **GIVEN** `--extensions` is set and `GET /platform/extensions/v2/extensions` returns HTTP 4xx or 5xx
- **WHEN** `dtwiz status --extensions` runs
- **THEN** the Platform Extensions line shows `✗ HTTP <status-code>` in red

#### Scenario: Client construction fails (missing credential)

- **GIVEN** `--extensions` is set but a required credential is not configured
- **WHEN** `dtwiz status --extensions` runs
- **THEN** the Extensions API section shows a single `✗` line with the credential error; the rest of the status output (token validation, system analysis) is unaffected

#### Scenario: --extensions not set

- **GIVEN** the user runs `dtwiz status` without `--extensions`
- **WHEN** the command executes
- **THEN** no Extensions API probe is performed and the section is not shown
