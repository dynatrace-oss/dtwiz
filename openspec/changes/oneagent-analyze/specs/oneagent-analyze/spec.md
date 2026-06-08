# OneAgent Analyze and Probe

## ADDED Requirements

### Requirement: Environment detection before any network work

`InstallOneAgentV2` SHALL detect OS and architecture before issuing any HTTP request. Detection SHALL produce an `Environment` struct with fields `OS` (`"windows"`, `"linux"`, `"aix"`, or `"other"`), `Arch` (`"x86"`, `"arm"`, or `"other"`), `Supported`, and `Reason`. Architecture mapping: `amd64`/`386` → `"x86"`, `arm64`/`arm` → `"arm"`.

#### Scenario: Linux amd64 host

- **GIVEN** the process runs on Linux with `runtime.GOARCH == "amd64"`
- **WHEN** `DetectEnvironment()` is called
- **THEN** it returns `{OS: "linux", Arch: "x86", Supported: true}`

#### Scenario: Linux arm64 host

- **GIVEN** the process runs on Linux with `runtime.GOARCH == "arm64"`
- **WHEN** `DetectEnvironment()` is called
- **THEN** it returns `{OS: "linux", Arch: "arm", Supported: true}`

#### Scenario: Windows amd64 host

- **GIVEN** the process runs on Windows with `runtime.GOARCH == "amd64"`
- **WHEN** `DetectEnvironment()` is called
- **THEN** it returns `{OS: "windows", Arch: "x86", Supported: true}`

#### Scenario: AIX explicitly unsupported

- **GIVEN** the process runs on `aix`
- **WHEN** `DetectEnvironment()` is called
- **THEN** it returns `{OS: "aix", Supported: false, Reason: "AIX is not supported"}`

#### Scenario: macOS direct install unsupported

- **GIVEN** the process runs on `darwin`
- **WHEN** `DetectEnvironment()` is called
- **THEN** it returns `{OS: "other", Supported: false, Reason: "OneAgent direct install is not supported on macOS; use Docker or Linux"}`

### Requirement: Existing-OneAgent pre-flight check

`InstallOneAgentV2` SHALL detect an existing OneAgent installation by reusing `pkg/analyzer/detect_oneagent_*.go`. When found AND `--force` is not set, the install SHALL exit with the message `"OneAgent already installed at {path}. Use --force to reinstall."`. When `--force` is set, the install SHALL proceed and log a debug entry recording the override.

#### Scenario: Existing agent blocks install

- **GIVEN** OneAgent is installed at `/opt/dynatrace/oneagent`
- **AND** `--force` is not passed
- **WHEN** `dtwiz install oneagent` runs
- **THEN** the command exits with `"OneAgent already installed at /opt/dynatrace/oneagent. Use --force to reinstall."`
- **AND** no HTTP requests are made

#### Scenario: --force overrides existing-agent check

- **GIVEN** OneAgent is installed at `/opt/dynatrace/oneagent`
- **AND** `--force` is passed
- **WHEN** `dtwiz install oneagent --force` runs
- **THEN** the preflight emits a `logger.Debug` entry noting the override
- **AND** the install proceeds to subsequent stages

#### Scenario: No existing agent

- **GIVEN** no OneAgent installation is detected
- **WHEN** the preflight runs
- **THEN** it returns nil and the install proceeds

### Requirement: Privilege check before any network work

`InstallOneAgentV2` SHALL verify that the process has the necessary privileges to install OneAgent before issuing any HTTP request. On Unix, the check SHALL verify root access, and on failure the install SHALL exit with `"This command requires root privileges. Please run with sudo."`. On Windows, the check SHALL verify the process token belongs to the BUILTIN\Administrators group, and on failure the install SHALL exit with `"This command requires administrator privileges. Please run as an administrator."`.

#### Scenario: Non-privileged Unix process

- **GIVEN** the process runs as a non-root user
- **AND** sudo is not configured for this user
- **WHEN** `dtwiz install oneagent` runs
- **THEN** the command exits with `"This command requires root privileges. Please run with sudo."`
- **AND** no HTTP requests are made

#### Scenario: Non-privileged Windows process

- **GIVEN** the process runs without administrator rights
- **WHEN** `dtwiz install oneagent` runs
- **THEN** - **THEN** the command exits with `"This command requires administrator privileges. Please run as an administrator."`

#### Scenario: Root Unix process

- **GIVEN** the process runs as root
- **WHEN** the preflight runs
- **THEN** it returns nil and the install proceeds

### Requirement: Agent configuration resolved before network work

`InstallOneAgentV2` SHALL resolve agent configuration before any HTTP request. The resolved `AgentConfig` SHALL contain `MonitoringMode` (default `"fullstack"`). The `--monitoring-mode` CLI flag, when provided, SHALL override the default `MonitoringMode` and be passed through to the installer as `--set-monitoring-mode=<value>` without dtwiz-side allow-list validation.

#### Scenario: Default configuration

- **GIVEN** no `--monitoring-mode` flag is passed
- **WHEN** `ResolveAgentConfig(opts)` is called
- **THEN** it returns `{MonitoringMode: "fullstack"}`

#### Scenario: Monitoring mode override

- **GIVEN** `--monitoring-mode=infra-only` is passed
- **WHEN** `ResolveAgentConfig(opts)` is called
- **THEN** it returns `{MonitoringMode: "infra-only"}`

#### Scenario: Empty override preserves default

- **GIVEN** `--monitoring-mode=""` (empty string)
- **WHEN** `ResolveAgentConfig(opts)` is called
- **THEN** it returns `{MonitoringMode: "fullstack"}`

### Requirement: Pre-flight debug logging

Each pre-flight stage SHALL emit a `logger.Debug` line at completion using the project's structured key-value convention. Logs are gated by `--debug` and are suppressed without it. No user-supplied or minted credential value SHALL appear in any pre-flight log line.

#### Scenario: Environment detection logs structured fields

- **GIVEN** `--debug` is enabled
- **WHEN** `DetectEnvironment()` returns
- **THEN** stderr contains a Debug line with message `"detected environment"` and structured keys `os`, `arch`, `supported`, `reason`

#### Scenario: Existing-agent detection logs result

- **GIVEN** `--debug` is enabled
- **WHEN** `CheckExistingOneAgent` runs and an agent is detected at `/opt/dynatrace/oneagent`
- **THEN** stderr contains a Debug line with message `"existing oneagent detected"` and keys `path`, `force_override`

#### Scenario: --force override logged explicitly

- **GIVEN** `--debug` is enabled
- **AND** `--force` is passed
- **AND** an existing agent is detected
- **WHEN** `CheckExistingOneAgent` proceeds past the detection
- **THEN** stderr contains a Debug line with `force_override == true`

#### Scenario: Privilege check logs outcome

- **GIVEN** `--debug` is enabled
- **WHEN** `CheckPrivilege` runs
- **THEN** stderr contains a Debug line with message `"privilege check"` and keys `privileged`, `os`

#### Scenario: Agent config logged after resolution

- **GIVEN** `--debug` is enabled
- **WHEN** `ResolveAgentConfig` returns
- **THEN** stderr contains a Debug line with message `"resolved agent config"` and keys `monitoring-mode`, `app_log_content_access`

#### Scenario: Pre-flight logs suppressed without --debug

- **GIVEN** neither `--debug` nor `-v` is set
- **WHEN** any pre-flight stage runs
- **THEN** stderr contains no pre-flight Debug lines

### Requirement: Pre-flight ordering

Pre-flight checks SHALL run in the order: (1) environment detection, (2) existing-OneAgent check, (3) privilege check, (4) agent configuration resolution. The install SHALL NOT issue any HTTP request, mint any token, or download any artifact until all four steps return nil.

#### Scenario: Failed privilege check skips network work

- **GIVEN** the privilege check fails
- **WHEN** `InstallOneAgentV2` runs
- **THEN** the function returns the privilege error
- **AND** `ResolveEndpoints`, `MintInstallerToken`, `DownloadInstaller` are not called

### Requirement: Dynamic endpoint resolution via tenant API

`InstallOneAgentV2` SHALL resolve OneAgent communication endpoints dynamically from the Dynatrace tenant API. Hardcoded endpoint lists or IP fallbacks SHALL NOT be used. The resolver SHALL call `GET /api/v1/deployment/installer/agent/connectioninfo/endpoints` via the configured `*client.ClassicClient` (which already carries auth and retry policy).

#### Scenario: Endpoints resolved from tenant

- **GIVEN** a tenant API that returns `endpoint-1.example.com:443;endpoint-2.example.com:443` in the response body
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** it returns `[]Endpoint{{Host: "endpoint-1.example.com", Port: 443}, {Host: "endpoint-2.example.com", Port: 443}}`

#### Scenario: Endpoint without explicit port defaults to 443

- **GIVEN** the tenant API returns `endpoint-1.example.com` (no port)
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** the returned `Endpoint` has `Port: 443`

#### Scenario: IP literal endpoints

- **GIVEN** the tenant API returns `54.88.45.104:443`
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** the returned `Endpoint` has `Host: "54.88.45.104"`, `Port: 443`

#### Scenario: Newline-separated response

- **GIVEN** the tenant API returns `"endpoint-1.example.com:443\nendpoint-2.example.com:443"` (newline-separated)
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** it returns two endpoints parsed correctly

#### Scenario: CRLF line endings

- **GIVEN** the tenant API returns entries separated by `\r\n`
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** each host value contains no stray `\r` character

#### Scenario: Full HTTPS URL entries

- **GIVEN** the tenant API returns `"https://endpoint-1.example.com:9999/communication"` (full URL form)
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** the scheme (`https://`) and path (`/communication`) are stripped and the returned `Endpoint` has `Host: "endpoint-1.example.com"`, `Port: 9999`

#### Scenario: Mixed separators

- **GIVEN** the tenant API returns `"ep1.example.com:443;ep2.example.com\nep3.example.com:9999"` (semicolons and newlines mixed)
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** it returns 3 endpoints; `ep2.example.com` has `Port: 443`

### Requirement: Tenant ID extraction from --environment

`InstallOneAgentV2` SHALL extract the tenant ID from the `--environment` URL using `pkg/installer/installer.ExtractTenantID`. Both Live (`<id>.live.dynatrace.com`), Apps (`<id>.apps.dynatrace.com`), and Managed (`/e/<id>`) URL shapes SHALL be supported. When extraction returns an empty string, the install SHALL fail with an actionable error rather than guessing.

#### Scenario: Live SaaS URL

- **GIVEN** `--environment https://abc12345.live.dynatrace.com`
- **WHEN** `ExtractTenantID(envURL)` is called
- **THEN** it returns `"abc12345"`

#### Scenario: Apps SaaS URL

- **GIVEN** `--environment https://abc12345.apps.dynatrace.com`
- **WHEN** `ExtractTenantID(envURL)` is called
- **THEN** it returns `"abc12345"`

#### Scenario: Managed URL with /e/ID

- **GIVEN** `--environment https://my-managed.example.com/e/abc12345`
- **WHEN** `ExtractTenantID(envURL)` is called
- **THEN** it returns `"abc12345"`

#### Scenario: Unrecognized URL shape

- **GIVEN** `--environment https://something.weird`
- **WHEN** `ExtractTenantID(envURL)` returns an empty string
- **THEN** `InstallOneAgentV2` fails with an error that includes the URL and guidance to verify the `--environment` flag

### Requirement: Endpoint resolution debug logging

`ResolveEndpoints` SHALL emit structured `logger.Debug` lines for the API call, the resolved endpoint count, and each individual endpoint, plus a `logger.Verbose` line summarising the count for operators running with `-v`. The full `--environment` URL SHALL NOT be logged if it contains embedded credentials; only the extracted tenant ID and the request path may be logged at Debug.

#### Scenario: Tenant ID logged after extraction

- **GIVEN** `--debug` is enabled
- **WHEN** `ExtractTenantID(envURL)` returns `"abc12345"`
- **THEN** stderr contains a Debug line with message `"extracted tenant id"` and key `tenant_id` (value `"abc12345"`)

#### Scenario: Endpoint API request logged

- **GIVEN** `--debug` is enabled
- **WHEN** `ResolveEndpoints` issues `GET /api/v1/deployment/installer/agent/connectioninfo/endpoints`
- **THEN** stderr contains a Debug line with message `"resolving tenant endpoints"` and key `url`

#### Scenario: Each resolved endpoint logged at Debug

- **GIVEN** `--debug` is enabled
- **AND** the API returns 3 endpoints
- **WHEN** `ResolveEndpoints` parses the response
- **THEN** stderr contains 3 Debug lines with message `"tenant endpoint"` and keys `host`, `port`

#### Scenario: Endpoint summary visible with -v

- **GIVEN** `-v` is enabled (but not `--debug`)
- **WHEN** `ResolveEndpoints` returns 3 endpoints
- **THEN** stderr contains a single Verbose line with message `"resolved tenant endpoints"` and key `count` (value `3`)
- **AND** per-endpoint Debug lines are NOT printed

### Requirement: Endpoint API error handling

`ResolveEndpoints` SHALL wrap non-2xx responses with both the request URL and HTTP status. An empty endpoint set in a 2xx response SHALL be treated as a server bug and returned as an error.

#### Scenario: 401 Unauthorized

- **GIVEN** the tenant API returns 401 with body `"Invalid token"`
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** it returns an error containing the request URL, HTTP status 401, and the response body

#### Scenario: 5xx server error

- **GIVEN** the tenant API returns 503
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** it returns a wrapped error indicating the server-side failure (after the resty client's retry policy is exhausted)

#### Scenario: Empty response body

- **GIVEN** the tenant API returns 200 with an empty body
- **WHEN** `ResolveEndpoints(c)` is called
- **THEN** it returns an error stating "tenant returned no endpoints"

### Requirement: Parallel TCP probe of resolved endpoints

`InstallOneAgentV2` SHALL optionally probe each resolved endpoint via parallel `net.DialTimeout("tcp", host:port, perEndpointTimeout)` calls. The probe SHALL be runnable independently of the install (`--connectivity-check-only`) or as a pre-install diagnostic. Per-endpoint results SHALL include reachability, observed latency, and any error.

#### Scenario: All endpoints reachable

- **GIVEN** 3 resolved endpoints all accepting TCP connections within 10s
- **WHEN** `CheckAllEndpoints(endpoints, 10*time.Second)` runs
- **THEN** the returned `ConnectivityReport` has `AllPassed == true`, `FailedCount == 0`
- **AND** each `ConnectivityResult` has `Reachable: true` and a positive `Latency`

#### Scenario: Endpoint times out

- **GIVEN** one endpoint never accepts the connection within the per-endpoint timeout
- **WHEN** `CheckAllEndpoints` runs
- **THEN** that endpoint's `ConnectivityResult` has `Reachable: false`, `Error != nil`, and `Latency` approximately equal to the timeout
- **AND** `ConnectivityReport.FailedCount == 1`

#### Scenario: Probes run in parallel

- **GIVEN** 5 endpoints each with a 10s timeout
- **WHEN** `CheckAllEndpoints` runs
- **THEN** total wall-clock time is bounded by the slowest single probe, not the sum of all timeouts

### Requirement: User-facing probe output format — --connectivity-check-only

Under `--connectivity-check-only`, `InstallOneAgentV2` SHALL print `display.Header("Checking network connectivity...")` BEFORE calling `CheckAllEndpoints`, so the user sees the section header immediately at the start of the dial timeout window rather than after it. After `CheckAllEndpoints` returns, one `display.PrintStatusLine` per endpoint is printed: `display.ColorOK` with `"✓ <latency>"` for reachable endpoints, `display.ColorError` with `"✗ <friendly-error>"` for unreachable ones. Failure cause is a short human-readable phrase produced by `friendlyDialError` (see below), not the raw Go error string.

#### Scenario: Header appears before probe

- **GIVEN** `--connectivity-check-only` is passed
- **AND** each endpoint probe takes ~1s to complete
- **WHEN** `InstallOneAgentV2` reaches the connectivity stage
- **THEN** stdout contains the `display.Header` output before any per-endpoint status lines appear

#### Scenario: All endpoints reachable under --connectivity-check-only

- **GIVEN** 2 endpoints both reachable in 23ms and 31ms
- **WHEN** the probe completes
- **THEN** stdout contains the header via `display.Header`
- **AND** stdout contains status lines for each endpoint via `display.PrintStatusLine` with `display.ColorOK` showing latency (e.g. `"✓ 23ms"`)

#### Scenario: Mixed reachable and unreachable under --connectivity-check-only

- **GIVEN** 4 endpoints: 2 reachable, 1 times out, 1 connection-refused
- **WHEN** the probe completes
- **THEN** stdout contains 4 status lines via `display.PrintStatusLine`
- **AND** unreachable endpoints use `display.ColorError` with a friendly error phrase (e.g. `"✗ timed out"`, `"✗ connection refused"`)

### Requirement: In-progress indicator during normal install probe

When the probe runs as part of the normal install path (not `--connectivity-check-only`), `InstallOneAgentV2` SHALL call `display.PrintPending("connectivity", "checking endpoints...")` before `CheckAllEndpoints` and `display.ClearPending()` immediately after it returns. This gives TTY users a transient status line during the dial timeout window. The pending output is suppressed in non-TTY environments (CI, pipes) by the existing `display.PrintPending` implementation.

#### Scenario: Pending indicator on TTY

- **GIVEN** the process stdout is a TTY
- **AND** the normal install path runs
- **WHEN** `CheckAllEndpoints` is executing
- **THEN** stderr shows `connectivity:  checking endpoints...` transiently
- **AND** the line is erased by `display.ClearPending()` before any subsequent output appears

### Requirement: Connectivity failures are warnings, not errors

When the probe is run as part of the normal install (not `--connectivity-check-only`), unreachable endpoints SHALL produce a warning section. The install SHALL proceed regardless.

The warning section format is:

1. `display.Header("Warning: connectivity check failed")`
2. `display.PrintStatusLine("action", "allow outbound TCP to the following addresses", display.ColorWarning)`
3. `display.PrintSectionDivider()`
4. One `display.PrintStatusLine("<host>:<port>", "✗ <friendly-error>", display.ColorError)` per unreachable endpoint
5. `display.PrintSectionDivider()`
6. `display.PrintStatusLine("tip", "if a proxy is required, set HTTP_PROXY / HTTPS_PROXY", display.ColorWarning)`

The `<friendly-error>` term is produced by `friendlyDialError` (not the raw Go error string).

#### Scenario: Partial failure proceeds with install

- **GIVEN** `--skip-connectivity-check` is NOT set
- **AND** 2 of 5 resolved endpoints are unreachable
- **WHEN** `InstallOneAgentV2` reaches the connectivity-probe stage
- **THEN** stdout outputs the warning header (`"Warning: connectivity check failed"`), the action line (`"allow outbound TCP..."`), the 2 unreachable endpoints with friendly errors, and the proxy tip
- **AND** the install proceeds to `MintInstallerToken`

### Requirement: Friendly error messages in connectivity output

`InstallOneAgentV2` SHALL translate raw `net.DialTimeout` error strings to short human-readable phrases via a `friendlyDialError` helper before displaying them to the user. The mapping:

| Raw error contains | Displayed as |
|---|---|
| `"i/o timeout"`, `"timed out"`, `"deadline exceeded"` | `"timed out"` |
| `"connection refused"` | `"connection refused"` |
| `"no route to host"` | `"no route to host"` |
| `"network is unreachable"` | `"network unreachable"` |
| `"connection reset"` | `"connection reset"` |
| anything else (non-empty) | `"unreachable"` |
| empty string | `""` |

#### Scenario: Timeout error is friendly

- **GIVEN** a probe that times out (Go error contains `"i/o timeout"`)
- **WHEN** the result is displayed
- **THEN** the user sees `"timed out"`, not `"dial tcp 1.2.3.4:443: i/o timeout"`

#### Scenario: Connection refused is friendly

- **GIVEN** a probe that gets a reset (Go error contains `"connection refused"`)
- **WHEN** the result is displayed
- **THEN** the user sees `"connection refused"`

### Requirement: Probe debug logging

`CheckAllEndpoints` SHALL emit one `logger.Debug` line per endpoint with the host, port, reachability, latency, and any error, and a single `logger.Verbose` summary line with totals.

#### Scenario: Per-endpoint probe result logged at Debug

- **GIVEN** `--debug` is enabled
- **AND** 3 endpoints are probed (2 reachable, 1 timeout)
- **WHEN** `CheckAllEndpoints` returns
- **THEN** stderr contains 3 Debug lines with message `"endpoint probe result"` and keys `host`, `port`, `reachable`, `latency`, `error`

#### Scenario: Probe summary logged at Verbose

- **GIVEN** `-v` is enabled
- **AND** 5 endpoints are probed with 2 failures
- **WHEN** `CheckAllEndpoints` returns
- **THEN** stderr contains a Verbose line with message `"connectivity probe complete"` and keys `total` (value `5`), `failed` (value `2`)

### Requirement: --skip-connectivity-check disables the probe

When `--skip-connectivity-check` is set, `CheckAllEndpoints` SHALL NOT be called. The install SHALL proceed from endpoint resolution directly to token minting.

#### Scenario: Skip flag set

- **GIVEN** `--skip-connectivity-check` is passed
- **WHEN** `InstallOneAgentV2` runs
- **THEN** `CheckAllEndpoints` is not called
- **AND** no TCP connections are opened by dtwiz before `MintInstallerToken`

### Requirement: --connectivity-check-only runs probe and exits

When `--connectivity-check-only` is set, `InstallOneAgentV2` SHALL run preflights, resolve endpoints, run the probe, print the report, and exit with code 0 — without minting a token, downloading the installer, or executing the install command.

#### Scenario: Probe-only run

- **GIVEN** `--connectivity-check-only` is passed
- **WHEN** `dtwiz install oneagent --connectivity-check-only` runs
- **THEN** preflights run and resolve endpoints
- **AND** the probe runs and the report is printed
- **AND** `MintInstallerToken`, `DownloadInstaller`, `BuildInstallCommand`, `ExecuteInstallCommand`, `WaitForHostRegistration` are NOT called
- **AND** the command exits with code 0

### Requirement: --print-endpoints prints resolved endpoints

When `--print-endpoints` is set, `InstallOneAgentV2` SHALL run preflights, resolve endpoints, print each endpoint on its own line in `host:port` format, and exit with code 0 — before minting any token.

#### Scenario: Print-endpoints run

- **GIVEN** `--print-endpoints` is passed
- **AND** the tenant returns two endpoints
- **WHEN** `dtwiz install oneagent --print-endpoints` runs
- **THEN** stdout contains exactly two lines, one per endpoint in `host:port` format
- **AND** `MintInstallerToken` is NOT called
- **AND** the command exits with code 0
