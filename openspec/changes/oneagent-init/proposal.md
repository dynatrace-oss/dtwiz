# Why

The OneAgent PoC epic (Tasks 1–11) implements a production-ready installer flow with mandatory token minting, signature verification, post-install confirmation, and cross-platform support. All subsequent tasks depend on foundational scaffolding: a feature flag to gate the new implementation during development, type definitions for configuration and state, CLI flags for the new options, and feature-flag branching to run both old and new flows in parallel until the new code is ready to ship.

This change lays that foundation without touching the core installer logic.

## What Changes

- **Feature flag:** `ONEAGENT_POC` (env var `DTWIZ_ONEAGENT_POC`) added to `pkg/featureflags` — disabled by default, removable in a single commit at the end of Task 8.
- **New file:** `pkg/installer/oneagent_v2.go` with entry point `InstallOneAgentV2()` and stub functions for Tasks 2–7.
- **Type definitions:** `Environment`, `AgentConfig`, `InstallOptions`, `Endpoint`, `ConnectivityReport`, `ConnectivityResult` — all exported and used by subsequent tasks.
- **Test scaffold:** `pkg/installer/oneagent_v2_test.go` with helper stubs and test structure.
- **CLI flags:** `--force`, `--monitoring-mode`, `--no-verify-signature`, `--skip-connectivity-check`, `--connectivity-check-only`, `--print-endpoints` — all wired on `installOneAgentCmd`.
- **Feature-flag branching:** In `cmd/install.go`, conditional dispatch based on `ONEAGENT_POC` enables the new flow when the flag is set.

## Capabilities

### New Capabilities

- `oneagent-scaffolding`: Feature flag, type definitions, and scaffolded functions for all OneAgent PoC tasks.
- `oneagent-feature-flag`: Temporary gate for the new implementation during development.
- `oneagent-cli-flags`: New CLI options for pre-flight, signature verification, connectivity, and configuration.

## Impact

- **New files:** `pkg/installer/oneagent_v2.go`, `pkg/installer/oneagent_v2_test.go`
- **Modified files:** `pkg/featureflags/featureflags.go` (new constant + registry entry), `cmd/install.go` (new flags + branching)
- **No changes to existing behavior** — by default (`ONEAGENT_POC=false`), `dtwiz install oneagent` uses the existing flow.
- **Rollback:** Delete `pkg/installer/oneagent_v2*.go`, remove the feature flag constant and registry entry, revert `cmd/install.go` branching. Three-file change to undo.
- **Pre-requisite:** This change must land before Tasks 2–7 can be implemented.
