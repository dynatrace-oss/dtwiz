# Proposal: Integration Test Improvements

## Why

The integration test suite had several gaps that made it harder to use and trust:

- Tests ran sequentially, so the full suite took longer than necessary.
- Setting `AutoConfirm = true` was done per-test, which was unsafe to do in parallel (a finishing test could reset the flag while other tests were still running).
- The Kubernetes and Azure installers had no lifecycle tests; install and uninstall were never validated end-to-end against a real cluster or cloud account.
- When `TestAzureLifecycle` wanted to verify that a monitoring configuration was removed after uninstall, there was no exported function to check for it.
- The test output was hard to read: no summary of how many tests passed, failed, or were skipped.
- When a test failed, there was no easy way to see the DQL queries that ran or the Kubernetes state at the time of failure.

## What Changes

- Integration tests run in parallel by default; pass `SEQUENTIAL=true` to run them one at a time.
- `AutoConfirm = true` is set once in `TestMain` for the whole binary instead of per-test.
- A `Parallelize(t)` helper marks a test as parallel, and respects `SEQUENTIAL=true`.
- A `TestKubernetesLifecycle` test installs and uninstalls the Dynatrace Operator against a real cluster and checks that topology data appeared in Dynatrace.
- A `TestAzureLifecycle` test installs and uninstalls the Azure Monitor integration against a real Azure subscription and checks that the Dynatrace connection and monitoring configuration exist and are cleaned up.
- `azure.MonitoringConfigExists()` is exported so the Azure lifecycle test can check post-uninstall state.
- A DQL debug mode (`TEST_DEBUG=1`) logs DQL queries as they run and dumps Kubernetes pod/event state on test failure.
- `make test-integration` prints a summary (passed/failed/skipped counts) after the run.
- The timeout for `make test-integration` is raised from 15 to 30 minutes to accommodate the new lifecycle tests.

## Capabilities

### New Capabilities

none

### Modified Capabilities

none

## Impact

- `test/e2e/main_test.go`: new file; sets `AutoConfirm = true` in `TestMain`
- `test/e2e/kubernetes_test.go`: new file; `TestKubernetesLifecycle`
- `test/e2e/azure_test.go`: new file; `TestAzureLifecycle`
- `test/e2e/oneagent_test.go`: removes per-test `AutoConfirm` save/restore; adds `Parallelize` call
- `test/e2e/otel_test.go`: removes per-test `AutoConfirm` save/restore; adds `Parallelize` call
- `test/integration/setup.go`: adds `Parallelize(t)` helper
- `test/integration/grail/helpers.go`: adds `kubernetesClusterByNameQuery`
- `test/integration/grail/kubernetes.go`: new file; `WaitForKubernetesCluster` and `RequireKubernetesCluster`
- `test/integration/grail/execute.go`: adds `TEST_DEBUG` DQL logging
- `pkg/installer/azure/uninstall.go`: exports `MonitoringConfigExists`
- `makefile`: parallel-safe test runner, `SEQUENTIAL` flag, test summary, 30 min timeout, `TEST_DEBUG` pass-through
- `docs/contributing/testing.md`: updated to reflect parallel execution, `Parallelize` usage, and new infrastructure-dependent tests
