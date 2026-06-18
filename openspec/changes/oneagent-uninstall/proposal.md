# Why

`UninstallOneAgent` in `pkg/installer/oneagent_uninstall.go` was written before the V2 package (`pkg/installer/oneagent/`) existed. The V2 package uses `display.PrintStatusLine` for output and `logger.Debug`/`logger.Verbose` for diagnostics. The uninstall should use the same patterns and follow the same UX as `UninstallOtelCollector`: show the plan first, then ask for confirmation.

This change is gated by the existing `OneAgentPoC` feature flag (`--oneagent-poc` / `DTWIZ_ONEAGENT_POC`). No separate flag is added, install and uninstall share the same gate.

## What Changes

- New `UninstallOneAgentV2(opts UninstallOptions) error` in `pkg/installer/oneagent/uninstall.go`.
- Platform split: `uninstall_unix.go` (`//go:build !windows`) and `uninstall_windows.go` (`//go:build windows`), same pattern as `detect_unix.go`/`detect_windows.go`.
- `uninstall_unix.go` reuses `oneAgentInstalled()`, `needsSudoFn`, and `sudoPathFn` from the same package.
- `cmd/uninstall.go` routes to V2 when `OneAgentPoC` is enabled.

## Capabilities

### Modified Capabilities

- `oneagent-uninstall`: V2 path checks `oneAgentInstalled()` before proceeding, always shows the plan before asking for confirmation (or returning early on `--dry-run`), uses `display.PrintStatusLine` for output, and emits `logger.Debug`/`logger.Verbose` for diagnostics.

## Impact

- **New files:** `pkg/installer/oneagent/uninstall.go`, `uninstall_unix.go`, `uninstall_windows.go`, `uninstall_unix_test.go`
- **Modified files:** `cmd/uninstall.go` (feature-flag dispatch + `ErrInstallCancelled` handling)
- **No new flag** — gated by the existing `OneAgentPoC` flag.
- **V1 path unchanged** — default until `OneAgentPoC` is enabled.
- **Rollback:** disable the flag; V1 runs immediately with no code change.
