# Design

## Context

`pkg/installer/oneagent.go` today implements `InstallOneAgent(c *client.ClassicClient, dryRun, quiet bool, hostGroup string) error`. The flow is:

1. Optional connectivity ping to `/api/v1/time`.
2. `GET /api/v1/deployment/installer/agent/{osType}/default/latest?arch={arch}` using the user's `--access-token`.
3. Write to a temp file, `chmod 0755` on Unix.
4. Shell out with optional `sudo`, passing `--set-server=<apiURL>`, `--set-app-log-content-access=true`, and optionally `--set-host-group=...`.

Gaps relative to the production requirements for this rework:

- No OS/arch validation beyond the installer URL lookup (AIX is not explicitly rejected; macOS is rejected but with a flat error).
- No pre-flight check for an already-installed OneAgent. `pkg/analyzer/detect_oneagent_*.go` already exists for this purpose but is not called from the installer.
- No privilege check before download. `pkg/installer/sudo_unix.go` / `sudo_windows.go` exist but are only invoked at command-execution time.
- The user's long-lived token is reused for the installer download — no minting.
- No signature verification on Linux. The Dynatrace Linux installer ships with an OpenSSL-CMS detached signature, and `ca.dynatrace.com/dt-root.cert.pem` is the published root.
- No post-install confirmation. The function returns success as soon as the installer subprocess exits 0. (The caller wires `WatchIngest` from `pkg/installer/ingest_watch.go` which polls Grail via DQL for general data flow, but it does not confirm that *this specific host* has registered.)
- `--monitoring-mode` is not user-configurable — the flow always behaves as full-stack.

The existing implementation is not assumed correct; this change rebuilds the flow in `pkg/installer/oneagent/` and replaces the old function at the end.

The codebase already has:

- `pkg/featureflags/featureflags.go` — registry-based feature flag package (`Flag` enum, `RegisterFlags`, `ApplyCLIOverrides`, `IsEnabled`, `SetCLIOverrideForTest`). Adding `ONEAGENT_POC` is a one-const + one-registry-entry addition.
- `pkg/client/client.go` — resty-based `*client.ClassicClient` injected via `setupClient()` in `cmd/root.go`. The new flow reuses this client.
- `pkg/installer/sudo_*.go` — platform-tagged privilege helpers (`needsSudo`, `RunCommand`, `RunCommandQuiet`).
- `pkg/analyzer/detect_oneagent_*.go` — existing-agent detection on Linux/macOS/Windows.

## Goals / Non-Goals

**Goals:**

- A new `InstallOneAgentV2` flow in `pkg/installer/oneagent/` covering the eight critical tasks documented in [tasks.md](../tasks.md).
- Fail-fast pre-flights before any network work.
- Mandatory installer-token minting with no fallback.
- Linux signature verification gated only by the explicit `--no-verify-signature` flag.
- Post-install confirmation that the host appears in the tenant inventory.
- Feature-flag branching during development; clean removal of both the flag and the old code path at the end of Task 8.
- Optional, non-blocking connectivity diagnostics (Task 9).

**Non-Goals:**

- macOS direct-install support — remains unsupported; the existing error is preserved.
- Custom monitoring modes beyond what the OneAgent installer already accepts — `--monitoring-mode` is passed through as `--set-monitoring-mode=<value>` without dtwiz-side validation of the value set.
- Replacing `pkg/analyzer/detect_oneagent_*.go` — it is reused as-is for the existing-agent preflight.
- Native Go signature verification — `openssl cms -verify` is shelled out; a Go-crypto rewrite is out of scope.
- AIX support — explicitly rejected.

## Decisions

### 1. Package split: `pkg/installer/oneagent/`

The new flow lives entirely in `pkg/installer/oneagent/` (Go package `oneagent`), leaving the rest of `pkg/installer/` unchanged until Task 8. The package is split across three source files:

- `oneagent.go` — types, entry point `InstallOneAgentV2`, `DefaultAgentConfig`, `ResolveAgentConfig`, `detectRuntimeEnvironment`
- `download.go` — `DownloadInstaller`, `readErrorBody`, OS/arch helpers
- `verify.go` — `VerifyInstallerSignature`, CA fetch, openssl invocation

At Task 8, the `pkg/installer/oneagent/` package replaces the old `InstallOneAgent` entirely and the feature flag is removed.

This split keeps each concern isolated, makes rollback mechanical (delete the directory), and avoids touching the old call sites until Task 8.

### 2. Feature flag: `ONEAGENT_POC`, removed at Task 8

`pkg/featureflags/featureflags.go` already supports adding new flags via a single registry entry. We add:

```go
const (
    AllRuntimes Flag = iota
    OneAgentPoC
)

// ...registry entry: name "oneagent-poc", env "DTWIZ_ONEAGENT_POC", default false
```

During Tasks 1–7, `cmd/install.go`'s `installOneAgentCmd.RunE` branches:

```go
if featureflags.IsEnabled(featureflags.OneAgentPoC) {
    return oneagent.InstallOneAgentV2(c, opts)
}
return installer.InstallOneAgent(c.Classic, installDryRun, quiet, hostGroup)
```

At Task 8, the branch is deleted, `InstallOneAgent` is replaced with the v2 implementation, and the `OneAgentPoC` registry entry + constant are removed. The cobra `--oneagent-poc` flag disappears with the registry entry.

### 3. End-to-end flow

The new `InstallOneAgentV2` orchestrates the following stages, each implemented as an exported (testable) function. Stages run in strict order; any failure short-circuits with a clear error.

```text
InstallOneAgentV2(c *client.Client, opts InstallOptions) error
  │
  ├─ DetectEnvironment()                          // Task 2A
  ├─ RunPreflightChecks(env, opts)                // Task 2B
  ├─ ResolveAgentConfig(opts)                     // Task 2.5
  ├─ ResolveEndpoints(c.Classic, tenantID)        // Task 3
  ├─ (optional) CheckAllEndpoints(...)            // Task 9
  ├─ MintInstallerToken(c.Classic)                // Task 4 — MANDATORY
  ├─ DownloadInstaller(c.Classic, mintedToken,
  │      env)                                     // Task 5A
  ├─ VerifyInstallerSignature(env, path,
  │      opts.NoVerifySignature)                  // Task 5B
  ├─ BuildInstallCommand(env, cfg, path)          // Task 6A
  ├─ ExecuteInstallCommand(cmd, opts.DryRun)      // Task 6B
  └─ WaitForHostRegistration(c.Platform,
         hostname, 2*time.Minute)                 // Task 7 — warn on timeout
```

The entry function takes `*client.Client` (not `*client.ClassicClient`) because post-install verification needs the `Platform` client for DQL/Grail queries, while download / token minting / endpoint resolution use the `Classic` client. This matches the existing `cmd/install.go` pattern where `setupClient()` returns a full `*client.Client` that already carries both halves.

**`InstallOptions` struct** carries all CLI-derived inputs:

```go
type InstallOptions struct {
    DryRun                 bool
    Force                  bool
    MonitoringMode         string
    NoVerifySignature      bool
    SkipConnectivityCheck  bool
    ConnectivityCheckOnly  bool
    PrintEndpoints         bool
    HostGroup              string
    Quiet                  bool
}
```

This replaces the positional `(dryRun, quiet, hostGroup)` signature in Task 8.

### 4. Pre-flight: detect environment, then check, then configure

Pre-flights run in a single function that returns either a fully-populated context (env + agent config) or a typed error. Order:

1. `DetectEnvironment()` populates:

   ```go
   type Environment struct {
       OS        string // "windows", "linux", "aix", "other"
       Arch      string // "x86", "arm", "other"
       Supported bool
       Reason    string // populated when Supported is false
   }
   ```

   Mapping: `runtime.GOOS` → `OS`; `runtime.GOARCH` (`amd64`/`386` → `"x86"`, `arm64`/`arm` → `"arm"`). `aix` and `darwin` resolve to `Supported: false` with a `Reason`. macOS's existing error message is preserved verbatim.

2. `RunPreflightChecks(env, opts)`:
   - **Existing-OneAgent check:** call into `pkg/analyzer/detect_oneagent_*.go`. If detected and `opts.Force == false`, return `"OneAgent already installed at {path}. Use --force to reinstall."`. If `opts.Force == true`, log via `logger.Debug` and proceed.
   - **Privilege check:** Unix calls `sudo -k && sudo -l` (reuse `needsSudo` semantics); Windows checks the process token for the BUILTIN\Administrators SID. On failure: `"This command requires administrator/root privileges. Please run with sudo or as an administrator."`.

3. `ResolveAgentConfig(opts)`:

   ```go
   type AgentConfig struct {
       MonitoringMode string // default "fullstack"
   }
   ```

   `--monitoring-mode <value>` overrides the default. No allow-list — the value is passed through to `--set-monitoring-mode=<value>`.

### 5. Tenant ID extraction

`--environment` is documented as a full URL. We extract the first DNS label as the tenant ID, reusing `installer.ExtractTenantID(envURL)` from `pkg/installer/installer.go`:

- `https://abc12345.live.dynatrace.com` → `abc12345`
- `https://abc12345.apps.dynatrace.com` → `abc12345`
- `https://my-managed.example.com/e/abc12345` → `abc12345` (the `/e/<id>` path is unwrapped first)

If extraction returns empty, the new flow errors with a guidance message rather than guessing.

### 6. Endpoint resolution: dynamic, no hardcoded list

```go
type Endpoint struct {
    Host string // hostname or IP literal
    Port int    // default 443 when omitted
}

func ResolveEndpoints(c *client.ClassicClient) ([]Endpoint, error)
```

Uses the classic client to call `GET /api/v1/deployment/installer/agent/connectioninfo/endpoints`. The response body is a newline-/semicolon-separated list of `host:port` entries (or just `host`, defaulting to 443). Errors are wrapped with the URL and HTTP status to make debugging straightforward.

Empty response → error; an empty endpoint set is treated as a server bug and surfaced loudly.

### 7. Installer token: mandatory, scoped, short-lived

`POST /api/v2/tokens` with body:

```json
{
  "name": "dtwiz-oneagent-installer",
  "scopes": ["InstallerDownload"],
  "expiresIn": { "value": 1, "unit": "HOURS" }
}
```

Auth header on this request uses the user's `--access-token` via the existing client. The response's `token` field is held only in memory and passed to `DownloadInstaller`. There is no fallback path: if minting fails (401/403/5xx, network), the whole install fails with the body of the API error included.

Logging: the token value is never written to logs, stdout, or files. Only the fact that minting succeeded/failed plus the response status is logged.

### 8. Download + signature verification

**Download:** reuses the existing `downloadOneAgentInstaller` pattern (resty streaming to a temp file) but uses the minted token instead of the user's `--access-token`. The temp file is `os.Chmod(0o700)` on Unix to prevent other local users from reading it.

**Alternative considered (Go-native crypto):** The Linux verification could be implemented entirely in Go using `crypto/x509` + `crypto.cms` (third-party) to avoid the `openssl` subprocess dependency. Rejected — `openssl cms -verify` against the Dynatrace-published pipeline is the documented and supported approach; rewriting it in Go risks subtle divergence in PKCS#7 parsing edge cases and adds an external Go-CMS dependency. Revisit if `openssl` availability becomes a problem in target environments.

**Linux signature verification (`oneagent-installer-download`):**

- Skipped if `opts.NoVerifySignature` is true.
- Skipped if `env.OS != "linux"`.
- Otherwise:
  1. Look up `openssl` via `exec.LookPath`. If missing → error: `"openssl is required to verify the installer signature. Install openssl or pass --no-verify-signature to skip."`
  2. Download `https://ca.dynatrace.com/dt-root.cert.pem` to a second temp file.
  3. Run the verification pipeline as documented by Dynatrace:

     ```bash
     ( echo 'Content-Type: multipart/signed; protocol="application/x-pkcs7-signature"; micalg="sha-256"; boundary="--SIGNED-INSTALLER"'; \
       echo ; echo '----SIGNED-INSTALLER' ; cat <installer> ) \
     | openssl cms -verify -CAfile <dt-root.cert.pem>
     ```

  4. Non-zero exit → error with stderr included.

`--no-verify-signature` is the only way to skip on Linux; missing-openssl does not silently skip.

### 9. Build + execute install command

`BuildInstallCommand(env Environment, cfg AgentConfig, installerPath string) ([]string, error)` returns the argv. Linux includes a leading `/bin/sh`; Windows runs the `.exe` directly. The minimal arg set is:

```bash
--set-monitoring-mode=<cfg.MonitoringMode>
```

`--set-host-group=<opts.HostGroup>` is appended when `opts.HostGroup != ""`. On Linux, `--set-server=<apiURL>` is also passed (preserves current behavior — the installer needs the server explicitly).

`ExecuteInstallCommand(argv []string, dryRun, quiet bool) (int, error)`:

- `dryRun` → print `Command: <argv joined>` and return `(0, nil)`.
- Otherwise → wrap with `sudo` on Unix when `needsSudo()` returns true; on Windows, rely on the existing UAC re-launch path.
- stdout/stderr are streamed to the user when `quiet == false`, captured otherwise. The subprocess exit code is returned alongside any wrapping error.

### 10. Post-install verification

```go
WaitForHostRegistration(p *client.PlatformClient, hostname string, timeout time.Duration) (string, error)
```

Verification reuses the **Platform API / Grail** stack already used by `WatchIngest` ([pkg/installer/ingest_watch.go:70](pkg/installer/ingest_watch.go#L70)). The function `POST`s a DQL query to `<apps-url>/platform/storage/query/v1/query:execute` with Bearer auth (platform token), looking for the local host by name in smartscape:

```dql
smartscapeNodes HOST, from:now()-1h
  | filter name == "<hostname>"
  | fields id, name
  | limit 1
```

- Local hostname via `os.Hostname()` (case-insensitive match via DQL `lower()` if Windows hostname casing diverges from what Dynatrace records).
- Poll every 5s; on the first non-empty record set, extract the `id` field (e.g. `HOST-abc123`) and return it.
- Default timeout: 2 minutes.
- **Warning, not error:** on timeout, the function returns `("", nil)` after printing `⚠ Host registration verification timed out after 2 minutes. Check the tenant UI.`. The install is still considered successful.
- The DQL request payload (`requestTimeoutMilliseconds`, `maxResultRecords`) mirrors the values in `executeDQL` ([ingest_watch.go:280](pkg/installer/ingest_watch.go#L280)) so we don't introduce a parallel HTTP convention.

The polling cadence is `5s` to balance API load and time-to-first-detection; this also matches the polling cadence in `watchIngest()` (defined by `watchPollInterval` in `pkg/installer/ingest_watch.go`).

**Why DQL/Grail over `/api/v1/entity/infrastructure/hosts`:** the classic Hosts API would require Api-Token auth and a second URL family in the post-install path. Reusing the platform DQL path keeps the post-install verification consistent with the rest of the post-install behavior (`WatchIngest` already runs after this in `cmd/install.go`), uses the platform token that's already validated by `validateCredentials`, and avoids introducing a new auth-and-URL combination to the v2 flow.

**Failure mode:** if the platform token is missing (`pToken == ""`), `WaitForHostRegistration` SHALL skip the polling and print a warning `"⚠ Platform token not set — skipping host registration verification."` rather than fail. The install still reports success; this mirrors `WatchIngest`'s early-return at [ingest_watch.go:64-67](pkg/installer/ingest_watch.go#L64-L67).

### 11. Debug logging strategy

All diagnostic logging in the new flow SHALL use `pkg/logger`, following the project-wide convention already established in `pkg/installer/otel_*.go`: a short lowercase message followed by structured key-value pairs (`logger.Debug("resolved endpoints", "count", 3, "url", reqURL)`). `pkg/logger` writes to stderr and is gated by `--debug` (Debug) / `-v` (Verbose); without those flags the logs are suppressed entirely, so the user-facing UX is unchanged.

Logging is layered:

- **`logger.Debug`** — fine-grained diagnostics: detected environment, preflight outcomes, request URLs, HTTP statuses, file paths, durations, retry attempts. This is where most of the new logging lives, and it is the right level for "how do I troubleshoot a stuck install" cases.
- **`logger.Verbose`** — milestone events that operators may want to see without full debug noise (e.g. "endpoints resolved", "installer downloaded", "host registered"). Used sparingly.
- **`logger.Warn`** — non-fatal degradations that the user should see even at default verbosity: signature-skip notices, post-install timeout, connectivity probe failures. These also produce user-visible stdout messages (per their spec); the `logger.Warn` line is for structured machine-readable consumption.

**Redaction rules (mandatory):**

- The minted installer token value SHALL NEVER appear in any log line, at any level, including in error messages, request dumps, or response bodies. Mint-success logs record the HTTP status only.
- The user-supplied `--access-token` and `--platform-token` SHALL NEVER appear in any log line.
- Response bodies that may contain token values (the 2xx body of `POST /api/v2/tokens`) SHALL NOT be logged. Failure bodies (4xx/5xx) MAY be logged because they describe the error, not the token.
- The `Authorization` request header SHALL NOT be logged. The resty client's verbose hooks (gated by `--verbose`) already redact this — no change needed there.

**Per-stage log lines** (canonical messages; exact wording may evolve, but the structured keys are part of the spec):

| Stage | Level | Message | Keys |
|---|---|---|---|
| Env detection | Debug | `"detected environment"` | `os`, `arch`, `supported`, `reason` |
| Existing-agent check | Debug | `"existing oneagent detected"` | `path`, `force_override` |
| Privilege check | Debug | `"privilege check"` | `privileged`, `os` |
| Agent config | Debug | `"resolved agent config"` | `monitoring-mode`, `app_log_content_access` |
| Tenant ID | Debug | `"extracted tenant id"` | `tenant_id` (NOT the full URL if it contains credentials) |
| Endpoint API call | Debug | `"resolving tenant endpoints"` | `url` |
| Endpoint resolution | Verbose | `"resolved tenant endpoints"` | `count` |
| Endpoint resolution | Debug | (per endpoint) `"tenant endpoint"` | `host`, `port` |
| Token mint request | Debug | `"minting installer token"` | `url`, `scopes`, `expires_in` |
| Token mint success | Debug | `"installer token minted"` | `status` (HTTP code only, never the value) |
| Token mint failure | Debug | `"installer token mint failed"` | `status`, `body` |
| Download start | Debug | `"downloading installer"` | `url`, `os`, `arch` (no token) |
| Download done | Verbose | `"installer downloaded"` | `path`, `size_bytes` |
| Signature: tool lookup | Debug | `"openssl lookup"` | `path`, `found` |
| Signature: cert fetch | Debug | `"fetching dynatrace root ca"` | `url`, `path` |
| Signature: verify | Verbose | `"installer signature verified"` | (none) |
| Signature: failure | Debug | `"signature verification failed"` | `exit_code`, `stderr` |
| Build command | Debug | `"built install command"` | `argv` (token is not in argv — safe) |
| Execute start | Debug | `"executing installer"` | `argv` |
| Execute done | Verbose | `"installer exited"` | `exit_code`, `duration` |
| Poll attempt | Debug | `"polling smartscape for host"` | `hostname`, `attempt`, `elapsed` |
| Poll transient err | Debug | `"smartscape poll error"` | `status` or `error` |
| Poll success | Verbose | `"host registered"` | `host_id`, `hostname`, `elapsed` |
| Poll timeout | Warn | `"host registration timed out"` | `timeout`, `hostname` |
| Probe per endpoint | Debug | `"endpoint probe result"` | `host`, `port`, `reachable`, `latency`, `error` |
| Probe summary | Verbose | `"connectivity probe complete"` | `total`, `failed` |

Test coverage for the redaction rules is enforced by unit tests in Task 4 (audit that the minted token value never reaches captured log output).

### 12. Optional connectivity probe (Task 9)

```go
CheckAllEndpoints(endpoints []Endpoint, perEndpointTimeout time.Duration) ConnectivityReport
```

- One goroutine per endpoint; `net.DialTimeout("tcp", host:port, perEndpointTimeout)`.
- `ConnectivityReport` aggregates results, latencies, and a `FailedCount`. Failures are warnings.
- `--connectivity-check-only` skips minting/download/install entirely and exits after printing the report.
- `--skip-connectivity-check` skips this stage.
- `--print-endpoints` prints the resolved endpoint list and exits before minting.

### 13. CLI flags

All new flags live on `installOneAgentCmd` (not on `installCmd`, which already owns `--dry-run`):

| Flag | Type | Default | Purpose |
|---|---|---|---|
| `--force` | bool | `false` | Override existing-OneAgent preflight |
| `--monitoring-mode` | string | `"fullstack"` | Passed through as `--set-monitoring-mode` |
| `--no-verify-signature` | bool | `false` | Skip Linux signature verification |
| `--skip-connectivity-check` | bool | `false` | Skip Task 9 probe |
| `--connectivity-check-only` | bool | `false` | Run only Task 9 probe, then exit |
| `--print-endpoints` | bool | `false` | Print resolved endpoints, then exit |

`--dry-run`, `--quiet`, `--host-group` are pre-existing and reused unchanged.

## Risks / Trade-offs

- **`openssl` dependency on Linux** → Some minimal container images (e.g. distroless, scratch-based) lack `openssl`. Mitigation: the error message tells the user to install it or pass `--no-verify-signature`. We do not silently skip.
- **Polling host inventory may race with eventual consistency** → A 2-minute timeout is generous for normal cases, but heavily loaded tenants may take longer. Mitigation: timeout warns rather than fails, with a pointer to the UI. The exit code remains 0.
- **Tenant ID extraction is heuristic** → Customers using vanity hostnames or proxies in front of Dynatrace may not match the first-DNS-label rule. Mitigation: the helper returns empty on ambiguity; a clear error is surfaced rather than guessing.
- **Token-minting failure modes hide credential issues** → If the user-supplied token lacks `tokens.write` scope, minting returns 403. Mitigation: the error wraps the response body so the user can see which scope is missing.
- **Cross-platform privilege check** → Windows admin detection (`OpenProcessToken` + SID lookup) is more involved than `sudo -l`. Mitigation: add a `_windows.go`/`_unix.go` split; if the check itself fails (rather than returning "not admin"), fall back to "attempt and let the installer report" rather than blocking the install.
- **Feature flag removal coordination** → If the v2 flow ships partly behind the flag and ROaches into trunk before Task 8, accidental removal of the flag would expose unfinished code. Mitigation: keep `ONEAGENT_POC` default-false until all of Tasks 2–7 are merged and tested.

## Open Questions

1. **Tenant API surface stability.** Does `connectioninfo/endpoints` return a stable schema across Managed and SaaS? If not, we may need to special-case Managed.
2. **Host-registration DQL exactness.** The proposed `smartscapeNodes HOST | filter name == "<hostname>"` assumes Dynatrace records the host by `os.Hostname()`'s exact value. Edge cases to validate during Task 7 implementation: Windows hostnames recorded in different casing; hostnames with embedded domain (FQDN vs short name); Kubernetes node names vs Linux `hostname` output. The DQL should likely use `lower(name) == lower("<hostname>")` and possibly also match `detectedName` if smartscape exposes it.
3. **Token scope name.** Mint requests specify `InstallerDownload`; the Dynatrace API docs sometimes refer to it as `installerDownload` (camelCase). We will use whichever the API accepts and update the design accordingly.
4. **Typical registration latency.** The 2-minute timeout is a guess. If field data shows registration usually completes inside ~30s, we can shorten the timeout (or expose it as a flag). If it routinely exceeds 2 minutes on busy tenants, we should extend it before users complain.
5. **Signature pipeline portability.** Do all Dynatrace Linux installers post-1.300 ship the CMS signature format the verify pipeline expects? Older installers might require a different command.
6. **Future flag — `--dns-check-only`.** The Task 9 probe currently lumps DNS failures together with TCP failures. A `--dns-check-only` flag could separate "can't resolve" from "can resolve but can't connect", which is the most common diagnostic split when proxies are involved. Not in scope for this change, but a likely follow-up if support tickets show the distinction matters.
