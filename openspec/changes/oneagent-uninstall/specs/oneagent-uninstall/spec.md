# OneAgent Uninstall V2

## ADDED Requirements

### Requirement: Pre-check before uninstall

`UninstallOneAgentV2` SHALL call `oneAgentInstalled()` before any other operation. If OneAgent is not detected, it SHALL return an error with the message `"OneAgent is not installed — nothing to uninstall"` without printing a plan or prompting the user.

#### Scenario: OneAgent not installed — returns clear error

- **GIVEN** `oneAgentInstalled()` returns false
- **WHEN** `UninstallOneAgentV2` is called
- **THEN** it returns an error containing `"not installed"`
- **AND** no plan is printed
- **AND** no confirmation prompt is shown

---

### Requirement: Plan is always shown before acting

`UninstallOneAgentV2` SHALL call `printPlan()` after the installed check and before the dry-run check or confirmation prompt. The plan output is platform-specific:

- Linux: the uninstall script path and whether `sudo` is required
- Windows: the WMI method description

#### Scenario: Plan is shown before dry-run returns

- **GIVEN** `opts.DryRun == true`
- **AND** `oneAgentInstalled()` returns true
- **WHEN** `UninstallOneAgentV2` is called
- **THEN** the plan is printed before the dry-run status line

#### Scenario: Plan is shown before the confirmation prompt

- **GIVEN** `opts.DryRun == false`
- **AND** `oneAgentInstalled()` returns true
- **WHEN** `UninstallOneAgentV2` is called
- **THEN** the plan is printed before the `"Proceed with OneAgent uninstall?"` prompt

---

### Requirement: Dry-run shows plan and returns without changes

When `opts.DryRun == true`, `UninstallOneAgentV2` SHALL print the plan via `printPlan()`, emit a `display.PrintStatusLine("dry-run", "no changes made", ...)` status line, and return nil. No subprocess SHALL be spawned and no files SHALL be modified.

#### Scenario: Dry-run on Linux

- **GIVEN** `opts.DryRun == true`
- **AND** `oneAgentInstalled()` returns true (Linux host)
- **WHEN** `UninstallOneAgentV2` is called
- **THEN** stdout contains the uninstall script path
- **AND** stdout contains `"no changes made"` via `display.PrintStatusLine`
- **AND** `runUninstall()` is NOT called

#### Scenario: Dry-run on Windows

- **GIVEN** `opts.DryRun == true`
- **AND** `oneAgentInstalled()` returns true (Windows host)
- **WHEN** `UninstallOneAgentV2` is called
- **THEN** stdout contains `WMI product lookup + msiexec /x`
- **AND** stdout contains `"no changes made"` via `display.PrintStatusLine`

---

### Requirement: Confirmation before uninstall

When `opts.DryRun == false` and OneAgent is detected, `UninstallOneAgentV2` SHALL prompt the user via `installer.ConfirmProceed` after printing the plan. The prompt honours `installer.AutoConfirm`. On decline, it SHALL emit a `display.PrintStatusLine` "uninstall cancelled" line and return `installer.ErrInstallCancelled`.

#### Scenario: User confirms — uninstall proceeds

- **GIVEN** `oneAgentInstalled()` returns true
- **AND** the user answers `Y` (or presses Enter)
- **WHEN** `UninstallOneAgentV2` runs
- **THEN** `runUninstall()` is called

#### Scenario: User declines — uninstall cancelled

- **GIVEN** `oneAgentInstalled()` returns true
- **AND** the user answers `n`
- **WHEN** `UninstallOneAgentV2` runs
- **THEN** stdout contains `"uninstall cancelled"` via `display.PrintStatusLine`
- **AND** `runUninstall()` is NOT called
- **AND** `UninstallOneAgentV2` returns `installer.ErrInstallCancelled`

---

### Requirement: Linux uninstall runs the bundled script

On Linux, `runUninstall` SHALL execute `/opt/dynatrace/oneagent/agent/uninstall.sh`. When the process is non-root (`needsSudoFn()` returns true), the resolved sudo binary path SHALL be prepended. Missing sudo is a hard error. After the script completes, the residual install directory SHALL be cleaned up via `cleanupInstallDir`.

#### Scenario: Linux non-root invocation prepends sudo

- **GIVEN** `needsSudoFn()` returns true
- **WHEN** `runUninstall()` runs
- **THEN** the subprocess argv begins with the resolved sudo binary path

#### Scenario: Linux root invocation skips sudo

- **GIVEN** `needsSudoFn()` returns false
- **WHEN** `runUninstall()` runs
- **THEN** the subprocess argv begins directly with the uninstall script path

#### Scenario: Uninstall script not found — clear error

- **GIVEN** `/opt/dynatrace/oneagent/agent/uninstall.sh` does not exist
- **WHEN** `runUninstall()` runs
- **THEN** it returns an error whose message contains the script path

---

### Requirement: Residual install directory cleanup

After the Linux uninstall script runs, `cleanupInstallDir` SHALL remove the stub directory left behind. Absent paths and non-directory paths SHALL be silently skipped (nil return). Removal errors SHALL be returned to the caller.

#### Scenario: Absent path — nil returned

- **GIVEN** the path does not exist
- **WHEN** `cleanupInstallDir` is called
- **THEN** it returns nil

#### Scenario: Path is a file — skipped silently

- **GIVEN** the path exists and is a regular file
- **WHEN** `cleanupInstallDir` is called
- **THEN** the file is NOT deleted and nil is returned

#### Scenario: Empty directory — removed

- **GIVEN** the path is an empty directory
- **WHEN** `cleanupInstallDir` is called
- **THEN** the directory is removed and nil is returned

#### Scenario: Non-empty directory — removed recursively

- **GIVEN** the path is a non-empty directory tree
- **WHEN** `cleanupInstallDir` is called
- **THEN** the entire tree is removed and nil is returned

---

### Requirement: Windows uninstall uses WMI + msiexec

On Windows, `runUninstall` SHALL use PowerShell to query WMI for the Dynatrace OneAgent product and uninstall it via `msiexec /x`. msiexec SHALL be invoked via `Start-Process -Verb RunAs -Wait -PassThru` so that PowerShell blocks until the elevated msiexec process finishes, ensuring the success message is only shown after the uninstall actually completes. The uninstall log SHALL be written to `uninstall.log` in the current directory.

#### Scenario: WMI query returns no product

- **GIVEN** the WMI query finds no Dynatrace OneAgent product
- **WHEN** `runUninstall()` runs
- **THEN** it returns an error

#### Scenario: msiexec completes before success is reported

- **GIVEN** OneAgent is installed
- **AND** the user confirms the uninstall prompt
- **WHEN** `runUninstall()` returns nil
- **THEN** msiexec has fully completed (not merely spawned)
- **AND** `"OneAgent uninstalled successfully"` is printed only after `runUninstall()` returns

---

### Requirement: V2 is the default path

`dtwiz uninstall oneagent` always calls `oneagent.UninstallOneAgentV2`. There is no feature flag gate and no V1 fallback.

#### Scenario: ErrInstallCancelled is a clean exit

- **GIVEN** the user declines the confirmation prompt
- **WHEN** `dtwiz uninstall oneagent` returns
- **THEN** the CLI exits with code 0
