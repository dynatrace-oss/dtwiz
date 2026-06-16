# OneAgent Analyze and Probe

## ADDED Requirements

### Requirement: classifyEnvironment maps any GOOS/GOARCH to canonical Environment values

`classifyEnvironment(goos, goarch string) Environment` SHALL map the raw `runtime.GOOS` and `runtime.GOARCH` strings to the canonical OS and arch token sets. It SHALL never return an error. `"linux"` and `"windows"` are the supported OS values; `"darwin"` is classified explicitly so `validateEnvironment` can return a targeted macOS message. All other OS values (including `"aix"`, `"freebsd"`, etc.) map to `"other"`.

OS mapping:

| Input (`goos`) | `env.OS`    |
| -------------- | ----------- |
| `"linux"`      | `"linux"`   |
| `"windows"`    | `"windows"` |
| `"darwin"`     | `"darwin"`  |
| anything else  | `"other"`   |

Arch mapping:

| Input (`goarch`)   | `env.Arch` |
| ------------------ | ---------- |
| `"amd64"`, `"386"` | `"x86"`    |
| `"arm64"`, `"arm"` | `"arm"`    |
| anything else      | `"other"`  |

#### Scenario: Linux amd64 — canonical linux/x86

- **GIVEN** `goos == "linux"`, `goarch == "amd64"`
- **WHEN** `classifyEnvironment` is called
- **THEN** it returns `Environment{OS: "linux", Arch: "x86"}`

#### Scenario: Linux arm64 — canonical linux/arm

- **GIVEN** `goos == "linux"`, `goarch == "arm64"`
- **WHEN** `classifyEnvironment` is called
- **THEN** it returns `Environment{OS: "linux", Arch: "arm"}`

#### Scenario: Linux 386 — maps to x86

- **GIVEN** `goos == "linux"`, `goarch == "386"`
- **WHEN** `classifyEnvironment` is called
- **THEN** it returns `Environment{OS: "linux", Arch: "x86"}`

#### Scenario: Linux arm (32-bit) — maps to arm

- **GIVEN** `goos == "linux"`, `goarch == "arm"`
- **WHEN** `classifyEnvironment` is called
- **THEN** it returns `Environment{OS: "linux", Arch: "arm"}`

#### Scenario: Windows amd64 — canonical windows/x86

- **GIVEN** `goos == "windows"`, `goarch == "amd64"`
- **WHEN** `classifyEnvironment` is called
- **THEN** it returns `Environment{OS: "windows", Arch: "x86"}`

#### Scenario: macOS — classified as darwin

- **GIVEN** `goos == "darwin"`, `goarch == "amd64"`
- **WHEN** `classifyEnvironment` is called
- **THEN** it returns `Environment{OS: "darwin", Arch: "x86"}` (no error)

#### Scenario: Unknown OS (aix, freebsd, etc.) — classified as other

- **GIVEN** `goos == "aix"`, `goarch == "ppc64"`
- **WHEN** `classifyEnvironment` is called
- **THEN** it returns `Environment{OS: "other", Arch: "other"}` (no error)

#### Scenario: Unknown arch — classified as other

- **GIVEN** `goos == "linux"`, `goarch == "mips64"`
- **WHEN** `classifyEnvironment` is called
- **THEN** it returns `Environment{OS: "linux", Arch: "other"}` (no error)

### Requirement: detectRuntimeEnvironment returns Environment without error

`detectRuntimeEnvironment() Environment` SHALL call `classifyEnvironment(runtime.GOOS, runtime.GOARCH)` and return the result. It SHALL NOT return an error — the error-returning signature is replaced by a plain return.

#### Scenario: Called on the host platform

- **GIVEN** the process is running on any OS/arch
- **WHEN** `detectRuntimeEnvironment()` is called
- **THEN** it returns an `Environment` with non-empty `OS` and `Arch` fields, with no error

### Requirement: validateEnvironment fails fast on unsupported platforms

`validateEnvironment(env Environment) error` SHALL return a non-nil, actionable error for each unsupported platform. Supported platforms (`"linux"` and `"windows"` with arch `"x86"` or `"arm"`) SHALL return nil. OS is checked before arch.

#### Scenario: Linux/x86 is supported

- **GIVEN** `env == {OS: "linux", Arch: "x86"}`
- **WHEN** `validateEnvironment` is called
- **THEN** it returns nil

#### Scenario: Linux/arm is supported

- **GIVEN** `env == {OS: "linux", Arch: "arm"}`
- **WHEN** `validateEnvironment` is called
- **THEN** it returns nil

#### Scenario: Windows/x86 is supported

- **GIVEN** `env == {OS: "windows", Arch: "x86"}`
- **WHEN** `validateEnvironment` is called
- **THEN** it returns nil

#### Scenario: macOS rejection preserves existing message

- **GIVEN** `env.OS == "darwin"`
- **WHEN** `validateEnvironment` is called
- **THEN** it returns an error containing `"macOS"` and `"Docker or Linux"`

#### Scenario: Unknown OS rejection names the platform

- **GIVEN** `env.OS == "other"`
- **WHEN** `validateEnvironment` is called
- **THEN** it returns an error containing `runtime.GOOS` and `"Linux or Windows"`

#### Scenario: Unknown arch rejection names the architecture

- **GIVEN** `env.OS == "linux"`, `env.Arch == "other"`
- **WHEN** `validateEnvironment` is called
- **THEN** it returns an error containing `runtime.GOARCH` and `"x86 or ARM"`

#### Scenario: OS check precedes arch check

- **GIVEN** `env == {OS: "darwin", Arch: "other"}`
- **WHEN** `validateEnvironment` is called
- **THEN** the error message mentions `"macOS"` (not the arch)

### Requirement: InstallOneAgentV2 calls validateEnvironment before any network operation

In `InstallOneAgentV2`, `validateEnvironment(env)` SHALL be called immediately after `detectRuntimeEnvironment()` and before `ResolveAgentConfig`, `runPreflightChecks`, and `DownloadInstaller`. An unsupported platform returns the validation error before any HTTP request is made.

#### Scenario: macOS returns error before download

- **GIVEN** running on macOS
- **WHEN** `InstallOneAgentV2` is called
- **THEN** it returns an error containing `"macOS"`
- **AND** `DownloadInstaller` is NOT called
- **AND** no HTTP requests are made

### Requirement: runPreflightChecks validates system readiness before download

`runPreflightChecks(env Environment, opts InstallOptions) (preflightResult, error)` SHALL execute all system-readiness checks in order before any network operation. It SHALL be called in `InstallOneAgentV2` after `validateEnvironment` and `ResolveAgentConfig`, and before `DownloadInstaller`.

`preflightResult` carries:

```go
type preflightResult struct {
    IsUpdate bool
}
```

Checks run in this order:

1. Detect existing OneAgent installation via `oneAgentInstalled()`.
2. When an existing installation is found and `!opts.DryRun && !opts.Quiet`: prompt for update confirmation.
3. When `env.OS == "linux"` and the process is non-root: verify the `sudo` binary is available on PATH.

#### Scenario: Clean system, no existing install, running as root on Linux

- **GIVEN** `oneAgentInstalled()` returns `false`
- **AND** `needsSudoFn()` returns `false`
- **WHEN** `runPreflightChecks` runs
- **THEN** it returns `preflightResult{IsUpdate: false}` with nil error

#### Scenario: Clean system, non-root, sudo available

- **GIVEN** `oneAgentInstalled()` returns `false`
- **AND** `needsSudoFn()` returns `true`
- **AND** `sudoPathFn()` returns a valid path
- **WHEN** `runPreflightChecks` runs
- **THEN** it returns `preflightResult{IsUpdate: false}` with nil error

### Requirement: Pre-flight detects existing OneAgent installation

`runPreflightChecks` SHALL call `oneAgentInstalled()` and set `preflightResult.IsUpdate = true` when it returns true. `IsUpdate` SHALL be set regardless of `opts.DryRun` so the dry-run plan header can show `"Would update"` vs `"Would install"`.

#### Scenario: Existing installation detected — IsUpdate is true

- **GIVEN** `oneAgentInstalled()` returns `true`
- **WHEN** `runPreflightChecks` runs
- **THEN** the returned `preflightResult.IsUpdate` is `true`

#### Scenario: No installation detected — IsUpdate is false

- **GIVEN** `oneAgentInstalled()` returns `false`
- **WHEN** `runPreflightChecks` runs
- **THEN** the returned `preflightResult.IsUpdate` is `false`

### Requirement: Update confirmation prompt runs before download

When an existing installation is detected and the install is not a dry-run and not quiet, `runPreflightChecks` SHALL prompt the user for confirmation via `installer.ConfirmProceed`. On decline or EOF, it SHALL return `installer.ErrInstallCancelled` without making any network calls.

#### Scenario: Existing install, user confirms — proceeds

- **GIVEN** `oneAgentInstalled()` returns `true`
- **AND** `opts.DryRun == false`, `opts.Quiet == false`
- **AND** the user answers `Y`
- **WHEN** `runPreflightChecks` runs
- **THEN** it returns `preflightResult{IsUpdate: true}` with nil error

#### Scenario: Existing install, user declines — cancelled

- **GIVEN** `oneAgentInstalled()` returns `true`
- **AND** `opts.DryRun == false`, `opts.Quiet == false`
- **AND** the user answers `n`
- **WHEN** `runPreflightChecks` runs
- **THEN** it returns `installer.ErrInstallCancelled`
- **AND** no network calls are made

#### Scenario: Existing install, dry-run — no prompt, IsUpdate true

- **GIVEN** `oneAgentInstalled()` returns `true`
- **AND** `opts.DryRun == true`
- **WHEN** `runPreflightChecks` runs
- **THEN** no confirmation prompt is shown
- **AND** the returned `preflightResult.IsUpdate` is `true`

#### Scenario: Existing install, quiet mode — no prompt, proceeds

- **GIVEN** `oneAgentInstalled()` returns `true`
- **AND** `opts.Quiet == true`
- **WHEN** `runPreflightChecks` runs
- **THEN** no confirmation prompt is shown
- **AND** it returns nil error

### Requirement: Sudo availability check on Linux when non-root

When `env.OS == "linux"` and `needsSudoFn()` returns `true`, `runPreflightChecks` SHALL resolve the `sudo` binary via `sudoPathFn()`. If the binary is not found, it SHALL return an actionable error before any download.

#### Scenario: Non-root on Linux, sudo available — proceeds

- **GIVEN** `env.OS == "linux"`
- **AND** `needsSudoFn()` returns `true`
- **AND** `sudoPathFn()` returns a path successfully
- **WHEN** `runPreflightChecks` runs
- **THEN** it returns nil error

#### Scenario: Non-root on Linux, sudo missing — fast fail

- **GIVEN** `env.OS == "linux"`
- **AND** `needsSudoFn()` returns `true`
- **AND** `sudoPathFn()` returns an error
- **WHEN** `runPreflightChecks` runs
- **THEN** it returns an error containing `"sudo not found"` and `"root"`
- **AND** no network calls are made

#### Scenario: Root on Linux — sudo check skipped

- **GIVEN** `env.OS == "linux"`
- **AND** `needsSudoFn()` returns `false`
- **WHEN** `runPreflightChecks` runs
- **THEN** `sudoPathFn()` is NOT called
- **AND** it returns nil error

#### Scenario: Non-Linux platform — sudo check skipped

- **GIVEN** `env.OS == "windows"`
- **WHEN** `runPreflightChecks` runs
- **THEN** `sudoPathFn()` is NOT called
- **AND** it returns nil error

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

#### Scenario: Environment classification logs structured fields

- **GIVEN** `--debug` is enabled
- **WHEN** `detectRuntimeEnvironment()` returns
- **THEN** stderr contains a Debug line with message `"detected environment"` and structured keys `os`, `arch`

#### Scenario: Existing-agent detection logs result

- **GIVEN** `--debug` is enabled
- **WHEN** `runPreflightChecks` runs the existing-install check
- **THEN** stderr contains a Debug line with message `"preflight: oneagent detection"` and key `is_update`

#### Scenario: Sudo availability logged when check runs

- **GIVEN** `--debug` is enabled
- **AND** `env.OS == "linux"` and `needsSudoFn()` returns `true`
- **WHEN** `runPreflightChecks` confirms sudo is available
- **THEN** stderr contains a Debug line with message `"preflight: sudo available"`

#### Scenario: Agent config logged after resolution

- **GIVEN** `--debug` is enabled
- **WHEN** `ResolveAgentConfig` returns
- **THEN** stderr contains a Debug line with message `"resolved agent config"` and keys `monitoring-mode`, `override_set`

#### Scenario: Pre-flight logs suppressed without --debug

- **GIVEN** neither `--debug` nor `-v` is set
- **WHEN** any pre-flight stage runs
- **THEN** stderr contains no pre-flight Debug lines

### Requirement: Pre-flight ordering

`InstallOneAgentV2` SHALL execute stages in this order: (1) `detectRuntimeEnvironment`, (2) `validateEnvironment`, (3) `ResolveAgentConfig`, (4) `runPreflightChecks`, (5) `DownloadInstaller`. The install SHALL NOT issue any HTTP request, mint any token, or download any artifact until all four pre-download steps return nil.

#### Scenario: Failed validation skips network work

- **GIVEN** `validateEnvironment` returns an error (e.g. unsupported platform)
- **WHEN** `InstallOneAgentV2` runs
- **THEN** the function returns the validation error
- **AND** `runPreflightChecks`, `ResolveEndpoints`, `MintInstallerToken`, `DownloadInstaller` are not called

#### Scenario: Failed preflight skips network work

- **GIVEN** `runPreflightChecks` returns an error (e.g. sudo not found)
- **WHEN** `InstallOneAgentV2` runs
- **THEN** the function returns the preflight error
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

### Requirement: Connectivity failures are hard errors

When the probe is run as part of the normal install (not `--connectivity-check-only`), any unreachable endpoint SHALL terminate the install with a non-zero exit code. The install SHALL NOT proceed to downloading or executing the installer when connectivity fails.

The failure output format is:

1. `display.Header("Warning: connectivity check failed")`
2. `display.PrintStatusLine("action", "allow outbound TCP to the following addresses", display.ColorWarning)`
3. `display.PrintSectionDivider()`
4. One `display.PrintStatusLine("<host>:<port>", "✗ <friendly-error>", display.ColorError)` per unreachable endpoint
5. `display.PrintSectionDivider()`
6. `display.PrintStatusLine("tip", "if a proxy is required, set HTTP_PROXY / HTTPS_PROXY", display.ColorWarning)`

After the failure output, `InstallOneAgentV2` SHALL return a non-nil error of the form `"connectivity check failed: N/M endpoints unreachable"`, causing the command to exit non-zero.

The `<friendly-error>` term is produced by `friendlyDialError` (not the raw Go error string).

#### Scenario: Partial failure aborts install

- **GIVEN** `--skip-connectivity-check` is NOT set
- **AND** 2 of 5 resolved endpoints are unreachable
- **WHEN** `InstallOneAgentV2` reaches the connectivity-probe stage
- **THEN** stdout outputs the warning header (`"Warning: connectivity check failed"`), the action line (`"allow outbound TCP..."`), the 2 unreachable endpoints with friendly errors, and the proxy tip
- **AND** `InstallOneAgentV2` returns a non-nil error
- **AND** `MintInstallerToken`, `DownloadInstaller`, `BuildInstallCommand`, `ExecuteInstallCommand` are NOT called

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
