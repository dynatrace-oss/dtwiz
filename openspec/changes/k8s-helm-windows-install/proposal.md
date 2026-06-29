# Proposal

## Why

When `dtwiz install kubernetes` runs on Windows and Helm is not installed, the auto-install path calls `bash -c curl | bash`, which fails immediately because `bash` is not available on Windows. Users see `exec: "bash": executable file not found in %PATH%` and have no path forward.

## What Changes

- Replace the Unix-only `installHelm()` implementation with a platform-aware version
- On Windows: attempt installation via `winget install --id Helm.Helm`; if winget is unavailable or fails, return a clear error message with manual installation instructions
- On Unix: keep existing `bash -c curl | bash` behaviour unchanged
- No new commands, flags, or config changes

## Capabilities

### New Capabilities

- `helm-windows-install`: Windows-specific Helm auto-install path using winget with graceful fallback to manual instructions

### Modified Capabilities

<!-- none — the public interface (installHelm function) does not change; only the internal platform dispatch is added -->

## Impact

- `pkg/installer/kubernetes.go` — `installHelm()` gains a `runtime.GOOS` branch
- New helper `installHelmWindows()` (or split into `kubernetes_windows.go` / `kubernetes_unix.go`)
- No API surface changes; no new dependencies; no breaking changes
