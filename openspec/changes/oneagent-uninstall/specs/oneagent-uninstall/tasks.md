# OneAgent Uninstall V2 — Spec Tasks

## 1. UninstallOneAgentV2 entry point

**Files:** `pkg/installer/oneagent/uninstall.go` (new)

- [x] 1.1 Define `UninstallOptions { DryRun bool }`
- [x] 1.2 Pre-check: `oneAgentInstalled()` → error if false
- [x] 1.3 Call `printPlan()` before dry-run check and confirmation
- [x] 1.4 Dry-run branch: `display.PrintStatusLine("dry-run", "no changes made", ...)` → return nil
- [x] 1.5 Confirmation: `installer.ConfirmProceed` → `ErrInstallCancelled` on decline
- [x] 1.6 Call `runUninstall()` and emit success status line on nil return

## 2. Unix printPlan + runUninstall + cleanupInstallDir

**Files:** `pkg/installer/oneagent/uninstall_unix.go` (new), `//go:build !windows`

- [x] 2.1 `stateDir` const + `uninstallScriptPath()` derived from `oneAgentInstallDir` (in `detect_unix.go`) via `filepath.Join`
- [x] 2.2 `printPlan()`: script path, conditional sudo line; no dry-run prefix, no status line
- [x] 2.3 `runUninstall()`: stat script, build argv with sudo when `needsSudoFn()` is true (path via `sudoPathFn()`), `RunCommand`, `cleanupInstallDir`, state dir note
- [x] 2.4 `cleanupInstallDir(path string, needsSudo bool) error`: absent → nil, non-dir → skip, sudo → `sudoPathFn()` + `RunCommand`, else `os.RemoveAll`

## 3. Windows printPlan + runUninstall

**Files:** `pkg/installer/oneagent/uninstall_windows.go` (new), `//go:build windows`

- [x] 3.1 `printPlan()`: WMI method description; no status line
- [x] 3.2 `runUninstall()`: status line, PowerShell WMI + `Start-Process msiexec -Verb RunAs -Wait -PassThru` (blocks until msiexec finishes), log note

## 4. cmd/uninstall.go routing

**Files:** `cmd/uninstall.go`

- [x] 4.1 Import `oneagent` package
- [x] 4.2 Call `oneagent.UninstallOneAgentV2` directly in `uninstallOneAgentCmd.RunE` — V2 is the default, no feature flag check
- [x] 4.3 `errors.Is(err, installer.ErrInstallCancelled)` → return nil

## 5. Tests

**Files:** `pkg/installer/oneagent/uninstall_unix_test.go` (new), `//go:build !windows`

- [x] 5.1 `TestCleanupInstallDir_PathAbsent`
- [x] 5.2 `TestCleanupInstallDir_PathIsFile`
- [x] 5.3 `TestCleanupInstallDir_EmptyDir`
- [x] 5.4 `TestCleanupInstallDir_NonEmptyDir`
