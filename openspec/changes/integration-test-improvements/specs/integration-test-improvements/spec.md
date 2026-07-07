# Spec: Integration Test Improvements

## ADDED Requirements

### Requirement: Parallel test execution by default

`make test-integration` SHALL run tests in parallel by default.

When `SEQUENTIAL=true` is passed to make, it SHALL set `TEST_SEQUENTIAL=1` in the test binary environment, causing tests to run one at a time.

#### Scenario: Default run is parallel

- **WHEN** `make test-integration` is run without `SEQUENTIAL=true`
- **THEN** `TEST_SEQUENTIAL` is not set in the test binary environment
- **THEN** tests that call `Parallelize(t)` run concurrently

#### Scenario: Sequential run when opted in

- **WHEN** `make test-integration SEQUENTIAL=true` is run
- **THEN** `TEST_SEQUENTIAL=1` is set in the test binary environment
- **THEN** `Parallelize(t)` does not call `t.Parallel()`, so tests run one at a time

---

### Requirement: `Parallelize` helper respects `TEST_SEQUENTIAL`

`integration.Parallelize(t)` SHALL call `t.Parallel()` unless the environment variable `TEST_SEQUENTIAL` is set to a non-empty value.

#### Scenario: Marks test parallel when `TEST_SEQUENTIAL` is unset

- **WHEN** `Parallelize(t)` is called and `TEST_SEQUENTIAL` is not set
- **THEN** `t.Parallel()` is called

#### Scenario: Does not mark test parallel when `TEST_SEQUENTIAL=1`

- **WHEN** `Parallelize(t)` is called and `TEST_SEQUENTIAL=1` is set
- **THEN** `t.Parallel()` is NOT called

---

### Requirement: `AutoConfirm` set once for the whole test binary

`installer.AutoConfirm` SHALL be set to `true` in `TestMain` for the entire e2e test binary. Individual tests SHALL NOT save and restore this value.

#### Scenario: AutoConfirm is true throughout a parallel run

- **WHEN** multiple e2e tests run in parallel
- **THEN** `installer.AutoConfirm` remains `true` for the duration of the run
- **THEN** no test can accidentally reset it to `false` while other tests are running

---

### Requirement: Test summary printed after run

After `make test-integration` finishes, it SHALL print a one-line summary with the count of passed, failed, and skipped tests.

#### Scenario: Summary line appears after test output

- **WHEN** `make test-integration` completes
- **THEN** the last lines of output include a summary in the form `N passed, N failed, N skipped`

---

### Requirement: `TestKubernetesLifecycle` exercises the full Kubernetes installer lifecycle

`TestKubernetesLifecycle` SHALL install the Dynatrace Operator, verify the cluster appears in Dynatrace, then uninstall and verify the namespace is gone.

It SHALL skip if `kubectl` or `helm` are not found on PATH, or if no cluster is reachable via the current kubeconfig context.

#### Scenario: Skips when `kubectl` is not on PATH

- **WHEN** `kubectl` is not found on PATH
- **THEN** the test is skipped

#### Scenario: Skips when `helm` is not on PATH

- **WHEN** `helm` is not found on PATH
- **THEN** the test is skipped

#### Scenario: Skips when no cluster is reachable

- **WHEN** `kubectl cluster-info` fails
- **THEN** the test is skipped

#### Scenario: Full lifecycle succeeds

- **WHEN** `kubectl`, `helm`, and a reachable cluster are available
- **THEN** `InstallKubernetes` runs without error
- **THEN** exactly 2 DynaKube custom resources exist in the `dynatrace` namespace
- **THEN** the cluster name appears in Dynatrace topology within 5 minutes
- **THEN** `UninstallKubernetes` runs without error
- **THEN** the `dynatrace` namespace no longer exists

#### Scenario: Safety-net cleanup runs on partial failure

- **WHEN** the test fails after install but before explicit uninstall
- **THEN** `t.Cleanup` calls `UninstallKubernetes` to remove any created resources

---

### Requirement: `TestAzureLifecycle` exercises the full Azure Monitor installer lifecycle

`TestAzureLifecycle` SHALL install the Azure Monitor integration, verify the Dynatrace connection and monitoring configuration exist, then uninstall and verify both are gone.

It SHALL skip if `az` is not found on PATH. It SHALL fatal with an actionable message if the `az` session is not active or the Graph token is expired.

#### Scenario: Skips when `az` is not on PATH

- **WHEN** `az` is not found on PATH
- **THEN** the test is skipped

#### Scenario: Fatals when not logged in to Azure

- **WHEN** `az account show` exits with a non-zero code
- **THEN** the test fatals with a message instructing the developer to run `az login`

#### Scenario: Fatals when Graph token is expired

- **WHEN** `az ad signed-in-user show` exits with a non-zero code
- **THEN** the test fatals with a message explaining the Graph session is stale and instructing the developer to run `az login` again

#### Scenario: Full lifecycle succeeds

- **WHEN** `az` is installed, the session is active, and Graph token is valid
- **THEN** `InstallAzure` runs without error
- **THEN** `azure.ConnectionExists` returns true
- **THEN** `azure.MonitoringConfigExists` returns true
- **THEN** `UninstallAzure` runs without error
- **THEN** `azure.ConnectionExists` returns false
- **THEN** `azure.MonitoringConfigExists` returns false

#### Scenario: Safety-net cleanup runs on partial failure

- **WHEN** the test fails after install but before explicit uninstall
- **THEN** `t.Cleanup` calls `UninstallAzure` to remove any created resources

---

### Requirement: `MonitoringConfigExists` reports whether the Azure monitoring configuration is present

`azure.MonitoringConfigExists(envURL, platformToken string)` SHALL return `true` if a monitoring configuration named after the integration exists in the given Dynatrace environment, and `false` otherwise.

#### Scenario: Returns true when configuration exists

- **WHEN** the Azure monitoring configuration has been created
- **THEN** `MonitoringConfigExists` returns `(true, nil)`

#### Scenario: Returns false when configuration does not exist

- **WHEN** no Azure monitoring configuration exists
- **THEN** `MonitoringConfigExists` returns `(false, nil)`

---

### Requirement: `TEST_DEBUG` mode logs DQL queries and Kubernetes diagnostics

When `TEST_DEBUG` is set to a non-empty value, the test binary SHALL log each DQL query string before it is sent. On a Kubernetes test failure, it SHALL also log the output of `kubectl get pods` and `kubectl get events` in the `dynatrace` namespace.

#### Scenario: DQL query logged when `TEST_DEBUG` is set

- **WHEN** `TEST_DEBUG=1` is set and a DQL query is executed
- **THEN** the query string is logged via `log.Printf`

#### Scenario: No DQL logging when `TEST_DEBUG` is unset

- **WHEN** `TEST_DEBUG` is not set and a DQL query is executed
- **THEN** the query string is NOT logged

#### Scenario: Kubernetes diagnostics logged on test failure

- **WHEN** `TEST_DEBUG=1` is set and `TestKubernetesLifecycle` fails
- **THEN** `kubectl get pods -n dynatrace -o wide` output is logged
- **THEN** `kubectl get events -n dynatrace` warning events are logged
