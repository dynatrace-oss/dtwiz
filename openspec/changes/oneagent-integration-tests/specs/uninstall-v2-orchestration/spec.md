# UninstallOneAgentV2 Orchestration

## ADDED Requirements

### Requirement: Not-installed check returns early without printing a plan

`UninstallOneAgentV2` SHALL call `oneAgentInstalled()` first. If it returns false, the function SHALL return an error containing `"not installed"` without printing any plan or prompting the user.

#### Scenario: Not installed — error returned, no output

- **GIVEN** `oneAgentInstallDir` is redirected to a nonexistent path
- **AND** `oneagentctl` is not on PATH
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` is called
- **THEN** it returns a non-nil error containing `"not installed"`
- **AND** stdout is empty (no plan lines printed)

---

### Requirement: Plan is printed before dry-run exits

When `DryRun` is true and `oneAgentInstalled()` returns true, `UninstallOneAgentV2` SHALL print the plan (including the uninstall script path) before emitting the dry-run status line, and SHALL return nil without running any subprocess. The install dir SHALL remain on disk.

#### Scenario: Dry-run prints plan then exits cleanly

- **GIVEN** `oneAgentInstallDir` is redirected to an existing temp dir containing `agent/uninstall.sh`
- **AND** `DryRun` is true
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: true})` is called
- **THEN** it returns nil
- **AND** stdout contains `"no changes made"`
- **AND** the install dir still exists on disk

---

### Requirement: User confirmation gates execution

When `DryRun` is false and OneAgent is installed, `UninstallOneAgentV2` SHALL prompt `"Proceed with OneAgent uninstall?"`. On decline it SHALL return `installer.ErrInstallCancelled` and emit `"uninstall cancelled"` without touching the install dir. On accept it SHALL execute the uninstall and remove the install dir.

#### Scenario: User declines — ErrInstallCancelled returned, dir preserved

- **GIVEN** `oneAgentInstallDir` points to an existing temp dir
- **AND** `needsSudoFn` returns false
- **AND** stdin contains `"n\n"`
- **AND** `DryRun` is false
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: false})` is called
- **THEN** it returns `installer.ErrInstallCancelled`
- **AND** stdout contains `"uninstall cancelled"`
- **AND** the install dir still exists on disk

#### Scenario: User accepts — uninstall executes and install dir is removed

- **GIVEN** `oneAgentInstallDir` points to a temp dir containing `agent/uninstall.sh` (stub: `#!/bin/sh\nexit 0`)
- **AND** `needsSudoFn` returns false
- **AND** stdin contains `"y\n"`
- **AND** `DryRun` is false
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: false})` is called
- **THEN** it returns nil
- **AND** the install dir no longer exists on disk

#### Scenario: AutoConfirm bypasses prompt — uninstall executes

- **GIVEN** `oneAgentInstallDir` points to a temp dir containing `agent/uninstall.sh` (stub: `#!/bin/sh\nexit 0`)
- **AND** `needsSudoFn` returns false
- **AND** `installer.AutoConfirm` is true
- **AND** `DryRun` is false
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: false})` is called
- **THEN** it returns nil
- **AND** the install dir no longer exists on disk (no stdin interaction required)

---

### Requirement: Uninstall script failure is propagated as an error

If the uninstall script exits non-zero, `UninstallOneAgentV2` SHALL return an error and SHALL NOT print a success message.

#### Scenario: Script exits non-zero — error returned

- **GIVEN** `oneAgentInstallDir` points to a temp dir containing `agent/uninstall.sh` (stub: `#!/bin/sh\nexit 1`)
- **AND** `needsSudoFn` returns false
- **AND** `installer.AutoConfirm` is true
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: false})` is called
- **THEN** it returns a non-nil error
- **AND** stdout does NOT contain `"OneAgent uninstalled successfully"`

---

### Requirement: Linux runUninstall prepends sudo when needed

On Linux, `runUninstall` (the platform-specific function) SHALL prepend the resolved sudo binary path to the subprocess argv when `needsSudoFn()` returns true, and SHALL NOT prepend sudo when it returns false.

#### Scenario: Non-root invocation — sudo prepended to subprocess argv

- **GIVEN** `oneAgentInstallDir` is redirected to a temp dir containing `agent/uninstall.sh`
- **AND** `needsSudoFn` returns true
- **AND** `sudoPathFn` returns a stub path
- **AND** `runCommandFn` is injected with a spy that records its first argument
- **WHEN** `runUninstall()` is called directly
- **THEN** the recorded first argument equals the stub sudo path

#### Scenario: Root invocation — no sudo in argv

- **GIVEN** `oneAgentInstallDir` is redirected to a temp dir containing `agent/uninstall.sh`
- **AND** `needsSudoFn` returns false
- **AND** `runCommandFn` is injected with a spy that records its first argument
- **WHEN** `runUninstall()` is called directly
- **THEN** the recorded first argument equals the uninstall script path (not a sudo path)

#### Scenario: Uninstall script missing — clear error

- **GIVEN** `oneAgentInstallDir` is redirected to a temp dir without `agent/uninstall.sh`
- **WHEN** `runUninstall()` is called directly
- **THEN** it returns an error whose message contains the expected script path
