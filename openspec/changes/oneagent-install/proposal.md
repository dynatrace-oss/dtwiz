# Why

The existing OneAgent installer flow downloads the installer binary without any signature verification, then shells out with positional arguments and no structured command representation. This change implements a streamed download (reusing the credentials already embedded in the ClassicClient — same pattern as every other installer in the repo, confirmed by `oneagent-configure`), Linux signature verification against the published Dynatrace root Certificate Authority, and OS-specific command construction and execution.

## What Changes

- `DownloadInstaller(c *client.ClassicClient, env Environment) (string, error)`: streams the installer to a temp file using `c.HTTP().R()` — the resty client already carries the correct `Authorization` header. Temp file permissions are tightened to `0o700` on Unix.
- `VerifyInstallerSignature(env Environment, installerPath string, skip bool) error`: on Linux, verifies the installer's CMS signature against `https://ca.dynatrace.com/dt-root.cert.pem` via `openssl cms -verify`. Non-Linux and `--no-verify-signature` skip silently. Missing `openssl` is a hard error, not a silent skip.
- `BuildInstallCommand(env Environment, cfg AgentConfig, opts InstallOptions, installerPath string) ([]string, error)`: constructs the OS-specific argv from `AgentConfig` (`--set-monitoring-mode`, `--set-app-log-content-access`) and options.
- `ExecuteInstallCommand(argv []string, quiet bool) (int, error)`: runs the installer subprocess, streaming output. Dry-run is handled upstream in `InstallOneAgentV2` before this function is called.

## Capabilities

### New Capabilities

- `oneagent-installer-download`: Streamed installer download authenticated by the embedded ClassicClient credential; Linux signature verification via `openssl cms -verify`.
- `oneagent-install-execution`: OS-specific install command construction from `AgentConfig`; sudo/UAC elevation; `--dry-run` support.

## Impact

- **Modified files:** `pkg/installer/oneagent/` (extend — download in `download.go`, verification in `verify.go`, build/execute in `oneagent.go`), `pkg/installer/oneagent/oneagent_test.go` (extend)
- **New flag:** `--no-verify-signature` on `installOneAgentCmd` (skip Linux signature check).
- **Dependency:** `openssl` must be present on Linux unless `--no-verify-signature` is passed. Missing `openssl` aborts the install with a clear error.
