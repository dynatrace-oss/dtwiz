# Why

The OneAgent PoC epic (Tasks 1–11) implements a production-ready installer flow with mandatory token minting, signature verification, post-install confirmation, and cross-platform support. All subsequent tasks depend on foundational scaffolding: a feature flag to gate the new implementation during development, type definitions for configuration and state, CLI flags for the new options, and feature-flag branching to run both old and new flows in parallel until the new code is ready to ship.

This change lays that foundation without touching the core installer logic.

## What Changes

- **Feature flag:** `ONEAGENT_POC` (env var `DTWIZ_ONEAGENT_POC`) added to `pkg/featureflags` as a temporary development gate — has since been removed; `InstallOneAgentV2` is now the unconditional default.
- **New package:** `pkg/installer/oneagent/` with entry point `InstallOneAgentV2()` and implementations split across three files: `oneagent.go` (types, entry point, config resolution), `download.go` (installer download), `verify.go` (CMS signature verification).
- **Type definitions:** `Environment`, `AgentConfig`, `InstallOptions` — all exported in `pkg/installer/oneagent`.
- **Tests:** `pkg/installer/oneagent/oneagent_test.go` with HTTP mock helpers and full test coverage.
- **CLI flags:** `--force`, `--monitoring-mode`, `--no-verify-signature`, `--skip-connectivity-check`, `--connectivity-check-only` — all wired on `installOneAgentCmd`.
- **Feature-flag branching:** In `cmd/install.go`, conditional dispatch based on `ONEAGENT_POC` enables the new flow when the flag is set.

## Capabilities

### New Capabilities

- `oneagent-scaffolding`: Feature flag, type definitions, and scaffolded functions for all OneAgent PoC tasks.
- `oneagent-feature-flag`: Temporary gate for the new implementation during development.
- `oneagent-cli-flags`: New CLI options for pre-flight, signature verification, connectivity, and configuration.

## Impact

- **New package:** `pkg/installer/oneagent/` (`oneagent.go`, `download.go`, `verify.go`, `oneagent_test.go`)
- **Modified files:** `pkg/featureflags/featureflags.go` (new constant + registry entry), `cmd/install.go` (new flags + branching), `cmd/setup.go` (new import + call)
- **No changes to existing behavior** — by default (`ONEAGENT_POC=false`), `dtwiz install oneagent` used the existing flow. The flag has since been removed; the V2 flow is now unconditional.
- **Pre-requisite:** This change must land before Tasks 2–7 can be implemented.
