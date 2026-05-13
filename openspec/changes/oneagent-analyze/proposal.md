# Why

The existing `dtwiz install oneagent` flow in `pkg/installer/oneagent.go` is a thin wrapper around the legacy OneAgent shell/exe installer: it performs a single `/api/v1/time` connectivity check, downloads the installer using the user's long-lived API token, and shells out. It has no pre-flight validation, no Linux signature verification, no post-install confirmation, and reuses the caller-supplied token directly for installer download (against Dynatrace recommended practice). We treat the current implementation as untrustworthy — its behavior is not assumed correct and is not preserved.

This change replaces the OneAgent installer flow with a production-ready, zero-config installer aligned with the OneAgent Installation PoC epic. The new flow validates the environment up front, resolves tenant endpoints dynamically from the Dynatrace tenant API, mints a short-lived installer-scoped token, verifies the downloaded installer signature on Linux, executes with proper privilege elevation, and confirms host registration with the tenant before reporting success.

The new flow is gated behind `ONEAGENT_POC` during development. Once Task 8 lands, the flag is removed and the new flow becomes the only flow.

## What Changes

- Pre-flight validation (OS/arch detection, existing-OneAgent check, privilege check) runs before any network work.
- Agent configuration (`MonitoringMode`) resolved early from defaults and `--monitoring-mode` flag.
- Tenant endpoints resolved dynamically from `GET /api/v1/deployment/installer/agent/connectioninfo/endpoints` — no hardcoded IPs.
- Short-lived `InstallerDownload`-scoped token minted via `POST /api/v2/tokens` (1h expiry); the user's long-lived `--access-token` is never passed to the installer binary.
- Linux installer signature verified against `https://ca.dynatrace.com/dt-root.cert.pem` using `openssl cms -verify`; skippable via `--no-verify-signature`.
- OS-specific install command built from the resolved `AgentConfig` (`--set-monitoring-mode=...`) and executed with sudo/UAC; `--dry-run` prints the command without executing.
- Post-install verification polls Grail via the Platform API (`POST <apps-url>/platform/storage/query/v1/query:execute`) with a DQL `smartscapeNodes HOST` query filtered on the local hostname, for up to 2 minutes; timeout is a warning, not a failure. This matches the existing `WatchIngest` post-install flow (`pkg/installer/ingest_watch.go`) rather than introducing a parallel classic-API call path.
- Optional parallel TCP connectivity probe of all resolved endpoints; failures are warnings, not blockers.
- New flags on `dtwiz install oneagent`: `--force`, `--monitoring-mode`, `--no-verify-signature`, `--skip-connectivity-check`, `--connectivity-check-only`, `--print-endpoints`. `--dry-run` already exists on the parent `install` command.
- `ONEAGENT_POC` feature flag added to `pkg/featureflags/featureflags.go` during development; removed at end of Task 8 when the new flow replaces the old.

## Capabilities

### New Capabilities

- `oneagent-preflight`: OS/arch detection, AIX rejection, existing-OneAgent detection (`--force` override), privilege check, agent configuration resolution.
- `oneagent-tenant-endpoints`: Tenant ID extraction from `--environment`; dynamic endpoint resolution via the Dynatrace tenant API.
- `oneagent-installer-token`: Mandatory minting of a short-lived `InstallerDownload`-scoped token; no fallback to the user-supplied token.
- `oneagent-installer-download`: Download via the minted token; Linux signature verification with `openssl cms -verify`.
- `oneagent-install-execution`: OS-specific install command construction from `AgentConfig`; sudo/UAC elevation; `--dry-run` support.
- `oneagent-post-install-verification`: Polling Grail via DQL (`smartscapeNodes HOST`) on the Platform API for agent registration, with a 2-minute timeout and warning-on-timeout semantics.
- `oneagent-connectivity-probe`: Optional parallel TCP probe of resolved endpoints; non-blocking warnings.

### Modified Capabilities

- `oneagent-client-injection` (from archived `refactor-oneagent-http-client`): The signature of `InstallOneAgent` changes to accept an options struct carrying the new flags (`Force`, `MonitoringMode`, `NoVerifySignature`, `SkipConnectivityCheck`, etc.). `cmd/install.go` continues to inject `*client.ClassicClient` from `setupClient()`; the resty-based client, retry policy, and verbose logging behavior are preserved.

## Impact

- **New files:** `pkg/installer/oneagent_v2.go`, `pkg/installer/oneagent_v2_test.go`, `test/e2e/oneagent_test.go` (optional, Task 11).
- **Modified files:**
  - `pkg/installer/oneagent.go` — extended in Task 2 to return the `Environment` struct; replaced entirely in Task 8.
  - `pkg/featureflags/featureflags.go` — `ONEAGENT_POC` added in Task 1; removed in Task 8.
  - `cmd/install.go` — new flags wired on `installOneAgentCmd`; feature-flag branching during development; flag removed in Task 8.
- **No changes:** `pkg/analyzer/detect_oneagent_*.go` is reused for the existing-OneAgent preflight; `pkg/client/` provides the HTTP client; `pkg/installer/sudo_*.go` provides privilege elevation.
- **Breaking change:** `InstallOneAgent`'s signature changes in Task 8 (struct-based options). All callers (`cmd/install.go`, `cmd/setup.go`) update together.
- **Rollback:**
  - During development: set `DTWIZ_ONEAGENT_POC=false` (default) to fall back to the existing flow.
  - Pre-Task-8: delete `pkg/installer/oneagent_v2.go` and revert `cmd/install.go`/`pkg/featureflags/featureflags.go`.
  - Post-Task-8: the old flow no longer exists; rollback is a git revert of the Task-8 commit.
- **Feature flag:** `ONEAGENT_POC` (env `DTWIZ_ONEAGENT_POC`, CLI `--oneagent-poc`) — temporary, removed at the end of Task 8.
