# Why

`cmd/status.go` has grown into a single monolithic `RunE` closure mixing credential retrieval, token validation, output formatting, and color definitions. Five package-level color variables duplicate colors defined identically in `pkg/analyzer/` and `pkg/installer/`, with no shared source of truth. A new `pkg/display` package centralizes terminal colors and print helpers, and the status command is refactored to use them — eliminating duplication, improving consistency, and making both easier to test and extend.

## What Changes

- New `pkg/display` package with shared terminal color definitions (`ColorOK`, `ColorError`, `ColorHeader`, `ColorMuted`, `ColorLabel`) and print helpers (`Header()`, `PrintSectionDivider()`, `PrintStatusLine()`).
- `cmd/status.go` refactored: package-level color vars removed, `RunE` closure trimmed by extracting `printCredentialStatus()` helper, token validation logic deduplicated via `CredentialToken` struct.
- `pkg/analyzer/analyzer.go` migrated from local `colorHeader`/`colorMuted` vars to `display.ColorHeader`/`display.ColorMuted`.
- System analysis error in `dtwiz status` now propagates (`return err`) instead of being swallowed (`return nil`).
- Trailing blank line after Platform Token section removed (spacing normalized to single newline, matching all other credential lines).
- `dtwiz status` displays a "Feature Flags" section when any flags are enabled (via `featureflags.List()`), omitted entirely when none are active.

## Capabilities

### New Capabilities

- `display-package`: Centralized `pkg/display` package exposing shared terminal colors and print helpers for use across `cmd/` and `pkg/`.
- `status-command-structure`: Structural requirements for `dtwiz status` — how credential validation, output sections, and error handling are composed.
- `status-feature-flags`: Feature flags section in `dtwiz status`.

## Impact

- **New package:** `pkg/display/` — only dependency is `github.com/fatih/color` (already in go.mod).
- **Modified files:** `cmd/status.go`, `pkg/analyzer/analyzer.go`.
- **Behavioral changes:** (1) `dtwiz status` now exits non-zero when system analysis fails; (2) one fewer blank line after Platform Token output.
- **No breaking changes** to CLI flags, env vars, or output format beyond the spacing fix.
- **Rollback:** Revert the commit — no external state or data migration involved.
