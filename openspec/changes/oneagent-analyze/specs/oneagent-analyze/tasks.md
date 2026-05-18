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

Detect OS/arch and run the existing-OneAgent and privilege pre-flights before any network work. Fail fast with clear, actionable errors. Agent configuration (default + `--monitoring-mode` override) is a separate concern owned by Task 2.5.

**Files:** `pkg/installer/oneagent.go` (extend), `pkg/installer/oneagent_v2.go` (extend — created in Task 1), `pkg/installer/oneagent_v2_test.go` (extend — created in Task 1), `pkg/analyzer/detect_oneagent_unix.go` / `_windows.go` (reuse)

Task 1 has already created `oneagent_v2.go` with the `InstallOptions` struct and the stub `InstallOneAgentV2` entry function. This task fills in the stub with the OS-detection + preflight stages.

### Part A — OS/Arch detection

- [ ] 2.1 Define `Environment` struct (`OS`, `Arch`, `Supported`, `Reason`) in `oneagent_v2.go`
- [ ] 2.2 Implement `DetectEnvironment() Environment` mapping `runtime.GOOS`/`runtime.GOARCH` → `OS` ("windows"/"linux"/"aix"/"other") and `Arch` ("x86" for `amd64`/`386`, "arm" for `arm64`/`arm`, "other" otherwise)
- [ ] 2.3 Mark AIX as `Supported: false` with `Reason: "AIX is not supported"`; preserve the existing macOS rejection message
- [ ] 2.4 Unit tests covering Linux/amd64, Linux/arm64, Windows/amd64, AIX rejection, and unknown OS
- [ ] 2.4a After `DetectEnvironment`, emit `logger.Debug("detected environment", "os", env.OS, "arch", env.Arch, "supported", env.Supported, "reason", env.Reason)`

### Part B — Pre-flight checks

- [ ] 2.5 Implement `CheckExistingOneAgent(force bool) error` that calls `pkg/analyzer/detect_oneagent_*.go` and returns `"OneAgent already installed at {path}. Use --force to reinstall."` when found and `force` is false
- [ ] 2.6 When `force` is true and an agent is detected, log via `logger.Debug` and return nil
- [ ] 2.7a Implement `CheckPrivilege() error` in `preflight_unix.go` (build tag `//go:build !windows`) that checks `os.Getuid() == 0` and returns `"This command requires root privileges. Please run with sudo."` when the process is not root
- [ ] 2.7b Implement `CheckPrivilege() error` in `preflight_windows.go` (build tag `//go:build windows`) using process-token SID membership to detect admin elevation and returns `"This command requires administrator privileges. Please run as an administrator."` when not elevated
- [ ] 2.8 Add a `sudo_windows.go` admin check (process-token SID membership) if not already present; reuse `needsSudo()` semantics on Unix
- [ ] 2.9 Unit tests: existing-agent path (with/without `--force`), no existing agent, missing privilege
- [ ] 2.9a Emit `logger.Debug("existing oneagent detected", "path", path, "force_override", force)` when an agent is detected (both with and without `--force`)
- [ ] 2.9b Emit `logger.Debug("privilege check", "privileged", ok, "os", runtime.GOOS)` after the privilege probe

---

## 2.5 Agent Configuration

Resolve the agent's runtime configuration — monitoring mode and app-log content access — before the endpoint/token/download stages. This task is independent of Task 2's environment + preflight outputs; its result (`AgentConfig`) is consumed by Task 6 when building the OS-specific install command. Pulled out of Task 2 because it owns a distinct concern (config resolution) and a distinct CLI flag (`--monitoring-mode`).

**Goals:**

- **Default path** — when no flag is passed, `ResolveAgentConfig` returns `{MonitoringMode: "fullstack"}` exactly. This is the zero-config default for OneAgent installs per `AGENTS.md`.
- **Override path** — when `--monitoring-mode <value>` is passed (or `opts.MonitoringMode != ""`), the field is overridden. No allow-list — the value is passed through verbatim to the installer's `--set-monitoring-mode=<value>` flag in Task 6.

**Files:** `pkg/installer/oneagent_v2.go` (extend — scaffolded in Task 1), `pkg/installer/oneagent_v2_test.go` (extend — scaffolded in Task 1), `cmd/install.go` (modify — wire `--monitoring-mode`)

- [x] 2.5.1 Define the `AgentConfig` struct in `oneagent_v2.go`:

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

**Files:** `pkg/installer/oneagent_v2.go` (extend — scaffolded in Task 1), `pkg/installer/oneagent_v2_test.go` (extend — scaffolded in Task 1), `pkg/installer/installer.go` (reuse `ExtractTenantID`)

### Part A — Endpoint resolution

- [ ] 3.1 Define `Endpoint` struct (`Host string`, `Port int`)
- [ ] 3.2 Implement `ResolveEndpoints(c *client.ClassicClient) ([]Endpoint, error)` calling `GET /api/v1/deployment/installer/agent/connectioninfo/endpoints` via the resty client
- [ ] 3.3 Parse the response into `[]Endpoint`; default port `443` when omitted; tolerate both hostnames and IP literals
- [ ] 3.4 Return wrapped errors that include the URL and HTTP status when the API returns 4xx/5xx
- [ ] 3.5 Return an error when the response is empty (treat as a server bug)
- [ ] 3.6 Unit tests with `httptest.Server`: happy path (multiple endpoints), empty response, 401 with body, 5xx
- [ ] 3.7 Confirm `installer.ExtractTenantID()` correctly handles `live.dynatrace.com`, `apps.dynatrace.com`, and `/e/<tenantId>` Managed URLs; add cases if missing
- [ ] 3.8 Emit `logger.Debug("extracted tenant id", "tenant_id", id)` after extraction; do NOT log the full `--environment` URL at Debug if it may contain embedded credentials
- [ ] 3.9 Emit `logger.Debug("resolving tenant endpoints", "url", reqURL)` before the API call
- [ ] 3.10 Emit one `logger.Debug("tenant endpoint", "host", e.Host, "port", e.Port)` per parsed endpoint
- [ ] 3.11 Emit `logger.Verbose("resolved tenant endpoints", "count", len(endpoints))` as the operator-visible milestone
- [ ] 3.12 Unit test (with captured stderr): `--debug` shows per-endpoint lines; `-v` only shows the summary line; default shows nothing

### Part B — Connectivity probe

- [ ] 3.13 Define `ConnectivityResult` struct (`Endpoint`, `Reachable bool`, `Latency time.Duration`, `Error string`) and `ConnectivityReport` struct (`Results []ConnectivityResult`, `AllPassed bool`, `FailedCount int`) for use by the diagnostic exit modes below
- [ ] 3.14 Implement `CheckAllEndpoints(endpoints []Endpoint, timeout time.Duration) ConnectivityReport` using `net.DialTimeout("tcp", "host:port", timeout)` per endpoint; run probes concurrently and collect results with timing and error details
- [ ] 3.15 Use a default probe timeout of `5s` per endpoint; do not make it configurable at this stage
- [ ] 3.16 Wire `opts.ConnectivityCheckOnly` (from `InstallOptions`): if `true`, call `CheckAllEndpoints`, print a table-format report (`host:port  reachable  latency  error`), then return `nil` without proceeding to token minting, download, or install
- [ ] 3.17 Wire `opts.PrintEndpoints` (from `InstallOptions`): if `true`, print resolved endpoints one per line in `host:port` format after `ResolveEndpoints`, then return `nil` without probing or installing
- [ ] 3.18 When `len(report.Results with Reachable==false) > 0` in the normal install path, print a WARNING block and continue (non-blocking)
- [ ] 3.19 When all endpoints are reachable in the normal install path, print nothing (do not add a "all endpoints reachable" success line at default verbosity)
- [ ] 3.20 Wire `opts.SkipConnectivityCheck` (from `InstallOptions`): when `true`, skip `CheckAllEndpoints` entirely; emit `logger.Debug("skipping connectivity probe", "reason", "--skip-connectivity-check")`
- [ ] 3.21 Emit one `logger.Debug("endpoint probe result", "host", r.Endpoint.Host, "port", r.Endpoint.Port, "reachable", r.Reachable, "latency_ms", r.Latency.Milliseconds(), "error", r.Error)` per result when probing
- [ ] 3.22 Emit `logger.Verbose("connectivity probe complete", "total", len(report.Results), "failed", report.FailedCount)` after all probes finish (only when the probe ran in the normal path)
- [ ] 3.23 Unit tests:
  - Normal path: all reachable (no output, no remediation), some blocked (warning printed), all blocked, `SkipConnectivityCheck == true` (no probe, no output)
  - `--connectivity-check-only`: mixed reachable/blocked endpoints print the expected table, no token mint
  - `--print-endpoints`: prints correct `host:port` lines, exits before probing
  - Use an in-process TCP listener (`net.Listen`) to control reachability without real network access
