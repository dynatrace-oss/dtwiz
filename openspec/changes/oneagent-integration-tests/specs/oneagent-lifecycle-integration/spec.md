# OneAgent Lifecycle Integration

## ADDED Requirements

### Requirement: Install state is detected after simulated install

After `InstallOneAgentV2` runs (or an install is simulated by creating the install directory), `oneAgentInstalled()` SHALL return true. After `UninstallOneAgentV2` completes successfully, `oneAgentInstalled()` SHALL return false and the install directory SHALL no longer exist.

#### Scenario: Simulated install — installed state detected

- **GIVEN** `oneAgentInstallDir` is redirected to a temporary directory
- **AND** the temporary directory exists (simulating a completed install)
- **WHEN** `oneAgentInstalled()` is called
- **THEN** it returns true

#### Scenario: After uninstall — installed state cleared

- **GIVEN** `oneAgentInstallDir` is redirected to a temporary directory that exists
- **AND** a stub `agent/uninstall.sh` script is present inside it
- **AND** `needsSudoFn` returns false
- **AND** `installer.AutoConfirm` is true
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: false})` is called
- **THEN** it returns nil
- **AND** `oneAgentInstalled()` returns false after the call
- **AND** the install directory no longer exists on disk

---

### Requirement: Full install-then-uninstall lifecycle using real function calls

A test SHALL call `InstallOneAgentV2` with a fake HTTP server that serves a stub installer script, verify that the installed state is detected, then call `UninstallOneAgentV2`, and verify the installed state is cleared — all without any real Dynatrace tenant or real agent binary.

The stub installer script (served by the fake HTTP server) SHALL create the install directory structure, including `agent/uninstall.sh`, so that the subsequent uninstall call can proceed through its real execution path.

#### Scenario: InstallOneAgentV2 → oneAgentInstalled() true → UninstallOneAgentV2 → oneAgentInstalled() false

- **GIVEN** `oneAgentInstallDir` is redirected to a temp dir (initially absent)
- **AND** `needsSudoFn` returns false
- **AND** a fake HTTP server serves a stub installer script that creates the install dir structure at the temp dir
- **AND** `SkipConnectivityCheck` and `NoVerifySignature` are true (to keep the test hermetic)
- **WHEN** `InstallOneAgentV2(c, InstallOptions{...})` is called
- **THEN** it returns nil
- **AND** `oneAgentInstalled()` returns true
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: false})` is called with `AutoConfirm=true`
- **THEN** it returns nil
- **AND** `oneAgentInstalled()` returns false

---

### Requirement: Lifecycle dry-run does not remove the directory

A dry-run uninstall over the full lifecycle SHALL leave the install directory in place and report that no changes were made.

#### Scenario: Full lifecycle — dry-run does not remove the directory

- **GIVEN** `oneAgentInstallDir` is redirected to a temp dir that exists
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: true})` is called
- **THEN** the call returns nil
- **AND** the temp dir still exists after the call
- **AND** stdout contains `"no changes made"`
