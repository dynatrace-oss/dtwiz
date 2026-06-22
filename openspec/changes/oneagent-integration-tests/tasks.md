# Tasks: OneAgent Integration Tests

## 1. Make runUninstall injectable (production change)

- [x] 1.1 In `pkg/installer/oneagent/uninstall_unix.go`, declare `var runUninstallFn = runUninstall` and update `UninstallOneAgentV2` in `uninstall.go` to call `runUninstallFn()` instead of `runUninstall()` directly
- [x] 1.2 In `pkg/installer/oneagent/uninstall_windows.go` (if it exists and defines `runUninstall`), apply the same `runUninstallFn` pattern for symmetry
- [x] 1.3 Run `make lint` to confirm no linter errors from the change

## 2. UninstallOneAgentV2 orchestration tests (e2e style)

- [x] 2.1 Create `pkg/installer/oneagent/uninstall_test.go` (no build tag — uses `skipNonLinux` for tests that depend on install dir detection)
- [x] 2.2 Add `TestUninstallOneAgentV2_NotInstalled_ReturnsError`: redirect `oneAgentInstallDir` to nonexistent path + `PATH=""`, call `UninstallOneAgentV2({})`, assert error contains `"not installed"` and stdout is empty
- [x] 2.3 Add `TestUninstallOneAgentV2_DryRun_PrintsPlanAndReturnsNil`: redirect install dir to existing temp dir with stub script, call with `DryRun: true`, assert nil return and stdout contains `"no changes made"`, assert install dir still exists
- [x] 2.4 Add `TestUninstallOneAgentV2_Decline_ReturnsCancelled`: redirect install dir to existing temp dir, `needsSudoFn=false`, stdin `"n\n"`, assert `ErrInstallCancelled`, stdout contains `"uninstall cancelled"`, install dir still exists
- [x] 2.5 Add `TestUninstallOneAgentV2_Accept_RemovesInstallDir`: redirect install dir to temp dir with stub script (exits 0), `needsSudoFn=false`, `AutoConfirm=true`, assert nil return and install dir no longer exists
- [x] 2.6 Add `TestUninstallOneAgentV2_AutoConfirm_SkipsPrompt`: same as 2.5 but use `AutoConfirm=true` with no stdin wired up — verify no hang and install dir removed
- [x] 2.7 Add `TestUninstallOneAgentV2_ScriptFailure_PropagatesError`: stub script exits 1, `needsSudoFn=false`, `AutoConfirm=true`, assert non-nil error returned and stdout does NOT contain success message

## 3. Linux runUninstall unit tests

- [x] 3.1 In `pkg/installer/oneagent/uninstall_unix_test.go`, add `TestRunUninstall_ScriptMissing_Error`: redirect `oneAgentInstallDir` to a temp dir (no script), call `runUninstall()` directly, assert error contains the expected script path
- [x] 3.2 Add `TestRunUninstall_NeedsNoSudo_ArgvStartsWithScript`: create stub `agent/uninstall.sh` (`#!/bin/sh\nexit 0`) in temp dir, inject `needsSudoFn = false`, capture the argv by injecting a stub `runCommandFn` (if available) or verify via subprocess exit code, assert no sudo prefix
- [x] 3.3 Add `TestRunUninstall_NeedsSudo_ArgvStartsWithSudo`: inject `needsSudoFn = true` and `sudoPathFn = stubSudoPath`, stub out the actual execution (inject `runUninstallFn` or `runCommandFn`), assert argv begins with stub sudo path

## 4. Lifecycle integration tests

- [x] 4.1 Create `pkg/installer/oneagent/lifecycle_test.go` with `//go:build !windows` tag
- [x] 4.2 Add `TestLifecycle_InstallThenUninstall`: call `InstallOneAgentV2` with a fake HTTP server (stub installer that creates the install dir + `agent/uninstall.sh`), `SkipConnectivityCheck=true`, `NoVerifySignature=true`, `needsSudoFn=false`; assert `oneAgentInstalled()` true after install; call `UninstallOneAgentV2` with `AutoConfirm=true`; assert `oneAgentInstalled()` false
- [x] 4.3 Keep `lifecycle_test.go` scoped to the full install→uninstall chain only. State-detection (`oneAgentInstalled()` true/false transitions), full-uninstall-removes-dir, and dry-run-keeps-dir are already covered by the unit tests in `detect_unix_test.go` and `uninstall_test.go`, so no duplicate lifecycle tests are added for them.

## 5. Verification

- [x] 5.1 Run `make test ./pkg/installer/oneagent/...` and confirm all new tests pass
- [x] 5.2 Run `make lint` and confirm no new linter warnings

## 6. Real-tenant e2e lifecycle test

- [x] 6.1 Add a host-topology poller to `test/integration/grail/`: `hostByNameQuery` in `helpers.go`, and `RequireHost`/`WaitForHost` in `host.go`, sharing a `waitForRecords` core extracted from `WaitForTraces`
- [x] 6.2 Create `test/e2e/oneagent_test.go` (`//go:build integration`) with `TestOneAgentLifecycle`: skip unless Linux + root; `SetupIntegration(t)`; `installer.AutoConfirm = true`
- [x] 6.3 Install via `InstallOneAgentV2(env.Client, {HostGroup: env.TestID, Quiet: true})`; assert nil error and `/opt/dynatrace/oneagent` exists
- [x] 6.4 Poll `grail.RequireHost(t, env.Client, os.Hostname(), WithTimeout(5m), WithInterval(20s))` and assert the host registered
- [x] 6.5 Uninstall via `UninstallOneAgentV2({})`; assert nil error and `/opt/dynatrace/oneagent` no longer exists; register a `t.Cleanup` uninstall safety net
- [x] 6.6 Confirm `go build -tags=integration ./test/...` and `go vet -tags=integration ./test/...` pass
