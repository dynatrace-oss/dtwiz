# OneAgent Analyze Tasks

**Prerequisite:** `oneagent-init` (Task 1 — Feature Flag + Scaffolding) must be completed first. That change registers the `ONEAGENT_POC` feature flag, creates the v2 source/test files with stubs, and wires the feature-flag branching in `cmd/install.go`.

---

## 0. Investigate and Confirm Assumptions

Before implementing, review the design and spec documents to understand the requirements and constraints.

**Files:** `design.md`, `spec.md`

- [ ] 0.1 Read `design.md` and `spec.md` to understand environment detection and preflight check requirements
- [ ] 0.2 Identify and document any unclear assumptions about OS/arch support or detection logic
- [ ] 0.3 Review existing OneAgent detection code in `pkg/installer/oneagent.go` to understand patterns
- [ ] 0.4 Confirm error messages and behavior align with the specification

---

## 2. OS/Arch Detection and Pre-flight Checks

Implement OS/arch classification, platform validation, and system-readiness pre-flight before any network work. All checks run automatically as part of `InstallOneAgentV2` — no new CLI flags.

**Files:** `pkg/installer/oneagent/oneagent.go` (extend), `pkg/installer/oneagent/preflight.go` (new)

### Part A — OS/Arch classification and validation

- [x] 2.1 Add `classifyEnvironment(goos, goarch string) Environment` — pure function, no error return. Switch on `goarch` (`amd64`/`386`→`"x86"`, `arm64`/`arm`→`"arm"`, default→`"other"`), then switch on `goos` (`"linux"`, `"windows"`, `"darwin"`, default→`"other"`). AIX and all other unrecognised OS values map to `"other"`.
- [x] 2.2 Replace `detectRuntimeEnvironment() (Environment, error)` with `detectRuntimeEnvironment() Environment` that calls `classifyEnvironment(runtime.GOOS, runtime.GOARCH)` and returns the result directly.
- [x] 2.3 Update the `Environment` type to carry only `OS` and `Arch` fields. OS canonical values: `"linux"`, `"windows"`, `"darwin"`, `"other"`. Arch canonical values: `"x86"`, `"arm"`, `"other"`. Remove `Supported` and `Reason` — support decisions live in `validateEnvironment`.
- [x] 2.4 Add `validateEnvironment(env Environment) error`: darwin → macOS rejection (preserves existing message), other OS → generic message naming `runtime.GOOS`, supported OS + other arch → message naming `runtime.GOARCH`. OS is checked before arch. Supported combinations (`linux`/`windows` × `x86`/`arm`) return nil.
- [x] 2.4a In `InstallOneAgentV2`: replace `env, err := detectRuntimeEnvironment()` with `env := detectRuntimeEnvironment()` and add `if err := validateEnvironment(env); err != nil { return err }` immediately after, before `ResolveAgentConfig` and `runPreflightChecks`.
- [x] 2.4b Emit `logger.Debug("detected environment", "os", env.OS, "arch", env.Arch)` after `detectRuntimeEnvironment` returns.
- [x] 2.5 Unit tests — `TestClassifyEnvironment` table test covering: linux/amd64, linux/arm64, linux/386, linux/arm, windows/amd64, darwin/amd64, aix/ppc64 (→ other/other), freebsd/amd64 (→ other/x86), linux/mips64 (→ linux/other).
- [x] 2.6 Unit tests — `TestValidateEnvironment` covering: linux/x86 (nil), linux/arm (nil), windows/x86 (nil), darwin/x86 → macOS message, other/x86 → names `runtime.GOOS`, linux/other → names `runtime.GOARCH`, darwin/other → macOS message (OS checked before arch).
- [x] 2.7 Update `TestDetectRuntimeEnvironment`: remove darwin error assertion; verify it returns a non-empty `Environment` for any platform.

### Part B — Pre-flight checks

- [x] 2.8 Define `preflightResult struct { IsUpdate bool }` in `pkg/installer/oneagent/preflight.go`.
- [x] 2.9 Implement `runPreflightChecks(env Environment, opts InstallOptions) (preflightResult, error)`:
  - Call `oneAgentInstalled()`, store result in `result.IsUpdate`, emit `logger.Debug("preflight: oneagent detection", "is_update", result.IsUpdate)`.
  - When `result.IsUpdate && !opts.DryRun && !opts.Quiet`: call `installer.ConfirmProceed`; on decline/error return `installer.ErrInstallCancelled`.
  - When `env.OS == "linux" && needsSudoFn()`: call `sudoPathFn()`; on error return `fmt.Errorf("sudo not found: install sudo or run dtwiz as root")`; on success emit `logger.Debug("preflight: sudo available")`.
  - Return `result, nil`.
- [x] 2.10 Replace inline `updating := oneAgentInstalled()` + confirmation block in `InstallOneAgentV2` with `preflight, err := runPreflightChecks(env, opts); if err != nil { return err }`. Pass `preflight.IsUpdate` to `printDryRun`.
- [x] 2.11 Unit tests in `pkg/installer/oneagent/preflight_test.go`:
  - `TestRunPreflightChecks_NoInstall_NoSudo` — returns `{IsUpdate: false}`, nil error.
  - `TestRunPreflightChecks_NoInstall_SudoAvailable` — non-root, sudo found, returns nil error.
  - `TestRunPreflightChecks_NoInstall_SudoMissing` — non-root, sudo missing, returns error containing `"sudo not found"`.
  - `TestRunPreflightChecks_ExistingInstall_Confirmed` — user confirms, returns `{IsUpdate: true}`, nil error.
  - `TestRunPreflightChecks_ExistingInstall_Declined` — user declines, returns `ErrInstallCancelled`.
  - `TestRunPreflightChecks_ExistingInstall_DryRun` — no prompt, returns `{IsUpdate: true}`, nil error.
  - `TestRunPreflightChecks_ExistingInstall_Quiet` — no prompt, returns nil error.
  - `TestRunPreflightChecks_Windows_SkipsSudo` — `sudoPathFn` not called, returns nil error.
- [x] 2.12 Update existing `InstallOneAgentV2` integration tests (`TestInstallOneAgentV2_DryRun_*`, `TestInstallOneAgentV2_Update*`) to inject `withNeedsSudo(t, false)` where the sudo pre-flight would otherwise interfere.

---

## 2.5 Agent Configuration

Resolve the agent's runtime configuration — monitoring mode and app-log content access — before the endpoint/token/download stages. This task is independent of Task 2's environment + preflight outputs; its result (`AgentConfig`) is consumed by Task 6 when building the OS-specific install command. Pulled out of Task 2 because it owns a distinct concern (config resolution) and a distinct CLI flag (`--monitoring-mode`).

**Goals:**

- **Default path** — when no flag is passed, `ResolveAgentConfig` returns `{MonitoringMode: "fullstack"}` exactly. This is the zero-config default for OneAgent installs per `AGENTS.md`.
- **Override path** — when `--monitoring-mode <value>` is passed (or `opts.MonitoringMode != ""`), the field is overridden. No allow-list — the value is passed through verbatim to the installer's `--set-monitoring-mode=<value>` flag in Task 6.

**Files:** `pkg/installer/oneagent/` (extend — scaffolded in Task 1), `cmd/install.go` (modify — wire `--monitoring-mode`)

- [x] 2.5.1 Define the `AgentConfig` struct in `pkg/installer/oneagent/oneagent.go`:

  ```go
  type AgentConfig struct {
      MonitoringMode string // installer flag: --set-monitoring-mode
  }
  ```

- [x] 2.5.2 Implement `DefaultAgentConfig() AgentConfig` returning `{MonitoringMode: "fullstack"}` — this is the canonical zero-config default and the only construction path callers should use
- [x] 2.5.3 Implement `ResolveAgentConfig(opts InstallOptions) AgentConfig`:
  - Start with `DefaultAgentConfig()`
  - Always assign `cfg.MonitoringMode = opts.MonitoringMode` (no validation, pass-through; CLI default guarantees it is never empty)
  - Return the resolved config
- [x] 2.5.4 `MonitoringMode string` already present in `InstallOptions` (no change needed)
- [x] 2.5.5 `--monitoring-mode` flag already wired on `installOneAgentCmd` with default `"fullstack"` (no change needed)
- [x] 2.5.6 Emit `logger.Debug("resolved agent config", "monitoring-mode", cfg.MonitoringMode, "override_set", opts.MonitoringMode != "fullstack")` once the config is resolved
- [x] 2.5.7 Unit test — **default path**: `cfg := ResolveAgentConfig(InstallOptions{MonitoringMode: "fullstack"})` returns `{"fullstack"}`
- [x] 2.5.8 Unit test — **override path**: `cfg := ResolveAgentConfig(InstallOptions{MonitoringMode: "infra-only"})` returns `{"infra-only"}`
- [x] 2.5.9 Unit test — **default constant matches flag default**: `DefaultAgentConfig().MonitoringMode == "fullstack"` (documents that the struct default and the CLI flag default are in sync)
- [x] 2.5.10 Unit test — **debug log captures both default and override paths**: with `--debug`, run the function twice (default + override) and assert two separate `"resolved agent config"` Debug lines appear with the correct `monitoring-mode` and `override_set` field values

---

## 3. Resolve Tenant Endpoints and Connectivity Probe (Dynamic)

Resolve agent communication endpoints dynamically from the tenant API. Drop the hardcoded fallback used by the old flow. After resolving, probe each endpoint for TCP reachability. Connectivity failures are warnings, not blockers.

**Files:** `pkg/installer/oneagent/` (extend — scaffolded in Task 1), `pkg/installer/installer.go` (reuse `ExtractTenantID`)

### Part A — Endpoint resolution

- [x] 3.1 Define `Endpoint` struct (`Host string`, `Port int`)
- [x] 3.2 Implement `ResolveEndpoints(c *client.ClassicClient) ([]Endpoint, error)` calling `GET /api/v1/deployment/installer/agent/connectioninfo/endpoints` via the resty client
- [x] 3.3 Parse the response into `[]Endpoint`; default port `443` when omitted; tolerate both hostnames and IP literals
- [x] 3.3a Parse the response body using `strings.FieldsFunc` splitting on `;`, `\n`, and `\r` so that semicolon-separated, newline-separated, and CRLF-terminated responses are all handled identically
- [x] 3.3b Strip HTTPS scheme (`https://`) and path component (`/communication` etc.) from entries before parsing, so that full URL-format responses (`https://host:port/path`) are handled in addition to bare `host:port`
- [x] 3.3c Unit tests for: newline-separated response, CRLF-separated response (verify no stray `\r` in host), full HTTPS URL entries, and mixed separator formats
- [x] 3.4 Return wrapped errors that include the URL and HTTP status when the API returns 4xx/5xx
- [x] 3.5 Return an error when the response is empty (treat as a server bug)
- [x] 3.6 Unit tests with `httptest.Server`: happy path (multiple endpoints), empty response, 401 with body, 5xx
- [x] 3.7 Confirm `installer.ExtractTenantID()` correctly handles `live.dynatrace.com`, `apps.dynatrace.com`, and `/e/<tenantId>` Managed URLs; add cases if missing
- [x] 3.7a Fix: `ExtractTenantID` was not handling Managed URLs (`https://host/e/<id>`); added path-based extraction: when `url.Parse` succeeds and the path matches `/e/<id>`, return `<id>` from the path (not the first DNS label of the host). Added test case to `pkg/installer/installer_test.go`.
- [x] 3.8 Emit `logger.Debug("extracted tenant id", "tenant_id", id)` after extraction; do NOT log the full `--environment` URL at Debug if it may contain embedded credentials
- [x] 3.9 Emit `logger.Debug("resolving tenant endpoints", "url", reqURL)` before the API call
- [x] 3.10 Emit one `logger.Debug("tenant endpoint", "host", e.Host, "port", e.Port)` per parsed endpoint
- [x] 3.11 Emit `logger.Verbose("resolved tenant endpoints", "count", len(endpoints))` as the operator-visible milestone
- [x] 3.12 Unit test (with captured stderr): `--debug` shows per-endpoint lines; `-v` only shows the summary line; default shows nothing

### Part B — Connectivity probe

- [x] 3.13 Define `ConnectivityResult` struct (`Endpoint`, `Reachable bool`, `Latency time.Duration`, `Error string`) and `ConnectivityReport` struct (`Results []ConnectivityResult`, `AllPassed bool`, `FailedCount int`) for use by the diagnostic exit modes below
- [x] 3.14 Implement `CheckAllEndpoints(endpoints []Endpoint, timeout time.Duration) ConnectivityReport` using `net.DialTimeout("tcp", "host:port", timeout)` per endpoint; run probes concurrently and collect results with timing and error details
- [x] 3.15 Use a default probe timeout of `5s` per endpoint; do not make it configurable at this stage
- [x] 3.16 Wire `opts.ConnectivityCheckOnly` (from `InstallOptions`): if `true`, call `CheckAllEndpoints`, print one `display.PrintStatusLine` per endpoint (`host:port` as label, `✓ <latency>` in green or `✗ <friendly-error>` in red), then return `nil` without proceeding to token minting, download, or install
- [x] 3.16a Print `display.Header("Checking network connectivity...")` BEFORE calling `CheckAllEndpoints` when `ConnectivityCheckOnly` is true, so the header appears at the start of the dial window rather than after it
- [x] 3.17 Wire `opts.PrintEndpoints` (from `InstallOptions`): if `true`, print resolved endpoints one per line in `host:port` format after `ResolveEndpoints`, then return `nil` without probing or installing
- [x] 3.18 When `len(report.Results with Reachable==false) > 0` in the normal install path, print a WARNING block and continue (non-blocking)
- [x] 3.18a In the normal install path, call `display.PrintPending("connectivity", "checking endpoints...")` before `CheckAllEndpoints` and `display.ClearPending()` after it returns, giving TTY users a transient in-progress indicator
- [x] 3.18b Implement `friendlyDialError(errStr string) string` that maps raw `net.DialTimeout` error strings to short human-readable phrases: `"i/o timeout"` / `"deadline exceeded"` / `"timed out"` → `"timed out"`, `"connection refused"` → `"connection refused"`, `"no route to host"` → `"no route to host"`, `"network is unreachable"` → `"network unreachable"`, `"connection reset"` → `"connection reset"`, anything else → `"unreachable"`
- [x] 3.18c Update `printConnectivityWarning` format: header says `"Warning: connectivity check failed"`; lead with `display.PrintStatusLine("action", "allow outbound TCP to the following addresses", display.ColorWarning)`; frame the address list between two `display.PrintSectionDivider()` calls; use `friendlyDialError` for error messages; update proxy tip to `"if a proxy is required, set HTTP_PROXY / HTTPS_PROXY"`
- [x] 3.18d Unit test for `friendlyDialError`: all mapping cases including the unknown-error fallback and empty-string case
- [x] 3.19 When all endpoints are reachable in the normal install path, print nothing (do not add a "all endpoints reachable" success line at default verbosity)
- [x] 3.20 Wire `opts.SkipConnectivityCheck` (from `InstallOptions`): when `true`, skip `CheckAllEndpoints` entirely; emit `logger.Debug("skipping connectivity probe", "reason", "--skip-connectivity-check")`
- [x] 3.21 Emit one `logger.Debug("endpoint probe result", "host", r.Endpoint.Host, "port", r.Endpoint.Port, "reachable", r.Reachable, "latency_ms", r.Latency.Milliseconds(), "error", r.Error)` per result when probing
- [x] 3.22 Emit `logger.Verbose("connectivity probe complete", "total", len(report.Results), "failed", report.FailedCount)` after all probes finish (only when the probe ran in the normal path)
- [x] 3.23 Unit tests:
  - Normal path: all reachable (no output), some blocked (warning printed, install continues), all blocked, `SkipConnectivityCheck == true` (no probe, no output)
  - `--connectivity-check-only`: mixed reachable/blocked endpoints print the expected table, no token mint
  - Use an in-process TCP listener (`net.Listen`) to control reachability without real network access
