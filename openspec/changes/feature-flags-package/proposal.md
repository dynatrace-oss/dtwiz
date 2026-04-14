## Why

The CLI uses a raw `os.Getenv("DTWIZ_ALL_RUNTIMES")` call in `pkg/installer/otel.go` as a makeshift feature flag to gate unreleased runtimes (Java, Node.js, Go). This pattern doesn't scale — each new gated feature would add another ad-hoc `os.Getenv` call scattered across the codebase. A centralized feature flag package reduces duplication, improves discoverability, and makes flags testable without polluting the process environment.

## What Changes

- New `pkg/featureflags` package with a registry of known flags, each mapped to an environment variable name and a default value.
- Single-entry-point `IsEnabled(flag) bool` function for checking flags.
- `List() []Flag` function returning all registered flags and their current state (for debugging and `dtwiz status`).
- `SetForTest(t, flag, val)` test helper for scoped overrides using `t.Cleanup`, avoiding `os.Setenv`/`os.Unsetenv` and enabling `t.Parallel()`.
- Migrate all `os.Getenv("DTWIZ_ALL_RUNTIMES")` usage in `otel.go` to the new package.
- `DTWIZ_ALL_RUNTIMES` env var name preserved for backward compatibility.
- Each feature flag exposed as a cobra persistent boolean flag on the root command (e.g. `--all-runtimes`), reusing cobra's flag infrastructure. Resolution order: CLI flag → env var → default.
- `dtwiz status` prints active feature flags when any are enabled.
- Evaluate gating Docker, Kubernetes, OneAgent, AWS, Azure, and GCP analysis/recommendations behind feature flags — a manual evaluation task for the implementer.

## Capabilities

### New Capabilities

- `feature-flag-registry`: Central registry of feature flags with env var mapping, default values, IsEnabled/List API, cobra CLI flag support, and scoped test helper.
- `status-feature-flags`: Display active feature flags in `dtwiz status` output for diagnostics.

### Modified Capabilities
<!-- No existing spec-level requirements are changing. The migration from os.Getenv to the new package is an implementation detail that preserves identical behavior. -->

## Impact

- **New package:** `pkg/featureflags/` — zero external dependencies (stdlib only).
- **Modified files:** `pkg/installer/otel.go` (replace `allRuntimesEnabled()` with `featureflags.IsEnabled()`), `pkg/installer/otel_test.go` (update 3 test functions to use new API), `cmd/root.go` (register cobra persistent flags for each feature flag), `cmd/status.go` (feature flag display section).
- **No breaking changes:** `DTWIZ_ALL_RUNTIMES` env var continues to work identically.
- **Adding a new flag** requires only a single constant definition + env var mapping in the registry — no changes to consuming code patterns.
- **Rollback:** Revert the commit and restore the original `os.Getenv` call — no data migration or external state involved.
