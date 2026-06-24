# Tasks: OneAgent Integration Tests

## 1. Make runUninstall injectable

- [x] 1.1 Add `runUninstallFn = runUninstall` on Unix and Windows
- [x] 1.2 Make `UninstallOneAgentV2` call `runUninstallFn()`
- [x] 1.3 Run `make lint`

## 2. UninstallOneAgentV2 orchestration tests

- [x] 2.1 Create `pkg/installer/oneagent/uninstall_test.go`
- [x] 2.2 Test not-installed error and empty stdout
- [x] 2.3 Test dry-run output and preserved install dir
- [x] 2.4 Test decline returns `ErrInstallCancelled`
- [x] 2.5 Test accepted uninstall removes install dir
- [x] 2.6 Test `AutoConfirm` skips stdin
- [x] 2.7 Test script failure returns an error and prints no success message

## 3. Linux runUninstall unit tests

- [x] 3.1 Test missing uninstall script returns a path error
- [x] 3.2 Test no-sudo argv starts with the script path
- [x] 3.3 Test sudo argv starts with the sudo path

## 4. Lifecycle integration tests

- [x] 4.1 Create `pkg/installer/oneagent/lifecycle_test.go` with `//go:build !windows`
- [x] 4.2 Add `TestLifecycle_InstallThenUninstall` with a fake HTTP server and stub installer
- [x] 4.3 Keep lifecycle coverage to the full install-then-uninstall chain

## 5. Verification

- [x] 5.1 Run `make test ./pkg/installer/oneagent/...` and confirm all new tests pass
- [x] 5.2 Run `make lint` and confirm no new linter warnings

## 6. Real-tenant e2e lifecycle test

- [x] 6.1 Add `hostByNameQuery`, `RequireHost`, and `WaitForHost`
- [x] 6.2 Add `test/e2e/oneagent_test.go` with Linux/Windows gating
- [x] 6.3 Install with `HostGroup: env.TestID` and assert install dir exists
- [x] 6.4 Poll `grail.RequireHost` for `os.Hostname()`
- [x] 6.5 Uninstall and assert Linux dir removal or Windows service removal
- [x] 6.6 Confirm `go build -tags=integration ./test/...` and `go vet -tags=integration ./test/...` pass
