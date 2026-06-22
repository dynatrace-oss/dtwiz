# OneAgent E2E Lifecycle

## ADDED Requirements

### Requirement: Real-tenant install-then-uninstall lifecycle

A build-tagged (`integration`) end-to-end test SHALL install OneAgent against a real Dynatrace tenant using `InstallOneAgentV2`, confirm the host registers in Smartscape topology via a Grail/DQL query, then call `UninstallOneAgentV2` and verify the install directory is removed. The test SHALL reuse the shared `test/integration` harness (`SetupIntegration`, `grail`) so it stays aligned with the OTel auto-instrumentation e2e test.

The test SHALL run only on Linux and only when the process is root, and SHALL skip otherwise (running as non-root would trigger an interactive sudo prompt that hangs CI). It SHALL NOT be part of the default `make test` run.

#### Scenario: Host registers after a real install and clears after uninstall

- **GIVEN** the test runs on Linux as root with the `integration` build tag
- **AND** `TEST_DT_ENVIRONMENT` and `TEST_DT_PLATFORM_TOKEN` are set
- **AND** `installer.AutoConfirm` is true
- **WHEN** `InstallOneAgentV2(env.Client, InstallOptions{HostGroup: env.TestID, Quiet: true})` is called
- **THEN** it returns nil
- **AND** the install directory `/opt/dynatrace/oneagent` exists
- **AND** within the poll timeout, a `HOST` node named after the machine hostname appears in Smartscape topology
- **WHEN** `UninstallOneAgentV2(UninstallOptions{})` is called
- **THEN** it returns nil
- **AND** `/opt/dynatrace/oneagent` no longer exists

#### Scenario: Skipped when prerequisites are not met

- **GIVEN** the test process is not running on Linux, OR is not running as root
- **WHEN** `TestOneAgentLifecycle` runs
- **THEN** it is skipped with a message explaining the Linux/root requirement
- **AND** no install or uninstall is attempted

#### Scenario: Teardown safety net

- **GIVEN** OneAgent was installed during the test
- **WHEN** a later assertion fails before the explicit uninstall runs
- **THEN** a registered `t.Cleanup` attempts `UninstallOneAgentV2` so the runner is not left permanently monitored

---

### Requirement: Grail host-topology poller

The `test/integration/grail` package SHALL expose `RequireHost` and `WaitForHost` helpers that poll the DQL endpoint for a `HOST` Smartscape node by name, mirroring the existing `RequireTraces`/`WaitForTraces` service helpers. The trace and host pollers SHALL share a single polling loop implementation.

#### Scenario: Host poller returns once the host appears

- **GIVEN** a `*client.Client` for a tenant where the host has registered
- **WHEN** `WaitForHost(ctx, c, hostName, opts...)` is called
- **THEN** it polls `smartscapeNodes "HOST" | filter name == "<hostName>"` until at least one record is returned or the timeout elapses
- **AND** `RequireHost` fatals the test if the poll errors or returns no records
