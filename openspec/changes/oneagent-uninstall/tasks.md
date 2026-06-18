# OneAgent Uninstall V2 Tasks

## 1. Core uninstall entry point

Create `pkg/installer/oneagent/uninstall.go` with `UninstallOptions` and `UninstallOneAgentV2`.

**Files:** `pkg/installer/oneagent/uninstall.go` (new)

- [x] 1.1 Define `UninstallOptions struct { DryRun bool }`
- [x] 1.2 Implement `UninstallOneAgentV2(opts UninstallOptions) error`:
  - Call `oneAgentInstalled()` first; return clear error if false
  - Call `printPlan()` — always show what will happen before acting
  - If `opts.DryRun`: emit `display.PrintStatusLine("dry-run", "no changes made", ...)` and return nil
  - Call `installer.ConfirmProceed`; return `installer.ErrInstallCancelled` on decline
  - Call `runUninstall()`; emit `display.PrintStatusLine("result", "OneAgent uninstalled successfully", ...)` on success

## 2. Unix uninstall

Create `pkg/installer/oneagent/uninstall_unix.go` (`//go:build !windows`).

**Files:** `pkg/installer/oneagent/uninstall_unix.go` (new)

- [x] 2.1 Define `stateDir` const and `uninstallScriptPath()` — derive the script path from `oneAgentInstallDir` (declared in `detect_unix.go`) via `filepath.Join` so detection and uninstall share one install root
- [x] 2.2 Implement `printPlan()`: print script path and sudo note (if `needsSudoFn()` is true); no dry-run prefix, no status line — those are in `uninstall.go`
- [x] 2.3 Implement `runUninstall()`:
  - Stat the uninstall script; return clear error if absent
  - Build argv, prepend sudo path via `sudoPathFn()` when `needsSudoFn()` returns true
  - `display.PrintStatusLine("uninstall", "running uninstall script...", ...)` then `installer.RunCommand`
  - Call `cleanupInstallDir(oneAgentInstallDir, ...)`; warn (not error) on failure
  - Print note if state directory is present after uninstall
- [x] 2.4 Implement `cleanupInstallDir(path string, needsSudo bool) error`: absent → nil, non-directory → skip, sudo → resolve path via `sudoPathFn()` then `installer.RunCommand(sudoPath, "rm", "-rf", path)`, no sudo → `os.RemoveAll`

## 3. Windows uninstall

Create `pkg/installer/oneagent/uninstall_windows.go` (`//go:build windows`).

**Files:** `pkg/installer/oneagent/uninstall_windows.go` (new)

- [x] 3.1 Implement `printPlan()`: print WMI method description; no status line
- [x] 3.2 Implement `runUninstall()`: status line, PowerShell WMI + msiexec via `installer.RunCommand`, log note

## 4. Wire in cmd

Update `cmd/uninstall.go` to route to V2 when `OneAgentPoC` is enabled.

**Files:** `cmd/uninstall.go`

- [x] 4.1 Import `featureflags` and `oneagent` packages
- [x] 4.2 In `uninstallOneAgentCmd.RunE`: check `featureflags.IsEnabled(featureflags.OneAgentPoC)` and call `oneagent.UninstallOneAgentV2(oneagent.UninstallOptions{DryRun: uninstallDryRun})`
- [x] 4.3 Handle `installer.ErrInstallCancelled` in the V2 branch (return nil)

## 5. Tests

Create `pkg/installer/oneagent/uninstall_unix_test.go` (`//go:build !windows`).

**Files:** `pkg/installer/oneagent/uninstall_unix_test.go` (new)

- [x] 5.1 `TestCleanupInstallDir_PathAbsent` — absent path returns nil
- [x] 5.2 `TestCleanupInstallDir_PathIsFile` — regular file is skipped (not deleted)
- [x] 5.3 `TestCleanupInstallDir_EmptyDir` — empty directory is removed
- [x] 5.4 `TestCleanupInstallDir_NonEmptyDir` — non-empty tree is removed recursively
