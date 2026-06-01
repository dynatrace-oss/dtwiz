# OneAgent Installer Download, Verification, and Execution

## ADDED Requirements

### Requirement: Installer download reuses the ClassicClient credential

`InstallOneAgentV2` SHALL download the OneAgent installer using the resty client embedded in `c.Classic`. The credential is set upstream by `validateCredentials` / `setupClientFromCreds` and SHALL NOT be extracted to a variable in installer code. The download SHALL stream to a temporary file with `0o700` permissions on Unix (preventing other local users from reading the binary).

#### Scenario: Download uses the embedded Authorization header

- **WHEN** `DownloadInstaller(c, env)` is called
- **THEN** the request `Authorization` header is the one configured on `c.HTTP()` upstream
- **AND** no installer code extracts the raw token into a Go variable

#### Scenario: Linux x86 installer URL

- **GIVEN** `env == {OS: "linux", Arch: "x86"}`
- **WHEN** `DownloadInstaller` is called
- **THEN** the request path is `/api/v1/deployment/installer/agent/unix/default/latest?arch=x86`

#### Scenario: Linux arm installer URL

- **GIVEN** `env == {OS: "linux", Arch: "arm"}`
- **WHEN** `DownloadInstaller` is called
- **THEN** the request path is `/api/v1/deployment/installer/agent/unix/default/latest?arch=arm`

#### Scenario: Windows installer URL

- **GIVEN** `env == {OS: "windows", Arch: "x86"}`
- **WHEN** `DownloadInstaller` is called
- **THEN** the request path is `/api/v1/deployment/installer/agent/windows/default/latest?arch=x86`

#### Scenario: Temp file permissions on Unix

- **GIVEN** the download succeeds on a Unix host
- **WHEN** the temp file is created
- **THEN** its permissions are `0o700`

### Requirement: User-facing download output

`DownloadInstaller` SHALL output to stdout (visible at default verbosity, no `-v` required) a one-line confirmation on success via `display.PrintStatusLine("installer", "<filename> (<size>)", display.ColorOK)`. Multi-line or log-style progress output SHALL NOT be printed at default verbosity — finer detail belongs in `logger.Debug`/`logger.Verbose`.

During the download, a TTY-only `\r`-overwriting progress indicator MAY be emitted to stderr showing bytes received (and percentage when `Content-Length` is known). This indicator is suppressed automatically when stderr is not a terminal (CI, pipes). It does not appear on stdout and is erased before the final `PrintStatusLine` confirmation, so the net visible output remains a single line.

#### Scenario: Successful download produces one stdout line

- **GIVEN** the installer downloads successfully
- **WHEN** `DownloadInstaller` returns
- **THEN** stdout contains exactly one line via `display.PrintStatusLine` showing the installer filename and size (e.g. `dynatrace-oneagent-1963786212.sh (245MB)`)
- **AND** the filename is the OS-specific basename of the temp file (Unix env: `.sh`, Windows env: `.exe`)

#### Scenario: TTY progress indicator during download

- **GIVEN** stderr is a terminal
- **WHEN** `DownloadInstaller` is streaming the installer body
- **THEN** a `\r`-overwriting progress line is emitted to stderr at most once per 100 ms showing bytes downloaded and, when `Content-Length` is known, the percentage
- **AND** the progress line is erased (ANSI `\r\033[2K`) before `DownloadInstaller` returns

#### Scenario: Progress suppressed in non-TTY environments

- **GIVEN** stderr is not a terminal (e.g. CI, pipe)
- **WHEN** `DownloadInstaller` streams the installer body
- **THEN** no progress output is emitted to stderr

### Requirement: Download debug logging

`DownloadInstaller` SHALL emit `logger.Debug` for the download start (URL, OS, arch) and `logger.Verbose` for the download completion (path, size in bytes). The credential SHALL NOT appear in any log line. The `Authorization` request header SHALL NOT be logged.

#### Scenario: Download start logged at Debug

- **GIVEN** `--debug` is enabled
- **WHEN** `DownloadInstaller` issues the GET request
- **THEN** stderr contains a Debug line with message `"downloading installer"` and keys `url`, `os`, `arch`
- **AND** no log line contains the raw credential value

#### Scenario: Download completion logged at Verbose

- **GIVEN** `-v` is enabled
- **WHEN** `DownloadInstaller` finishes streaming the body
- **THEN** stderr contains a Verbose line with message `"installer downloaded"` and keys `path`, `size_bytes`

### Requirement: Linux installer signature verification by default

On Linux, `InstallOneAgentV2` SHALL verify the downloaded installer's signature against the Dynatrace root CA published at `https://ca.dynatrace.com/dt-root.cert.pem`. Verification SHALL be skipped only when the user explicitly passes `--no-verify-signature`.

#### Scenario: Signature verified on Linux

- **GIVEN** `env.OS == "linux"`
- **AND** `--no-verify-signature` is NOT passed
- **AND** `openssl` is on `PATH`
- **WHEN** `VerifyInstallerSignature` runs
- **THEN** it downloads `dt-root.cert.pem` to a temp file
- **AND** runs the documented `openssl cms -verify` pipeline against the installer
- **AND** returns nil on exit code 0

#### Scenario: --no-verify-signature skips verification

- **GIVEN** `env.OS == "linux"`
- **AND** `--no-verify-signature` is passed
- **WHEN** `VerifyInstallerSignature` runs
- **THEN** it returns nil without running openssl or downloading the root cert

#### Scenario: Non-Linux skips verification

- **GIVEN** `env.OS != "linux"` (Windows in this scenario)
- **WHEN** `VerifyInstallerSignature` runs
- **THEN** it returns nil without running any subprocess

### Requirement: Missing openssl on Linux is a clear error

When `env.OS == "linux"` and `--no-verify-signature` is not passed, `VerifyInstallerSignature` SHALL look up `openssl` via `exec.LookPath`. If `openssl` is missing, the install SHALL fail with the message `"openssl is required to verify the installer signature. Install openssl or pass --no-verify-signature to skip."`. Missing-openssl SHALL NOT silently skip verification.

#### Scenario: Missing openssl

- **GIVEN** `env.OS == "linux"`
- **AND** `--no-verify-signature` is NOT passed
- **AND** `openssl` is not on `PATH`
- **WHEN** `VerifyInstallerSignature` runs
- **THEN** it returns the missing-openssl error
- **AND** `ExecuteInstallCommand` is not called

### Requirement: User-facing signature verification output

`VerifyInstallerSignature` SHALL output to stdout on successful Linux verification via `display.PrintStatusLine("signature", "Installer signature verified", display.ColorOK)`. The skip paths (`--no-verify-signature` set, or non-Linux OS) SHALL produce no stdout output. Verification failure produces an error returned to the caller, not stdout output.

During verification, TTY-only `\r`-overwriting pending lines MAY be emitted to stderr via `display.PrintPending` at two milestones: before the root CA is fetched (`"fetching root CA..."`) and before openssl runs (`"verifying..."`). These lines are suppressed automatically when stderr is not a terminal (CI, pipes) and are erased with `display.ClearPending` before `VerifyInstallerSignature` returns (on both success and failure paths), so the net visible output remains a single `PrintStatusLine` on success.

#### Scenario: Successful Linux verification outputs status line

- **GIVEN** `env.OS == "linux"`
- **AND** `--no-verify-signature` is NOT set
- **WHEN** the openssl pipeline exits 0
- **THEN** stdout contains the status line via `display.PrintStatusLine` indicating signature verified

#### Scenario: Skip path produces no output

- **GIVEN** `--no-verify-signature` is set OR `env.OS != "linux"`
- **WHEN** `VerifyInstallerSignature` returns nil
- **THEN** stdout contains no signature-related output

### Requirement: Signature verification debug logging

`VerifyInstallerSignature` SHALL emit `logger.Debug` lines for the openssl lookup, the root-cert fetch, and the verification outcome. On success, `logger.Verbose` SHALL emit `"installer signature verified"` (no keys). On failure, `logger.Debug` SHALL include the openssl exit code and captured stderr.

#### Scenario: Openssl lookup logged

- **GIVEN** `--debug` is enabled
- **WHEN** `VerifyInstallerSignature` calls `exec.LookPath("openssl")`
- **THEN** stderr contains a Debug line with message `"openssl lookup"` and keys `path`, `found`

#### Scenario: Root cert fetch logged

- **GIVEN** `--debug` is enabled
- **WHEN** `VerifyInstallerSignature` downloads `dt-root.cert.pem`
- **THEN** stderr contains a Debug line with message `"fetching dynatrace root ca"` and keys `url`, `path`

#### Scenario: Verification success at Verbose

- **GIVEN** `-v` is enabled
- **AND** the openssl pipeline exits 0
- **WHEN** `VerifyInstallerSignature` returns nil
- **THEN** stderr contains a Verbose line with message `"installer signature verified"`

#### Scenario: Verification failure logs exit code and stderr

- **GIVEN** `--debug` is enabled
- **AND** the openssl pipeline exits non-zero with stderr `"Verification failure"`
- **WHEN** `VerifyInstallerSignature` returns the error
- **THEN** stderr contains a Debug line with message `"signature verification failed"` and keys `exit_code`, `stderr`

### Requirement: Signature verification failure aborts install

When `openssl cms -verify` returns a non-zero exit code, `VerifyInstallerSignature` SHALL return an error including the captured stderr. The install SHALL abort before `ExecuteInstallCommand` is called.

#### Scenario: Verification fails

- **GIVEN** the downloaded installer's signature does not match the Dynatrace root CA
- **WHEN** `VerifyInstallerSignature` runs the openssl pipeline
- **THEN** it returns an error whose message includes the openssl stderr
- **AND** `ExecuteInstallCommand` is not called

### Requirement: Install command built from AgentConfig

`InstallOneAgentV2` SHALL construct the OS-specific install command argv from the resolved `AgentConfig` and `InstallOptions`. The argv SHALL include `--set-monitoring-mode=<cfg.MonitoringMode>` and `--set-app-log-content-access=<cfg.AppLogContentAccess>`, plus OS-specific elements documented below.

#### Scenario: Linux argv

- **GIVEN** `env == {OS: "linux", Arch: "x86"}`
- **AND** `cfg == {MonitoringMode: "fullstack", AppLogContentAccess: true}`
- **AND** `opts.HostGroup == ""`
- **AND** the user is non-root with sudo available
- **WHEN** `BuildInstallCommand` runs
- **THEN** the argv is `["sudo", "/bin/sh", <installerPath>, "--set-server=<apiURL>", "--set-monitoring-mode=fullstack", "--set-app-log-content-access=true"]`

#### Scenario: Linux argv with host group

- **GIVEN** `opts.HostGroup == "prod-web"`
- **AND** all other inputs are the Linux defaults
- **WHEN** `BuildInstallCommand` runs
- **THEN** the argv contains `"--set-host-group=prod-web"`

#### Scenario: Linux argv as root skips sudo

- **GIVEN** the process is root (`needsSudo()` returns false)
- **WHEN** `BuildInstallCommand` runs on Linux
- **THEN** the argv does NOT begin with `"sudo"`

#### Scenario: Windows argv

- **GIVEN** `env.OS == "windows"`
- **AND** `cfg == {MonitoringMode: "fullstack", AppLogContentAccess: true}`
- **AND** `opts.Quiet == false`
- **WHEN** `BuildInstallCommand` runs
- **THEN** the argv is `[<installerPath>, "--set-monitoring-mode=fullstack", "--set-app-log-content-access=true"]`

#### Scenario: Windows --quiet must be first flag

- **GIVEN** `env.OS == "windows"`
- **AND** `opts.Quiet == true`
- **WHEN** `BuildInstallCommand` runs
- **THEN** the argv is `[<installerPath>, "--quiet", "--set-monitoring-mode=...", "--set-app-log-content-access=..."]`
- **AND** `"--quiet"` appears before any `--set-*` flag

#### Scenario: Custom monitoring mode passed through

- **GIVEN** `cfg.MonitoringMode == "infra-only"`
- **WHEN** `BuildInstallCommand` runs
- **THEN** the argv contains `"--set-monitoring-mode=infra-only"`

### Requirement: --dry-run prints command without executing

When `opts.DryRun == true`, `ExecuteInstallCommand` SHALL print the argv (joined with single spaces) prefixed with `"Command: "` and return `(0, nil)` without launching any subprocess.

#### Scenario: Dry-run does not execute

- **GIVEN** `opts.DryRun == true`
- **WHEN** `ExecuteInstallCommand(argv, true, false)` is called
- **THEN** stdout contains `"Command: " + strings.Join(argv, " ")`
- **AND** no subprocess is spawned

#### Scenario: Dry-run still confirms preflight passed

- **GIVEN** `--dry-run` is passed
- **WHEN** `InstallOneAgentV2` runs end-to-end
- **THEN** `DetectEnvironment`, `RunPreflightChecks`, `ResolveAgentConfig`, `ResolveEndpoints`, `DownloadInstaller`, and `VerifyInstallerSignature` all run
- **AND** `ExecuteInstallCommand` only prints the command
- **AND** `WaitForHostRegistration` is skipped (nothing to verify)

### Requirement: Execute streams output and captures exit code

When `opts.DryRun == false`, `ExecuteInstallCommand` SHALL launch the installer subprocess and stream its stdout/stderr to the user (or capture them when `quiet == true`). The subprocess exit code SHALL be returned alongside any wrapping error. A non-zero exit SHALL be treated as a failure with the captured output included in the error message.

#### Scenario: Successful install

- **GIVEN** the installer subprocess exits with code 0
- **WHEN** `ExecuteInstallCommand` returns
- **THEN** it returns `(0, nil)`

#### Scenario: Failed install

- **GIVEN** the installer subprocess exits with code 7 and stderr `"failed to write /opt/dynatrace"`
- **WHEN** `ExecuteInstallCommand` returns
- **THEN** it returns `(7, error)` where the error message contains the exit code and the stderr text

#### Scenario: Quiet mode captures output

- **GIVEN** `opts.Quiet == true`
- **AND** the installer succeeds
- **WHEN** `ExecuteInstallCommand` runs
- **THEN** stdout/stderr are NOT streamed to the user's terminal
- **AND** the install reports `"OneAgent installed successfully."` on success

### Requirement: User-facing execution output

`ExecuteInstallCommand` SHALL output user-facing stdout messages at default verbosity (no `-v` required): `display.PrintStatusLine("execute", "Executing installer...", display.ColorMessage)` when the subprocess starts (non-dry-run, non-quiet path), and `display.PrintStatusLine("result", "Installer executed successfully", display.ColorOK)` on a zero exit code. On non-zero exit, the wrapped error returned to the caller is sufficient — no separate stdout line is required.

#### Scenario: Non-quiet execution outputs status lines

- **GIVEN** `opts.DryRun == false` and `opts.Quiet == false`
- **AND** the installer subprocess exits 0
- **WHEN** `ExecuteInstallCommand` returns
- **THEN** stdout contains execution status line via `display.PrintStatusLine`
- **AND** stdout contains success status line via `display.PrintStatusLine`

#### Scenario: Quiet mode suppresses execution output

- **GIVEN** `opts.Quiet == true`
- **AND** the installer subprocess exits 0
- **WHEN** `ExecuteInstallCommand` returns
- **THEN** stdout contains no execution or success output

### Requirement: Build and execution debug logging

`BuildInstallCommand` SHALL emit a `logger.Debug` line capturing the full argv. `ExecuteInstallCommand` SHALL emit a `logger.Debug` line at execution start and a `logger.Verbose` line at completion with exit code and duration. The credential is never placed into argv, so the argv is safe to log at Debug.

#### Scenario: Built command logged at Debug

- **GIVEN** `--debug` is enabled
- **WHEN** `BuildInstallCommand` returns the argv
- **THEN** stderr contains a Debug line with message `"built install command"` and key `argv`

#### Scenario: Execution start logged at Debug

- **GIVEN** `--debug` is enabled
- **AND** `opts.DryRun == false`
- **WHEN** `ExecuteInstallCommand` launches the subprocess
- **THEN** stderr contains a Debug line with message `"executing installer"` and key `argv`

#### Scenario: Execution completion logged at Verbose

- **GIVEN** `-v` is enabled
- **WHEN** the installer subprocess exits
- **THEN** stderr contains a Verbose line with message `"installer exited"` and keys `exit_code`, `duration`

#### Scenario: Dry-run does not emit execution logs

- **GIVEN** `opts.DryRun == true`
- **WHEN** `ExecuteInstallCommand` runs
- **THEN** stderr does NOT contain `"executing installer"` or `"installer exited"` log lines (no subprocess was started)

### Requirement: Privilege elevation via existing infrastructure

`ExecuteInstallCommand` SHALL rely on the existing privilege-elevation infrastructure: `sudo` on Unix (via `needsSudo()` and the `sudo` argv prefix from `BuildInstallCommand`), and the existing Windows UAC handling (the installer `.exe` triggers UAC on launch).

#### Scenario: Sudo invoked on Unix when needed

- **GIVEN** the process is non-root on Linux
- **WHEN** `ExecuteInstallCommand` runs
- **THEN** the subprocess is `sudo /bin/sh <installer> ...`

#### Scenario: Direct invocation on Windows

- **GIVEN** the process runs on Windows
- **WHEN** `ExecuteInstallCommand` runs
- **THEN** the subprocess is the installer `.exe` directly (UAC handled by the installer)
