# Proposal

## Why

When `dtwiz install kubernetes` runs on Windows and Helm is not installed, the auto-install path calls `bash -c curl | bash`, which fails immediately because `bash` is not available on Windows. Users see `exec: "bash": executable file not found in %PATH%` and have no path forward.

A second issue: after winget installs Helm, the new binary is added to the Windows registry PATH but not to the current process's PATH (inherited at startup). This means both the install run itself and any subsequent `dtwiz uninstall kubernetes` run cannot find the `helm` binary.

## What Changes

- Replace the Unix-only `installHelm()` implementation with a platform-aware version
- On Windows: attempt installation via `winget install --id Helm.Helm`; if winget is unavailable or fails, return a clear error message with manual installation instructions
- After winget install: refresh the current process PATH from the Windows registry so helm is immediately usable
- Refresh the process PATH from the registry at the start of both `InstallKubernetes` and `UninstallKubernetes` so any previously winget-installed helm is visible across separate dtwiz invocations
- On Unix: keep existing `bash -c curl | bash` behaviour unchanged
- No new commands, flags, or config changes

## Capabilities

### New Capabilities

- `helm-windows-install`: Windows-specific Helm auto-install path using winget with graceful fallback to manual instructions, and process PATH refresh from the Windows registry

### Modified Capabilities

<!-- none — the public interface (installHelm function) does not change; only the internal platform dispatch is added -->

## Impact

- `pkg/installer/kubernetes.go` — `installHelm()` gains a `runtime.GOOS` branch; new `installHelmWindows()` and `refreshWindowsPath()` helpers added
- `pkg/installer/kubernetes_uninstall.go` — calls `refreshWindowsPath()` at startup
- No API surface changes; no new dependencies; no breaking changes
