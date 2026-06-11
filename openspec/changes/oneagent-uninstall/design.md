# Design

## Context

`pkg/installer/oneagent_uninstall.go` uses `color.New(color.FgMagenta, color.Bold)` for headers, raw `fmt.Println` for output, and does no pre-check for an existing installation. It predates the V2 package (`pkg/installer/oneagent/`) which introduced `display.PrintStatusLine`, `logger.Debug`/`logger.Verbose`, build-tagged platform files, and `oneAgentInstalled()`. Uninstall is the only OneAgent operation not yet in the V2 package.

## Goals / Non-Goals

**Goals:**

- `UninstallOneAgentV2` calls `oneAgentInstalled()` first; returns a clear "not installed" error instead of a mid-script failure.
- Always shows the plan before acting — same UX as `UninstallOtelCollector`: print what will happen, dry-run returns early with "no changes made", otherwise ask Y/n.
- Platform split via `//go:build !windows` / `//go:build windows`, same as `detect_unix.go`/`detect_windows.go` in this package.
- All user-facing output via `display.PrintStatusLine`; no `color.New(...)` calls.
- Diagnostics via `logger.Debug`/`logger.Verbose` only.
- Gated by the existing `OneAgentPoC` flag — no separate uninstall flag.

**Non-Goals:**

- Changing uninstall mechanics — script path, WMI query, and msiexec command match V1 exactly.
- API calls or credential resolution — uninstall is fully local, no `*client.Client` parameter.
- Quiet mode — uninstall is a destructive one-shot operation; confirmation is always required.

## Decisions

### 1. Pre-check via oneAgentInstalled()

`UninstallOneAgentV2` calls `oneAgentInstalled()` (defined in `detect_unix.go`/`detect_windows.go`) as its first step. This returns a clear "not installed" error instead of letting the uninstall script or WMI query fail with a less clear message.

### 2. Show plan before acting

`printPlan()` is called before the dry-run check and the confirmation prompt. This matches `UninstallOtelCollector` where the user always sees what will happen before being asked to confirm or before `--dry-run` short-circuits. The platform-specific `printPlan()` functions (in `uninstall_unix.go` / `uninstall_windows.go`) output the plan content only; `uninstall.go` owns the dry-run early return and the confirmation prompt.

### 3. Platform split via build tags

`printPlan()` and `runUninstall()` are defined once per platform in separate build-tagged files. `uninstall.go` (no build tag) calls them and owns the shared flow: detection check, plan print, dry-run branch, confirmation prompt, and success status line. This mirrors `install_agent.go` (no tag) + `detect_unix.go`/`detect_windows.go` (per-platform).

### 4. Reuse needsSudoFn and sudoPathFn

`install_agent.go` declares `needsSudoFn = installer.NeedsSudo` and `sudoPathFn` as package-level vars so they can be overridden in tests. `uninstall_unix.go` uses them directly. `cleanupInstallDir` also calls `sudoPathFn()` for the sudo path, consistent with `runUninstall`.

### 5. cleanupInstallDir in uninstall_unix.go

V1's `removeResidualDir` is unexported in `pkg/installer`. V2 re-implements the same logic as `cleanupInstallDir` in `uninstall_unix.go` (absent path → nil, non-directory → skip, sudo → `sudo rm -rf`, no sudo → `os.RemoveAll`). The duplication is ~30 lines; exporting from V1 would add to the parent package's exported API for no benefit.

### 6. Confirmation and ErrInstallCancelled

On decline, `UninstallOneAgentV2` returns `installer.ErrInstallCancelled` (matching `InstallOneAgentV2`). `cmd/uninstall.go` treats it as a clean exit (`errors.Is` guard, same as `uninstallAWSLambdaCmd` and `uninstallOtelCmd`).

### 7. Feature flag

V2 is gated by the existing `OneAgentPoC` flag (`--oneagent-poc` / `DTWIZ_ONEAGENT_POC`). Install and uninstall share the same gate.

### 8. Logging

| Stage | Level | Message | Keys |
|---|---|---|---|
| Uninstall entry | Debug | `"starting oneagent uninstall"` | `dry_run` |
| Agent detected | Debug | `"existing OneAgent detected"` | — |
| Script check | Debug | `"checking uninstall script"` | `path` |
| Privilege check | Debug | `"privilege check"` | `needs_sudo` |
| Sudo path | Debug | `"using sudo"` | `path` |
| Script argv | Debug | `"running uninstall script"` | `argv` |
| Script done | Verbose | `"uninstall script completed"` | — |
| Cleanup: absent | Debug | `"residual directory already absent"` | `path` |
| Cleanup: non-dir | Debug | `"residual path is not a directory, skipping"` | `path` |
| Cleanup: start | Debug | `"removing residual install directory"` | `path`, `needs_sudo` |
| Cleanup: done | Verbose | `"removed residual install directory"` | `path` |
| Cleanup: warn | Warn | `"could not remove residual directory"` | `path`, `error` |
| State dir note | Debug | `"state directory preserved"` | `path` |
| WMI start | Debug | `"running WMI uninstall"` | `method` |
| WMI done | Verbose | `"WMI uninstall completed"` | — |
