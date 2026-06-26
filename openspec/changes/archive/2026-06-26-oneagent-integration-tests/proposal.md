# Proposal: OneAgent Integration Tests

## Why

OneAgent V2 has unit tests for install and uninstall, but no test covers the full lifecycle: install, detect installed state, uninstall, verify cleanup.

## What Changes

- Add a hermetic install-then-uninstall lifecycle test.
- Add `UninstallOneAgentV2` orchestration tests: not installed, dry-run, accept, decline, AutoConfirm, script failure, sudo argv.
- Add state-detection checks for created and removed install dirs.
- Add a real-tenant e2e test behind the `integration` build tag. It runs on Linux (root) and Windows.

## Capabilities

### New Capabilities

- `oneagent-integration-tests`: lifecycle test coverage for OneAgent V2.
- Includes uninstall orchestration, hermetic install-then-uninstall, and real-tenant e2e coverage.

### Modified Capabilities

## Impact

- New package tests in `pkg/installer/oneagent/`.
- New `test/e2e/oneagent_test.go` and Grail host poller.
- E2e requires `-tags=integration`, tenant env vars, and Linux root or Windows.
- No new dependencies.
