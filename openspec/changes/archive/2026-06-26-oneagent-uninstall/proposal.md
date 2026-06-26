# Why

`UninstallOneAgent` in `pkg/installer/oneagent_uninstall.go` was written before the V2 package (`pkg/installer/oneagent/`) existed. The V2 package uses `display.PrintStatusLine` for output and `logger.Debug`/`logger.Verbose` for diagnostics. The uninstall should use the same patterns and follow the same UX as `UninstallOtelCollector`: show the plan first, then ask for confirmation.

## What Changes

- New `UninstallOneAgentV2(opts UninstallOptions) error` in `pkg/installer/oneagent/uninstall.go`.
- Platform split: `uninstall_unix.go` (`//go:build !windows`) and `uninstall_windows.go` (`//go:build windows`), same pattern as `detect_unix.go`/`detect_windows.go`.
- `uninstall_unix.go` reuses `oneAgentInstalled()`, `needsSudoFn`, and `sudoPathFn` from the same package.
- `cmd/uninstall.go` always routes to V2.

## Capabilities

### Modified Capabilities

- `oneagent-uninstall`: V2 path checks `oneAgentInstalled()` before proceeding, always shows the plan before asking for confirmation (or returning early on `--dry-run`), uses `display.PrintStatusLine` for output, and emits `logger.Debug`/`logger.Verbose` for diagnostics.

## Impact

- **New files:** `pkg/installer/oneagent/uninstall.go`, `uninstall_unix.go`, `uninstall_windows.go`, `uninstall_unix_test.go`
- **Modified files:** `cmd/uninstall.go` (`ErrInstallCancelled` handling)
