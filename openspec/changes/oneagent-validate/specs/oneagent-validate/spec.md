# OneAgent Post-Install Verification

## ADDED Requirements

### Requirement: Host registration polling via DQL/Grail

`InstallOneAgentV2` SHALL confirm post-install that the host has registered with the tenant by polling Grail through the Platform API. The implementation SHALL `POST` a DQL query to `<apps-url>/platform/storage/query/v1/query:execute` with Bearer auth (platform token). The DQL query SHALL look up smartscape HOST nodes filtered to the local hostname (case-insensitive). On the first non-empty result set, the function SHALL extract the `id` field as the `entityId` and return it.

The post-install verification SHALL NOT use the classic `/api/v1/entity/infrastructure/hosts` endpoint — staying on the Platform/Grail path keeps verification consistent with `WatchIngest` (`pkg/installer/ingest_watch.go`), reuses the platform token, and avoids introducing a second URL family to the v2 flow.

#### Scenario: Host registers within timeout

- **GIVEN** the installer subprocess succeeded
- **AND** smartscape returns a HOST node matching the local hostname on the second poll
- **WHEN** `WaitForHostRegistration(p, hostname, 2*time.Minute)` runs
- **THEN** the function returns `(entityId, nil)` where `entityId` is the `id` field from the DQL result (e.g. `"HOST-abc123"`)
- **AND** outputs `display.PrintStatusLine("host", "registered <entityId>", display.ColorOK)`

#### Scenario: DQL request shape

- **GIVEN** the local hostname is `"prod-web-01"`
- **WHEN** `WaitForHostRegistration` issues a poll
- **THEN** the HTTP request is `POST <apps-url>/platform/storage/query/v1/query:execute`
- **AND** the `Authorization` header is `Bearer <platform-token>`
- **AND** the body's `query` field contains `smartscapeNodes HOST`, a filter on `name` (case-insensitive against `"prod-web-01"`), and selects `id` and `name`
- **AND** the body sets `requestTimeoutMilliseconds` and `maxResultRecords` to values consistent with `executeDQL` in `ingest_watch.go`

#### Scenario: Polling cadence

- **GIVEN** the host has not registered yet
- **WHEN** `WaitForHostRegistration` polls the API
- **THEN** consecutive poll requests are issued approximately 5 seconds apart (matching the polling cadence in `watchIngest()` function defined by `watchPollInterval` in `pkg/installer/ingest_watch.go`)

#### Scenario: Missing platform token skips verification

- **GIVEN** no `--platform-token` and no `DT_PLATFORM_TOKEN` is set
- **WHEN** `WaitForHostRegistration` is called
- **THEN** it prints `⚠ Platform token not set — skipping host registration verification.`
- **AND** returns `("", nil)` without issuing any HTTP request
- **AND** `InstallOneAgentV2` returns success

### Requirement: Timeout is a warning, not a failure

When the poll timeout elapses without the host appearing, `WaitForHostRegistration` SHALL return `("", nil)` and output `display.PrintStatusLine("warning", "Host registration verification timed out after 2 minutes. Check the tenant UI.", display.ColorWarning)`. The overall `InstallOneAgentV2` SHALL still return success (the installer subprocess already exited 0).

#### Scenario: Timeout warns but succeeds

- **GIVEN** the host never appears within 2 minutes
- **WHEN** `InstallOneAgentV2` returns
- **THEN** stdout contains the timeout warning via `display.PrintStatusLine`
- **AND** `InstallOneAgentV2` returns nil (the command exits 0)

#### Scenario: Skipped after --dry-run

- **GIVEN** `--dry-run` was passed
- **WHEN** `InstallOneAgentV2` runs
- **THEN** `WaitForHostRegistration` is NOT called

### Requirement: Transient API errors don't abort polling

When the DQL endpoint returns 5xx or a network error during polling, `WaitForHostRegistration` SHALL log the error via `logger.Debug` and continue polling until either the host appears or the timeout elapses. A persistent 4xx response (e.g. 401 Unauthorized) SHALL abort polling immediately with an error.

#### Scenario: Transient 503 during poll

- **GIVEN** the DQL endpoint returns 503 on the first poll
- **AND** returns the matching HOST record on the second poll
- **WHEN** `WaitForHostRegistration` runs
- **THEN** it logs the 503 via `logger.Debug`
- **AND** returns `(entityId, nil)` after the second poll succeeds

#### Scenario: Persistent 401 during poll

- **GIVEN** the DQL endpoint returns 401 on every poll
- **WHEN** `WaitForHostRegistration` runs
- **THEN** it returns an error indicating the platform token is invalid or lacks DQL execute scope
- **AND** does not continue polling past the first 401

### Requirement: User-facing polling output

`WaitForHostRegistration` SHALL output a single status line to stdout (at default verbosity, no `-v` required) when polling begins via `display.PrintStatusLine("register", "Waiting for agent registration... (polling)", display.ColorMuted)`. This line is output once at the start, not on every poll iteration, to avoid log spam during the 2-minute wait. Per-poll detail belongs in `logger.Debug`.

#### Scenario: Polling status line output once

- **GIVEN** the installer subprocess has exited 0
- **WHEN** `WaitForHostRegistration` begins its polling loop
- **THEN** stdout contains exactly one line via `display.PrintStatusLine` indicating polling status before the first poll
- **AND** subsequent polls do NOT add additional status lines to stdout

### Requirement: Polling debug logging

`WaitForHostRegistration` SHALL emit a `logger.Debug` line for each poll attempt with the hostname, attempt counter, and elapsed time. Transient errors during polling (per the prior requirement) SHALL also be logged at Debug. On success, a `logger.Verbose` line SHALL record the registered host ID. On timeout, `logger.Warn` SHALL emit a structured warning in addition to the user-facing stdout warning required elsewhere.

#### Scenario: Each poll attempt logged at Debug

- **GIVEN** `--debug` is enabled
- **WHEN** `WaitForHostRegistration` issues its third poll
- **THEN** stderr contains a Debug line with message `"polling smartscape for host"` and keys `hostname`, `attempt` (value `3`), `elapsed`

#### Scenario: Transient error logged at Debug

- **GIVEN** `--debug` is enabled
- **AND** a poll returns 503
- **WHEN** `WaitForHostRegistration` continues polling
- **THEN** stderr contains a Debug line with message `"smartscape poll error"` and key `status` (value `503`)

#### Scenario: Successful registration logged at Verbose

- **GIVEN** `-v` is enabled
- **AND** the host registers on the second poll
- **WHEN** `WaitForHostRegistration` returns
- **THEN** stderr contains a Verbose line with message `"host registered"` and keys `host_id`, `hostname`, `elapsed`

#### Scenario: Timeout produces both stdout warning and Warn log

- **GIVEN** the poll times out
- **WHEN** `WaitForHostRegistration` returns
- **THEN** stdout contains `⚠ Host registration verification timed out after 2 minutes. Check the tenant UI.`
- **AND** stderr (when `-v` or `--debug` is enabled) contains a Warn line with message `"host registration timed out"` and keys `timeout`, `hostname`

### Requirement: Hostname source and casing

`WaitForHostRegistration` SHALL use `os.Hostname()` as the local hostname for the DQL filter. The filter SHALL be case-insensitive (using `lower(name) == lower("<hostname>")` or equivalent) to accommodate Windows hosts where the OS-reported hostname casing may differ from the casing Dynatrace records in smartscape.

#### Scenario: Mixed-case hostname matches

- **GIVEN** `os.Hostname()` returns `"MY-HOST-01"`
- **AND** smartscape records the host as `"my-host-01"`
- **WHEN** `WaitForHostRegistration` polls
- **THEN** the DQL filter matches and the `entityId` is returned
