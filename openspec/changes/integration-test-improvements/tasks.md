# Tasks: Integration Test Improvements

## 1. Parallel execution infrastructure

- [x] 1.1 Add `Parallelize(t)` to `test/integration/setup.go`: calls `t.Parallel()` unless `TEST_SEQUENTIAL=1`
- [x] 1.2 Create `test/e2e/main_test.go`: set `installer.AutoConfirm = true` in `TestMain` for the whole binary

## 2. Update existing tests

- [x] 2.1 `test/e2e/oneagent_test.go`: remove per-test `AutoConfirm` save/restore; add `integration.Parallelize(t)` call
- [x] 2.2 `test/e2e/otel_test.go`: remove per-test `AutoConfirm` save/restore; add `integration.Parallelize(t)` call

## 3. Kubernetes lifecycle test

- [x] 3.1 Add `kubernetesClusterByNameQuery` to `test/integration/grail/helpers.go`
- [x] 3.2 Create `test/integration/grail/kubernetes.go` with `WaitForKubernetesCluster` and `RequireKubernetesCluster`
- [x] 3.3 Create `test/e2e/kubernetes_test.go` with `TestKubernetesLifecycle`: skips if `kubectl` or `helm` are not on PATH or no cluster is reachable; installs, verifies two DynaKube CRs and cluster topology in Dynatrace, then uninstalls and verifies namespace is gone

## 4. Azure lifecycle test

- [x] 4.1 Export `MonitoringConfigExists(envURL, platformToken string)` from `pkg/installer/azure/uninstall.go`
- [x] 4.2 Create `test/e2e/azure_test.go` with `TestAzureLifecycle`: skips if `az` CLI is not on PATH; fatals with instructions if not logged in or Graph token is stale; installs, verifies connection and monitoring config exist, then uninstalls and verifies both are gone

## 5. Debug mode

- [x] 5.1 Add `TEST_DEBUG` check to `test/integration/grail/execute.go`: log DQL query string when `TEST_DEBUG != ""`
- [x] 5.2 In `TestKubernetesLifecycle`, add `t.Cleanup` that logs `kubectl get pods` and `kubectl get events` on failure when `TEST_DEBUG` is set

## 6. Make target

- [x] 6.1 Update `make test-integration` to capture output, print it in real time, and print a pass/fail/skip summary after the run
- [x] 6.2 Add `SEQUENTIAL` flag: when `SEQUENTIAL=true`, set `TEST_SEQUENTIAL=1` in the test environment
- [x] 6.3 Raise timeout from 15 to 30 minutes
- [x] 6.4 Export `TEST_DEBUG` from make so the test binary sees it
- [x] 6.5 Add `RACE_FLAG` conditional: skip `-race` on Windows (race detector requires cgo)

## 8. GCP lifecycle test

- [x] 8.1 Export `MonitoringConfigExists(envURL, platformToken string)` from `pkg/installer/gcp/uninstall.go`
- [x] 8.2 Create `test/e2e/gcp_test.go` with `TestGCPLifecycle`: skips if `gcloud` is not on PATH; fatals with instructions if no active project is set; installs, verifies connection and monitoring config exist, then uninstalls and verifies both are gone

## 7. Documentation

- [x] 7.1 Update `docs/contributing/testing.md`: document `Parallelize`, `SEQUENTIAL=true`, `TEST_DEBUG`, and the per-test infrastructure prerequisites table
