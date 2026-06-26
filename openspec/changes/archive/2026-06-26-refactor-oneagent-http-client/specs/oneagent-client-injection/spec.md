# OneAgent Client Injection

## CHANGED Requirements

### Requirement: OneAgent installer uses configured HTTP client

The `pkg/installer` package SHALL use the configured resty-based `*client.ClassicClient` from `pkg/client` instead of raw `http.DefaultClient` for all OneAgent HTTP operations. This ensures proper retry logic, timeout, user-agent headers, and verbosity logging for connectivity checks and installer downloads.

#### Scenario: OneAgent install command creates and injects configured client

- **GIVEN** a user runs `dtwiz install oneagent`
- **WHEN** the `installOneAgentCmd.RunE` handler executes
- **THEN** it calls `setupClient()` to create a `*client.Client`
- **AND** passes `client.Classic` to `installer.InstallOneAgent()`

#### Scenario: OneAgent setup command creates and injects configured client

- **GIVEN** a user runs `dtwiz setup` and selects OneAgent from recommendations
- **WHEN** the setup workflow switches to `MethodOneAgent`
- **THEN** it calls `setupClient()` to create a `*client.Client`
- **AND** passes `client.Classic` to `installer.InstallOneAgent()`

#### Scenario: Connectivity check uses configured client

- **GIVEN** `InstallOneAgent()` receives a `*client.ClassicClient`
- **AND** it is not a dry-run
- **WHEN** `checkOneAgentConnectivity()` is called
- **THEN** it uses `c.HTTP().R().Get("/api/v1/time")` (resty client)
- **AND** not raw `http.DefaultClient`

#### Scenario: Installer download uses configured client

- **GIVEN** `InstallOneAgent()` receives a `*client.ClassicClient`
- **WHEN** `downloadOneAgentInstaller()` is called
- **THEN** it uses `c.HTTP().R().SetDoNotParseResponse(true).Get(path)` for streaming downloads
- **AND** constructs request path relative to base URL (not full URL)

#### Scenario: API URL is obtained from client, not passed as parameter

- **GIVEN** `InstallOneAgent()` receives a `*client.ClassicClient`
- **WHEN** constructing installer arguments or printing dry-run output
- **THEN** it calls `c.BaseURL()` instead of using a separate `apiURL` parameter
- **AND** the signature is `InstallOneAgent(c *client.ClassicClient, dryRun, quiet bool, hostGroup string)`

### Requirement: Configured client provides retry and timeout

The `*client.ClassicClient` created by `setupClient()` SHALL have:

- 3 retry attempts on transient failures (429, 5xx errors)
- 1-second wait between retries, up to 10-second max
- 6-minute total timeout
- Proper User-Agent header (`dtwiz/{version}`)
- Verbose logging hooks when `--verbose` or `--debug` flags are set

#### Scenario: OneAgent HTTP requests respect retry policy

- **GIVEN** a transient 503 error from the Dynatrace API
- **WHEN** `checkOneAgentConnectivity()` makes a request via the resty client
- **THEN** the request is automatically retried up to 3 times
- **AND** the user does not need to implement retry logic in the installer

#### Scenario: OneAgent HTTP requests respect timeout

- **GIVEN** the Dynatrace API does not respond within 6 minutes
- **WHEN** `downloadOneAgentInstaller()` streams the binary
- **THEN** the request times out after 6 minutes
- **AND** returns an error to the caller

#### Scenario: Verbose logging shows HTTP details

- **GIVEN** a user runs `dtwiz install oneagent --verbose` with valid credentials
- **WHEN** `checkOneAgentConnectivity()` or `downloadOneAgentInstaller()` make HTTP requests
- **THEN** the resty client's hooks log request/response details to stderr
- **AND** the details include method, URL, status, and response time

### Requirement: No circular import between pkg/client and pkg/installer

The refactoring SHALL break the dependency of `pkg/client` on `pkg/installer.AuthHeader()`. Instead, the auth header logic is implemented locally in `pkg/client` so that `pkg/installer` can depend on `pkg/client` without creating a circular import.

#### Scenario: AuthHeader logic moved to pkg/client

- **GIVEN** the `authHeader(token string)` function in `pkg/client/client.go`
- **WHEN** `client.New()` creates a resty client
- **THEN** it calls the local `authHeader()` function
- **AND** no longer imports `pkg/installer` for this functionality

#### Scenario: Code compiles without circular import errors

- **GIVEN** the refactored codebase
- **WHEN** `make build` is run
- **THEN** compilation succeeds with no circular import errors
- **AND** all imports are acyclic: `cmd` → `pkg/installer` → `pkg/client`, `cmd` → `pkg/client`

### Requirement: Backward compatibility with command callers

Callers of `InstallOneAgent()` SHALL not need to know about or handle the configured client. The callers (`cmd/install.go`, `cmd/setup.go`) manage client creation and injection, while the installer focuses on the installation logic.

#### Scenario: Commands still handle credential validation and watch

- **GIVEN** a user runs `dtwiz install oneagent`
- **WHEN** the command handler executes
- **THEN** it continues to call `validateCredentials()` (using its own HTTP client)
- **AND** continues to call `installer.WatchIngest()` after installation succeeds
- **AND** the installer does not need to know about or call these functions

#### Scenario: Dry-run behavior unchanged

- **GIVEN** a user runs `dtwiz install oneagent --dry-run`
- **WHEN** `InstallOneAgent()` is called
- **THEN** it returns early with a preview of what would be installed
- **AND** no HTTP requests are made
- **AND** the configured client is created but not used in the dry-run path
