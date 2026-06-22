## Why

The OneAgent V2 install and uninstall functions (`InstallOneAgentV2`, `UninstallOneAgentV2`) each have isolated unit tests, but there are no tests that exercise the full lifecycle: install → detect installed state → uninstall → verify cleanup. A regression in state detection or cleanup logic would go undetected until production.

## What Changes

- Add lifecycle integration tests that exercise install followed by uninstall in a single hermetic test, using a temp dir as the install directory and a fake HTTP server as the Dynatrace tenant.
- Add orchestration tests for `UninstallOneAgentV2` covering: pre-check early return when not installed, plan printed before dry-run/prompt, confirmation accept/decline/AutoConfirm, `ErrInstallCancelled` propagation, and Linux sudo/no-sudo path selection.
- Add state-detection tests that assert `oneAgentInstalled()` transitions correctly as the install dir is created and removed.
- Add a real-tenant end-to-end test in `test/e2e/` that installs OneAgent against a live Dynatrace environment, waits for the host to register in Smartscape topology (via Grail/DQL), then uninstalls and verifies cleanup — mirroring the existing OTel auto-instrumentation e2e test. Gated behind the `integration` build tag and Linux + root.

## Capabilities

### New Capabilities

- `oneagent-lifecycle-integration`: Full install-then-uninstall test that exercises `InstallOneAgentV2` and `UninstallOneAgentV2` in sequence, verifying state transitions between them.
- `uninstall-v2-orchestration`: Orchestration-layer tests for `UninstallOneAgentV2` covering all branches (not-installed early return, dry-run, confirm/decline, sudo/no-sudo, cleanup).
- `oneagent-e2e-lifecycle`: Real-tenant end-to-end test that installs OneAgent against a live Dynatrace environment, confirms the host registers in Smartscape topology via Grail, then uninstalls and verifies cleanup.

### Modified Capabilities

## Impact

- Hermetic tests: new `_test.go` files in `pkg/installer/oneagent/`, run as part of `make test` with no real installs, network, or platform dependencies.
- E2e test: new `test/e2e/oneagent_test.go` plus a host-topology poller in `test/integration/grail/`. Gated behind `//go:build integration`; runs only when invoked with `-tags=integration` and requires `TEST_DT_ENVIRONMENT` + `TEST_DT_PLATFORM_TOKEN`, Linux, and root. Not part of the default `make test`.
- No production code changes beyond the previously-introduced `runUninstallFn`/`runCommandFn` injectables; no new dependencies.
