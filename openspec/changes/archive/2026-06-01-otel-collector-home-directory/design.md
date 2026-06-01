# Design: OTel Collector Home Directory

## Context

The `dtwiz install otel` command downloads and runs the Dynatrace OTel Collector. Previously, the binary and config were placed in `<cwd>/opentelemetry/`. This is problematic because:

1. The user may invoke `dtwiz` from a directory they don't own (e.g., `/opt/app`, a root-owned project dir)
2. On Windows, the CWD is often a system-managed path or a location with restrictive ACLs
3. The install location becomes unpredictable — it changes depending on where the user happens to run the command

The uninstall flow (`candidateOtelDirs`) already checked `~/opentelemetry` as a fallback, acknowledging this problem implicitly.

## Goals / Non-Goals

**Goals:**

- Use `~/opentelemetry` as the single install directory on all platforms
- Avoid permission errors on any OS without requiring elevated privileges
- Provide a stable, predictable install location independent of CWD

**Non-Goals:**

- Custom install path via flag (can be added later if needed)
- Migrating existing installations from `<cwd>/opentelemetry` to `~/opentelemetry`
- Changing the uninstall discovery logic (it already checks both paths)

## Decisions

### Decision 1: Use `os.UserHomeDir()` on all platforms

**Choice**: A single `otelCollectorInstallDir()` function returns `filepath.Join(home, "opentelemetry")` unconditionally.

**Alternatives considered**:

- Windows-only change (home dir on Windows, CWD elsewhere) — rejected because Linux/macOS have the same permission issues
- `os.UserConfigDir()` (`~/.config/` on Linux, `~/Library/Application Support/` on macOS) — rejected because this is a runtime binary, not a config file; the home directory is more intuitive and discoverable

**Rationale**: Simplest approach, consistent behavior everywhere. Users can always find their collector at `~/opentelemetry/`.

### Decision 2: Reuse the helper in `updateOtelCollectorIfPresent`

The helper that patches the collector config after Java/Node.js instrumentation also used `os.Getwd()`. Updated to call `otelCollectorInstallDir()` for consistency.

### Decision 3: No migration of existing installs

Existing installs in `<cwd>/opentelemetry/` are left in place. The uninstall flow already discovers them via `candidateOtelDirs()` which checks both `~/opentelemetry` and `<cwd>/opentelemetry`. No automatic migration is performed.

## Risks / Trade-offs

- [Home dir unavailable] Extremely unrealistic — `os.UserHomeDir()` only fails if `$HOME` is unset and `/etc/passwd` lookup fails simultaneously, which virtually never happens even in minimal containers. Mitigation: return a clear error message
