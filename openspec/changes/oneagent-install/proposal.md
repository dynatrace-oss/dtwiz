# Why

The existing OneAgent installer flow downloads the installer binary using the user's long-lived token without any signature verification, then shells out with positional arguments and no structured command representation. This change implements a secure download pipeline — using the minted token from `oneagent-configure` — followed by Linux signature verification and OS-specific command construction and execution.

## What Changes

- `DownloadInstaller(c *client.ClassicClient, mintedToken string, env Environment) (string, error)`: streams the installer to a temp file authenticated with the minted token. Temp file permissions are `0o700` on Unix.
- `VerifyInstallerSignature(env Environment, installerPath string, skip bool) error`: on Linux, verifies the installer's CMS signature against `https://ca.dynatrace.com/dt-root.cert.pem` via `openssl cms -verify`. Non-Linux and `--no-verify-signature` skip silently. Missing `openssl` is a hard error, not a silent skip.
- `BuildInstallCommand(env Environment, cfg AgentConfig, opts InstallOptions, installerPath string) ([]string, error)`: constructs the OS-specific argv from `AgentConfig` (`--set-monitoring-mode`, `--set-app-log-content-access`) and options.
- `ExecuteInstallCommand(argv []string, dryRun, quiet bool) (int, error)`: runs the installer subprocess, streaming output. `--dry-run` prints the command without executing.

## Capabilities

### New Capabilities

- `oneagent-installer-download`: Installer download authenticated with the minted token; Linux signature verification via `openssl cms -verify`.
- `oneagent-install-execution`: OS-specific install command construction from `AgentConfig`; sudo/UAC elevation; `--dry-run` support.

## Impact

- **Modified files:** `pkg/installer/oneagent_v2.go` (extend — download, verification, build, execute), `pkg/installer/oneagent_v2_test.go` (extend)
- **New flag:** `--no-verify-signature` on `installOneAgentCmd` (skip Linux signature check).
- **Dependency:** `openssl` must be present on Linux unless `--no-verify-signature` is passed. Missing `openssl` aborts the install with a clear error.
