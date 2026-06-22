## Context

The OneAgent V2 package (`pkg/installer/oneagent/`) is well-structured for testing: `oneAgentInstallDir`, `needsSudoFn`, and `sudoPathFn` are all package-level `var`s that tests can redirect. The install path (`InstallOneAgentV2`) and platform-specific detection and install-agent helpers have good unit test coverage in `oneagent_test.go`, `detect_unix_test.go`, `install_agent_test.go`, and `uninstall_unix_test.go`.

What's missing:

1. **Uninstall orchestration tests** — `UninstallOneAgentV2` in `uninstall.go` has no tests. Its three branches (not-installed early return, dry-run, confirmation flow) are entirely untested.
2. **Lifecycle integration tests** — no test exercises install state detection followed by uninstall in a single test, so state-transition bugs between the two would go undetected.

## Goals / Non-Goals

**Goals:**
- Test all branches of `UninstallOneAgentV2`: not-installed, dry-run, confirm-accept, confirm-decline, AutoConfirm.
- Test the lifecycle state transition: post-install dir exists → `oneAgentInstalled()` true → uninstall → dir gone.
- Use real stub shell scripts and real file I/O — e2e style, matching the OTel Collector integration test pattern.
- All new tests in `pkg/installer/oneagent/`, same package, using existing test helpers.

**Non-Goals:**
- Testing the V1 (`installer.UninstallOneAgent`) path.
- CLI command-level tests (`cmd/` layer) — no cmd-layer tests exist anywhere in the codebase.
- Windows lifecycle tests — Windows detection and uninstall paths require WMI/PowerShell, out of scope.
- Including the OneAgent e2e test in the default `make test` run — it is build-tagged `integration` and requires a live tenant, Linux, and root.
- Multi-host or parallel OneAgent e2e cases — there is one agent per host, so the e2e test is a single serial case (unlike the parallel OTel language table).

## Decisions

**Decision 1: Introduce `runUninstallFn` and `runCommandFn` as injectable vars**

Add two injectable vars to `uninstall_unix.go` (and the Windows equivalent):

- `var runUninstallFn = runUninstall` — allows `UninstallOneAgentV2` to be decoupled from the platform implementation, and makes future Windows-specific testing possible without a real MSI.
- `var runCommandFn = func(name string, args ...string) error { return installer.RunCommand(name, args...) }` — allows `runUninstall()` tests to capture argv without spawning a real process when verifying sudo path construction (tasks 3.2/3.3).

Update `UninstallOneAgentV2` in `uninstall.go` to call `runUninstallFn()`, and update `runUninstall()` in `uninstall_unix.go` to call `runCommandFn(...)` instead of `installer.RunCommand(...)` directly.

**Decision 2: Orchestration and lifecycle tests use real stub shell scripts (e2e style)**

All tests that exercise the accept/execute path use a real `#!/bin/sh\nexit 0` stub written to the temp install dir. The full chain runs without mocking intermediate functions: `oneAgentInstalled()` → `printPlan()` → `runUninstallFn()` → `runCommandFn()` (real subprocess) → `cleanupInstallDir()`. Tests verify filesystem state outcomes (dir present/absent) rather than spy-call counts.

This aligns with the OTel Collector integration test pattern, where tests exercise real file I/O with temp dir fixtures. The `runUninstallFn` injectable exists in production code but is NOT injected in orchestration tests — the real function runs.

Only `runCommandFn` is injected in tasks 3.2/3.3 where the goal is to verify subprocess argv with and without sudo, since real sudo cannot be assumed in CI.

Behavioral assertions used instead of spy-call counts:
- Accept path: assert install dir is gone after the call (removed by `cleanupInstallDir`).
- Decline path: assert install dir still exists.
- Script failure path: assert non-nil error returned (stub script exits 1).

**Decision 3: New test file `uninstall_test.go` for cross-platform orchestration**

Orchestration tests (not-installed, dry-run, decline, accept) are placed in a new `uninstall_test.go` with no build tag. All tests that depend on Unix install dir detection use `skipNonLinux(t)`, matching the existing pattern. Unix-specific tests (sudo path, cleanup) go in `uninstall_unix_test.go`.

**Decision 4: Real-tenant e2e test mirrors the OTel e2e pattern, adapted for a system-wide agent**

The e2e test (`test/e2e/oneagent_test.go`, `//go:build integration`) reuses the existing `test/integration` harness: `SetupIntegration(t)` for tenant credentials/client, `installer.AutoConfirm = true` to drive the uninstall prompt, and the `grail` poller for the success signal. It diverges from the OTel test where OneAgent's nature forces it:

- **Linux + root only.** `detectRuntimeEnvironment()` errors on macOS and a real install writes to `/opt/dynatrace/oneagent`. The test skips unless `runtime.GOOS == "linux"` and `os.Geteuid() == 0` — running as non-root would trigger an interactive sudo prompt that hangs CI.
- **Single serial case, not a parallel table.** There is one agent per host, and install/uninstall mutate global system state, so the test cannot use `t.Parallel()` cases like the OTel language table.
- **Success signal = host appears in Smartscape topology.** A new `grail.RequireHost`/`WaitForHost` helper polls `smartscapeNodes "HOST" | filter name == "<hostname>"`, mirroring `RequireTraces`. The shared polling loop is extracted into `waitForRecords` so the trace and host pollers share one implementation. The host is correlated by `os.Hostname()`; the install also sets `HostGroup = env.TestID` to aid manual debugging.
- **Mandatory teardown.** Unlike the OTel test (which just removes an app dir), leaving OneAgent installed would keep the runner permanently monitored, so uninstall is both an asserted step and a `t.Cleanup` safety net.
- **Cleanup assertion against the literal path.** `oneAgentInstalled()` and `oneAgentInstallDir` are unexported, so the external `e2e_test` package asserts on the well-known `/opt/dynatrace/oneagent` path via `os.Stat` instead.

## Risks / Trade-offs

- **Real subprocess execution** → stub scripts must be created before calling `runUninstall()`. Tests that skip creating the script test the "missing script" error path, not the success path.
- **Stub script on Linux only** → tests skip on Windows and macOS via `skipNonLinux(t)`. Acceptable: agent lifecycle tests only make sense on Linux.
- **`runUninstallFn` and `runCommandFn` are small production code changes** despite the "test-only" framing of the proposal. Both follow established patterns and add negligible risk.
- **Host correlation by `os.Hostname()` is an assumption.** OneAgent may register the host under a FQDN or cloud-provided name that differs from `os.Hostname()`. If the topology query misses, the `HostGroup = env.TestID` tag gives a fallback correlation handle to refine the DQL against a real tenant. This is acceptable for a build-tagged e2e test that only runs interactively/CI with a live tenant.
- **Host registration latency.** A freshly installed agent can take minutes to connect and appear in Smartscape; the poller uses a 5-minute timeout with a 20s interval (vs. 3 minutes for OTel traces). Slower and more sensitive to tenant load than the hermetic tests.
- **Real signature/CA fetch.** The e2e install performs real installer download and signature verification (CA fetched from ca.dynatrace.com), so it depends on outbound network beyond the tenant itself.
