# OneAgent Windows-Specific Implementation

## ADDED Requirements

### Requirement: Windows installer download and file handling

Windows installer downloads SHALL be saved with a `.exe` extension so the operating system recognizes the file as executable. The temp file creation and permission handling SHALL be Windows-specific.

#### Scenario: Windows temp file has .exe extension

- **GIVEN** `env == {OS: "windows", Arch: "x86"}`
- **WHEN** `DownloadInstaller(c, token, env)` creates a temp file
- **THEN** the temp file name ends with `.exe`
- **AND** the file path can be executed directly without shell interpretation

#### Scenario: Windows download URL path segment

- **GIVEN** `env == {OS: "windows", Arch: "x86"}`
- **WHEN** `DownloadInstaller` is called
- **THEN** the request path is `/api/v1/deployment/installer/agent/windows/default/latest?arch=x86`

#### Scenario: Windows file permissions handling

- **GIVEN** the download succeeds on Windows
- **WHEN** `DownloadInstaller` processes the temp file
- **THEN** no `chmod` or permission-setting calls are made (NTFS ACLs are handled by the OS)

### Requirement: Windows Authenticode signature verification

On Windows, `VerifyInstallerSignature` SHALL verify the installer's Authenticode signature using PowerShell, independent of the Linux OpenSSL-based verification.

#### Scenario: Windows Authenticode verification via PowerShell

- **GIVEN** `env.OS == "windows"`
- **AND** `--no-verify-signature` is NOT passed
- **AND** `powershell.exe` is available on `PATH`
- **WHEN** `VerifyInstallerSignature(env, installerPath, false)` runs
- **THEN** it invokes `powershell.exe -NoProfile -NonInteractive -Command "(Get-AuthenticodeSignature '<installerPath>').Status"`
- **AND** checks that the trimmed output equals `Valid`
- **AND** returns nil on match

#### Scenario: Missing PowerShell on Windows

- **GIVEN** `env.OS == "windows"`
- **AND** `--no-verify-signature` is NOT passed
- **AND** `powershell.exe` is not on `PATH`
- **WHEN** `VerifyInstallerSignature` runs
- **THEN** it returns an error: `"powershell.exe is required for signature verification on Windows. Pass --no-verify-signature to skip."`

#### Scenario: Invalid Authenticode status

- **GIVEN** `env.OS == "windows"`
- **AND** the PowerShell command returns `NotSigned` or `HashMismatch`
- **WHEN** `VerifyInstallerSignature` processes the output
- **THEN** it returns an error including the status string (e.g. `"Authenticode signature status: NotSigned"`)

#### Scenario: Skip flag honored on Windows

- **GIVEN** `env.OS == "windows"`
- **AND** `--no-verify-signature` is passed
- **WHEN** `VerifyInstallerSignature(env, installerPath, true)` runs
- **THEN** it returns nil without invoking PowerShell or checking the signature

#### Scenario: Non-Windows OS skips to normal verification

- **GIVEN** `env.OS == "linux"` or `env.OS == "other"`
- **WHEN** `VerifyInstallerSignature` runs
- **THEN** it does not attempt PowerShell invocation
- **AND** continues with the appropriate verification for that OS (Linux: OpenSSL, others: skip)

### Requirement: Windows Authenticode debug logging

`VerifyInstallerSignature` on Windows SHALL emit structured debug and verbose logs matching the Linux verification pattern.

#### Scenario: PowerShell lookup logged

- **GIVEN** `--debug` is enabled
- **WHEN** `VerifyInstallerSignature` calls `exec.LookPath("powershell.exe")`
- **THEN** stderr contains a Debug line with message `"powershell lookup"` and keys `path`, `found`

#### Scenario: Authenticode check logged

- **GIVEN** `--debug` is enabled
- **WHEN** `VerifyInstallerSignature` invokes the PowerShell command
- **THEN** stderr contains a Debug line with message `"windows authenticode check"` and keys `status`, `path`

#### Scenario: Windows verification success at Verbose

- **GIVEN** `-v` is enabled
- **AND** the Authenticode status is `Valid`
- **WHEN** `VerifyInstallerSignature` returns nil
- **THEN** stderr contains a Verbose line with message `"installer signature verified"` (same milestone as Linux)

### Requirement: Windows privilege check

On Windows, `CheckPrivilege` SHALL verify the process token belongs to the BUILTIN\Administrators group before any network work.

#### Scenario: Process token admin check

- **GIVEN** the process is running with administrator elevation
- **WHEN** `CheckPrivilege()` runs on Windows
- **THEN** it returns nil
- **AND** a Debug log confirms `"privilege check"` with `privileged == true`

#### Scenario: Non-admin process rejected

- **GIVEN** the process is running without administrator rights
- **WHEN** `CheckPrivilege()` runs on Windows
- **THEN** it returns an error: `"This command requires administrator privileges. Please run as an administrator."`
- **AND** a Debug log confirms `privileged == false, os == "windows"`

#### Scenario: Privilege check uses test-injectable helper

- **GIVEN** a test needs to mock the admin check
- **WHEN** the test sets a package-level `isAdmin` variable to a custom function
- **THEN** `CheckPrivilege()` uses that function instead of the real Windows API call
- **AND** no elevated privileges are required at test time

### Requirement: Windows install command construction

The Windows install command SHALL be built distinctly from the Linux version, with no `/bin/sh` prefix and with `--quiet` positioned as the first flag (if present).

#### Scenario: Windows command argv order

- **GIVEN** `env.OS == "windows"`, `cfg.MonitoringMode == "fullstack"`, `opts.Quiet == true`
- **WHEN** `BuildInstallCommand(env, cfg, opts, installerPath)` runs
- **THEN** the argv is `[installerPath, "--quiet", "--set-monitoring-mode=fullstack", "--set-app-log-content-access=true", ...]`
- **AND** `--quiet` is the first and only flag before configuration flags

#### Scenario: Windows command without shell wrapper

- **GIVEN** `env.OS == "windows"`
- **WHEN** `BuildInstallCommand` constructs the command
- **THEN** the first element is the installer `.exe` path directly
- **AND** there is NO `/bin/sh` prefix
- **AND** there is NO `sudo` prefix

#### Scenario: Windows no --set-server flag

- **GIVEN** `env.OS == "windows"`
- **WHEN** `BuildInstallCommand` runs
- **THEN** the argv does NOT include `--set-server=...`
- **AND** only includes `--set-monitoring-mode`, `--set-app-log-content-access`, and optionally `--set-host-group`

### Requirement: Windows execution uses native UAC

The Windows execution path relies on the native Windows UAC mechanism embedded in the installer `.exe`.

#### Scenario: Windows direct execution without sudo

- **GIVEN** `env.OS == "windows"`
- **WHEN** `ExecuteInstallCommand(argv, false, false)` runs
- **THEN** the subprocess is invoked with the argv directly
- **AND** no `sudo` prefix is prepended
- **AND** the installer `.exe` triggers UAC elevation if needed

#### Scenario: Windows UAC elevation requested by installer

- **GIVEN** the installer `.exe` is run on a non-admin account
- **WHEN** the Windows UAC elevation dialog appears
- **THEN** the installer request elevation via standard Windows mechanisms
- **AND** the executing process does not attempt to manage elevation

### Requirement: Windows integration test scenarios

Integration tests for Windows-specific paths SHALL cover the happy path with Windows-specific assertions.

#### Scenario: Windows happy-path test

- **GIVEN** a test environment with mocked tenant API, token minting, and installer download
- **AND** the download returns a `.exe` body
- **AND** Authenticode verification is mocked to return `Valid` status
- **AND** the installer command has `--quiet` as the first flag (after the executable)
- **WHEN** the test runs `InstallOneAgentV2` with `env.OS == "windows"`
- **THEN** no `chmod` is called on the downloaded installer
- **AND** no `sudo` prefix is added to the command argv
- **AND** the temp file name ends in `.exe`

#### Scenario: Windows path normalization

- **GIVEN** a Windows temp file path with backslashes
- **WHEN** the installed binary path is used in the command argv
- **THEN** the path is correctly formatted for the Windows subprocess execution
- **AND** forward slashes are tolerated by the Windows API

### Requirement: Windows feature gate and testing hooks

Windows-specific code paths SHALL be testable without requiring elevated privileges or Windows-only tools at test time.

#### Scenario: Admin check mockable in tests

- **GIVEN** a unit test that needs to verify privilege-check behavior
- **WHEN** the test sets `isAdmin = func() bool { return false }`
- **THEN** `CheckPrivilege()` uses that mock
- **AND** returns the privilege-required error without calling the real Windows API

#### Scenario: PowerShell mocked in tests

- **GIVEN** a unit test that needs to verify Authenticode verification behavior
- **WHEN** the test temporarily replaces `powershell.exe` in `$PATH` with a fake script
- **THEN** `VerifyInstallerSignature` invokes the fake script
- **AND** returns success/failure based on the fake script's output
- **AND** no real PowerShell is required

### Requirement: Windows build tag consistency

Windows-specific code shall use consistent Go build tags and file naming conventions.

#### Scenario: Windows-specific files use correct build tags

- **GIVEN** implementation files like `preflight_windows.go` exist
- **WHEN** examined for build tags
- **THEN** they contain `//go:build windows` at the top
- **AND** Unix-specific counterparts (`preflight_unix.go`) contain `//go:build !windows`

#### Scenario: Shared code handles both platforms

- **GIVEN** functions like `CheckPrivilege` that have platform-specific implementations
- **WHEN** the shared `oneagent_v2.go` calls the function
- **THEN** the function dispatch is automatic via build tags
- **AND** no runtime platform checks are needed within the function call site
