# Design: Integration Test Improvements

## Context

The integration tests live in `test/e2e/` and run against a live Dynatrace environment. Before this change, they ran sequentially, each test set and restored `installer.AutoConfirm` individually, and there were no lifecycle tests for the Kubernetes or Azure installers.

## Goals / Non-Goals

Goals:

- Run integration tests in parallel by default to shorten the total run time.
- Add lifecycle (install → verify → uninstall → verify) tests for Kubernetes and Azure.
- Give developers a way to see DQL queries and Kubernetes state when a test fails.
- Print a pass/fail/skip summary at the end of `make test-integration`.

Non-Goals:

- Provisioning test infrastructure (clusters, Azure subscriptions): tests skip if prerequisites are not available.
- Adding lifecycle tests for other installers (OneAgent and OTel already have them).

## Decisions

### Set `AutoConfirm = true` once in `TestMain`

Each test previously saved and restored `installer.AutoConfirm` around its own body. That pattern breaks under parallel execution: a test finishing early restores the flag to `false` while other tests are still running mid-install. Moving the assignment to `TestMain` sets it once for the entire binary and removes the race.

### `Parallelize(t)` helper instead of calling `t.Parallel()` directly

Tests opt in to parallelism by calling `integration.Parallelize(t)`. The helper calls `t.Parallel()` unless `TEST_SEQUENTIAL=1` is set, which lets developers run tests one at a time by passing `SEQUENTIAL=true` to make. This is simpler than trying to control parallelism from outside the test binary.

`Parallelize` is called before `SetupIntegration` so that Go's parallel scheduling starts as early as possible (before the test body waits for the parallel semaphore to release).

### Azure pre-flight checks before acquiring any resources

`TestAzureLifecycle` checks `az account show` and `az ad signed-in-user show` before calling the installer. This catches two common failure modes (no active session, expired Graph token) before any Azure or Dynatrace resources are created. Without these checks, a test could create partial state (e.g. a Dynatrace connection) and then fail mid-install, leaving the cleanup to guess what was created.

### `MonitoringConfigExists` exported from `azure/uninstall.go`

The test needs to verify that a monitoring configuration exists after install and is gone after uninstall. `findAllMonitoringConfigs` is already used internally by the uninstaller; wrapping it in an exported function gives the test access to the same check without duplicating logic or changing the uninstall path.

### Safety-net `t.Cleanup` on both lifecycle tests

Both `TestKubernetesLifecycle` and `TestAzureLifecycle` register a cleanup that calls the uninstaller regardless of test outcome. Installers can create real resources (namespaces, App Registrations, Dynatrace connections) and may fail partway through. Without a safety-net cleanup, a partial failure leaves resources behind. The cleanup is skipped only when the explicit uninstall step in the test body already succeeded (tracked by the `uninstalled` flag in the Kubernetes test).

### `TEST_DEBUG` for DQL and cluster diagnostics

Debugging a failing integration test often requires knowing what DQL queries ran and what state the cluster was in at the time of failure. Setting `TEST_DEBUG=1` enables two things:
- DQL queries are logged as they run (added to `grail/execute.go`).
- On Kubernetes test failure, `kubectl get pods` and `kubectl get events` output is logged.

`TEST_DEBUG` is passed through from make via `export TEST_DEBUG` so it is available to the test binary without extra steps.

### Test summary in `make test-integration`

`go test -v` output includes pass/fail lines but no totals. The updated make target captures output in a temp file, prints it in real time via `tee`, and then greps the file to print counts. The temp file is deleted via `trap` after the target exits, so it does not accumulate across runs.

### Timeout raised

`TestKubernetesLifecycle` alone can take up to 20 minutes: 10 min waiting for Operator pods to become ready, 5 min polling for cluster topology in Dynatrace, and 5 min waiting for pods to terminate on uninstall. The previous 15-minute timeout was already too short for this single test.

### `kubernetesClusterByNameQuery` and `WaitForKubernetesCluster`

The existing `grail` helpers follow the pattern: a `*Query` function builds the DQL string, a `WaitFor*` function polls until records appear, and a `Require*` function wraps the poll and fatals if it fails. The Kubernetes helpers follow the same pattern. The DQL query fetches logs with `k8s.cluster.name` matching the cluster context name (lowercased to match what Dynatrace normalizes it to).

## Risks / Trade-offs

- Parallel tests share a single Dynatrace tenant, so they all write to the same environment. There is no tenant-level isolation. For the installers under test (OneAgent, OTel Collector, Kubernetes Operator, Azure Monitor) this is acceptable because each installer manages distinct resources.
- `TestKubernetesLifecycle` and `TestAzureLifecycle` create and destroy real cloud resources. Pointing them at a production environment would be destructive. The tests document this in their comments; skipping when prerequisites are absent helps, but cannot prevent misuse.
- The Azure test uses `integrationName` (from the installer package) as the resource name. If that constant changes, the test continues to work because it calls the same installer functions. The monitoring-config existence check is also based on the same constant via `MonitoringConfigExists`.
