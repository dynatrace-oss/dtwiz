# OneAgent Integration Tests

## ADDED Requirements

### Requirement: Uninstall returns early when OneAgent is absent

`UninstallOneAgentV2` SHALL check `oneAgentInstalled()` before printing a plan or prompting.

#### Scenario: Not installed

- **GIVEN** `oneAgentInstallDir` points to a missing path
- **AND** `oneagentctl` is not on `PATH`
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` runs
- **THEN** it returns an error containing `"not installed"`
- **AND** stdout is empty

---

### Requirement: Dry-run prints the plan and keeps files

When OneAgent is installed and `DryRun` is true, `UninstallOneAgentV2` SHALL print the plan, print `"no changes made"`, and not uninstall anything.

#### Scenario: Dry-run

- **GIVEN** `oneAgentInstallDir` contains `agent/uninstall.sh`
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: true})` runs
- **THEN** it returns nil
- **AND** stdout contains `"no changes made"`
- **AND** the install dir still exists

---

### Requirement: Confirmation controls uninstall

`UninstallOneAgentV2` SHALL prompt before uninstalling unless `installer.AutoConfirm` is true.

#### Scenario: User declines

- **GIVEN** OneAgent is installed
- **AND** stdin contains `"n\n"`
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` runs
- **THEN** it returns `installer.ErrInstallCancelled`
- **AND** stdout contains `"uninstall cancelled"`
- **AND** the install dir still exists

#### Scenario: User accepts

- **GIVEN** OneAgent is installed with a stub `agent/uninstall.sh` that exits 0
- **AND** stdin contains `"y\n"`
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` runs
- **THEN** it returns nil
- **AND** the install dir is removed

#### Scenario: AutoConfirm skips the prompt

- **GIVEN** OneAgent is installed with a stub `agent/uninstall.sh` that exits 0
- **AND** `installer.AutoConfirm` is true
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` runs
- **THEN** it returns nil
- **AND** the install dir is removed without reading stdin

---

### Requirement: Uninstall script errors are returned

If the uninstall script exits non-zero, `UninstallOneAgentV2` SHALL return an error and not print the success message.

#### Scenario: Script fails

- **GIVEN** OneAgent is installed with a stub `agent/uninstall.sh` that exits 1
- **AND** `installer.AutoConfirm` is true
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` runs
- **THEN** it returns an error
- **AND** stdout does not contain `"OneAgent uninstalled successfully"`

---

### Requirement: Linux runUninstall uses sudo only when needed

On Linux, `runUninstall` SHALL prepend `sudoPathFn()` to argv only when `needsSudoFn()` is true.

#### Scenario: Sudo needed

- **GIVEN** `agent/uninstall.sh` exists
- **AND** `needsSudoFn` returns true
- **AND** `sudoPathFn` returns a stub path
- **WHEN** `runUninstall()` runs with a `runCommandFn` spy
- **THEN** argv starts with the stub sudo path

#### Scenario: Sudo not needed

- **GIVEN** `agent/uninstall.sh` exists
- **AND** `needsSudoFn` returns false
- **WHEN** `runUninstall()` runs with a `runCommandFn` spy
- **THEN** argv starts with the uninstall script path

#### Scenario: Script missing

- **GIVEN** the install dir exists without `agent/uninstall.sh`
- **WHEN** `runUninstall()` runs
- **THEN** it returns an error containing the script path

---

### Requirement: Install state changes are detectable

`oneAgentInstalled()` SHALL return true when the install dir exists and false after uninstall removes it.

#### Scenario: Install dir exists

- **GIVEN** `oneAgentInstallDir` points to an existing temp dir
- **WHEN** `oneAgentInstalled()` runs
- **THEN** it returns true

#### Scenario: After uninstall

- **GIVEN** OneAgent is installed with a stub `agent/uninstall.sh` that exits 0
- **AND** `installer.AutoConfirm` is true
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` runs
- **THEN** it returns nil
- **AND** `oneAgentInstalled()` returns false
- **AND** the install dir is removed

---

### Requirement: Hermetic lifecycle test covers install then uninstall

A test SHALL install from a fake HTTP server, verify installed state, uninstall, and verify cleared state.

#### Scenario: Fake install then uninstall

- **GIVEN** `oneAgentInstallDir` points to a missing temp path
- **AND** a fake HTTP server serves an installer script that creates `agent/uninstall.sh`
- **AND** `SkipConnectivityCheck` and `NoVerifySignature` are true
- **WHEN** `InstallOneAgentV2(c, InstallOptions{...})` runs
- **THEN** it returns nil
- **AND** `oneAgentInstalled()` returns true
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` runs with `installer.AutoConfirm` true
- **THEN** it returns nil
- **AND** `oneAgentInstalled()` returns false

---

### Requirement: Lifecycle dry-run keeps the install dir

Dry-run uninstall SHALL leave the install dir in place.

#### Scenario: Dry-run keeps files

- **GIVEN** `oneAgentInstallDir` points to an existing temp dir
- **WHEN** `UninstallOneAgentV2(UninstallOptions{DryRun: true})` runs
- **THEN** it returns nil
- **AND** the install dir still exists
- **AND** stdout contains `"no changes made"`

---

### Requirement: Real-tenant e2e covers install, topology, and uninstall

The `integration` e2e test SHALL install OneAgent, wait for the host in Smartscape, uninstall OneAgent, and verify removal.

The test SHALL run on Linux and Windows only. Linux requires root. Other OSes and non-root Linux SHALL skip. It SHALL NOT run in default `make test`.

#### Scenario: Real install then uninstall

- **GIVEN** the test runs with the `integration` build tag on Linux as root or on Windows
- **AND** `TEST_DT_ENVIRONMENT` and `TEST_DT_PLATFORM_TOKEN` are set
- **AND** `installer.AutoConfirm` is true
- **WHEN** `InstallOneAgentV2(env.Client, InstallOptions{HostGroup: env.TestID})` runs
- **THEN** it returns nil
- **AND** the OS-specific install dir exists
- **AND** a `HOST` node named after `os.Hostname()` appears in Smartscape topology
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` runs
- **THEN** it returns nil
- **AND** Linux removes the install dir
- **AND** Windows removes the `Dynatrace OneAgent` service

#### Scenario: Unsupported environment

- **GIVEN** the OS is not Linux or Windows, or Linux is not running as root
- **WHEN** `TestOneAgentLifecycle` runs
- **THEN** it skips before install

#### Scenario: Cleanup after failure

- **GIVEN** OneAgent was installed during the test
- **WHEN** the test fails before explicit uninstall
- **THEN** `t.Cleanup` attempts `UninstallOneAgentV2`

---

### Requirement: Grail host poller waits for a host node

The `test/integration/grail` package SHALL expose `RequireHost` and `WaitForHost` using the shared `waitForRecords` loop.

#### Scenario: Host appears

- **GIVEN** a `*client.Client` for a tenant where the host has registered
- **WHEN** `WaitForHost(ctx, c, hostName, opts...)` runs
- **THEN** it polls `smartscapeNodes "HOST", from: -30m, to: now() | filter name == "<hostName>"`
- **AND** it returns records when at least one match exists
- **AND** `RequireHost` fatals if polling errors or returns no records
