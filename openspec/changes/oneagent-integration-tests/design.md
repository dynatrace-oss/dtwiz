# Design: OneAgent Integration Tests

## Context

`pkg/installer/oneagent/` already has test hooks for `oneAgentInstallDir`, `needsSudoFn`, and `sudoPathFn`. Existing tests cover many install and detection helpers.

Missing coverage:

1. `UninstallOneAgentV2` branches: not installed, dry-run, prompt accept, prompt decline, AutoConfirm.
2. A single install-then-uninstall lifecycle test.
3. A real-tenant e2e test that proves the host appears in topology.

## Goals / Non-Goals

**Goals:**

- Cover `UninstallOneAgentV2` branch behavior.
- Cover install state before and after uninstall.
- Use temp dirs and stub scripts for hermetic tests.
- Keep package tests in `pkg/installer/oneagent/`.
- Keep real-tenant e2e behind the `integration` build tag.

**Non-Goals:**

- V1 uninstall tests.
- CLI command tests.
- Windows hermetic lifecycle tests. Those tests use Unix stub scripts. The real-tenant e2e test covers Windows.
- Running real-tenant e2e in default `make test`.
- Parallel OneAgent e2e cases. OneAgent is system-wide.

## Decisions

### Decision 1: Make uninstall execution injectable

Add these vars in `uninstall_unix.go` and `uninstall_windows.go`:

- `var runUninstallFn = runUninstall`
- `var runCommandFn = installer.RunCommand`

`UninstallOneAgentV2` calls `runUninstallFn()`. Platform `runUninstall()` functions call `runCommandFn(...)`.

This lets tests check orchestration and argv construction without real uninstall commands.

### Decision 2: Use real stub scripts for Unix tests

Unix accept-path tests create a real `agent/uninstall.sh` in a temp install dir.

Tests assert filesystem state instead of spy counts:

- Accept: install dir is removed.
- Decline: install dir remains.
- Script failure: error is returned and no success message is printed.

Only sudo argv tests inject `runCommandFn`.

### Decision 3: Split test files by scope

- `uninstall_test.go`: orchestration tests, no build tag, with `skipNonLinux` where needed.
- `uninstall_unix_test.go`: Unix `runUninstall` and cleanup tests.
- `lifecycle_test.go`: Unix install-then-uninstall lifecycle test.

### Decision 4: Real-tenant e2e mirrors existing OTel e2e style

`test/e2e/oneagent_test.go` uses:

- `//go:build integration`
- `integration.SetupIntegration(t)`
- `installer.AutoConfirm = true`
- `grail.RequireHost` to wait for topology

OS rules:

- Linux and Windows only.
- Linux requires root.
- Other OSes skip.

Cleanup rules:

- `t.Cleanup` attempts uninstall if install succeeded.
- Linux asserts `/opt/dynatrace/oneagent` is gone.
- Windows asserts the `Dynatrace OneAgent` service is gone.

## Risks

- Stub scripts are Unix-only, so hermetic lifecycle tests skip outside Linux.
- Real-tenant e2e depends on tenant credentials and outbound network.
- Host lookup uses `os.Hostname()`. Some environments may register a different host name.
- Host registration can take minutes, so the e2e poller uses a 5 minute timeout.
