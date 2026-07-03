# Tasks

## 1. Package Scaffolding & Shared Types

- [x] 1.1 Create `pkg/installer/azure/config.go`: `azureConfig` struct, `cmdRunner` type, `realRunner`, `execLookPath` alias, `maskToken`, and the fixed `integrationName` (`dtwiz-azure`) / `fedCredName` (`dtwiz-azure-Federated-Credential`) constants
- [x] 1.2 Add `github.com/dynatrace-oss/dtctl/sdk` (`httpclient`, `api/settings`, `api/extension`) to `go.mod`

## 2. Dynatrace Platform API Client

- [x] 2.1 Define the `dtclient` interface and `connRef` in `pkg/installer/azure/dtapi.go`: `createConnection`, `updateConnection`, `createMonitoring`, `updateMonitoring`, `findAllConnections`, `deleteConnection`, `findAllMonitoringConfigs`, `deleteMonitoring`
- [x] 2.2 Implement `sdkDTClient` via `newSDKDTClient(envURL, platformToken)` against `installer.AppsURL(envURL)` with a Bearer token; enable verbose SDK logging when `logger.IsDebug()`
- [x] 2.3 Implement `createConnection`/`updateConnection` against schema `builtin:hyperscaler-authentication.connections.azure` with `type: federatedIdentityCredential` and consumer `SVC:com.dynatrace.da`
- [x] 2.4 Implement `findAllConnections`/`findAllMonitoringConfigs` to return every name-matching object (duplicate-tolerant); `deleteConnection`/`deleteMonitoring` treating `404` as success
- [x] 2.5 Implement `buildMonitoringConfig` (shared body builder): select highest extension version (`cmpSemver`), fetch schema, populate `locationFiltering` from the `dynatrace.datasource.azure:location` enum and `featureSets` from `FeatureSetsType` keeping `*_essential`; error when either enum is empty; subscription filtering `INCLUDE` + `FEDERATED` credential entry. `createMonitoring` POSTs it; `updateMonitoring` PUTs it in place to an existing config ID
- [x] 2.6 Implement `extensionSchema`/`enumValues`, `fetchExtensionSchema`, `latestExtensionVersion`, and `cmpSemver`

## 3. Azure Helpers (az CLI)

- [x] 3.1 `azureIssuerURL`: derive the issuer from the apps URL qualifier (`token.`, `dev.token.`, `sprint.token.` + domain)
- [x] 3.2 `azureBuildFedCredJSON`: federated credential body (name, issuer, `dt:connection-id/<id>` subject, `<apps-host>/svc-id/com.dynatrace.da` audience)
- [x] 3.3 `azureGetSPObjectID`: `az ad sp show` with 5× / 3s backoff on not-found; fail fast on 403
- [x] 3.4 `azureListAppIDsByName` + `azureAppHasDtwizFedCred`: display-name lookup and federated-credential ownership fingerprint check
- [x] 3.5 `azureDeleteFedCred`, `azureDeleteRoleAssignment`, `azureDeleteApp`: idempotent deletes treating "not found" / "no matched assignments" as success

## 4. Preflight

- [x] 4.1 `azurePreflightChecks` in `pkg/installer/azure/preflight.go`: `az` on PATH, `az account show` parsed for subscription + tenant, subscription scope `/subscriptions/<id>`
- [x] 4.2 `azureCheckRBAC`: advisory ARM `checkAccess` for `roleAssignments/write`; warn-only, never blocks

## 5. Install Workflow

- [x] 5.1 `InstallAzure` entry point + `installAzureWithRunner` testable core (injected runner/sleeper/dtclient) in `pkg/installer/azure/install.go`
- [x] 5.2 Existence check (abort if `dtwiz-azure` connection exists), `azureConfig` assembly, `azurePrintPreview` (masked token), `--dry-run` stop, `ConfirmProceed`
- [x] 5.3 `runInstallSteps(offset, total, ...)`: the 7 ordered steps threading connection/client/object IDs forward, with `offset`-aware numbering
- [x] 5.4 Step 3 stale-credential replacement (delete + retry once on `already exists`)
- [x] 5.5 Step 6 retry loop: up to 10× / 5s on `AADSTS70025` or `Constraints violated`, stop on any other error
- [x] 5.6 `azurePartialFailureHint`: print created resources + cleanup commands on partial failure
- [x] 5.7 `azureWatchIngest`: tail ingest from start time; skip when start time is zero

## 6. Uninstall Workflow

- [x] 6.1 `UninstallAzure` entry point + `uninstallAzureWithRunner` core; `ConnectionExists` helper in `pkg/installer/azure/uninstall.go`
- [x] 6.2 Concurrent discovery of monitoring configs + connections; `azureGatherClientIDs` (trusted-by-connection ∪ ownership-verified-by-name), de-duplicated and sorted
- [x] 6.3 Nothing-to-do short-circuit; `azureUninstallPrintPreview` + `azureUninstallBuildSteps`; `--dry-run` stop; `ConfirmProceed`
- [x] 6.4 `uninstallStepCount` (1 per config + 2 per app + 1 per connection) and `runUninstallSteps(offset, total, ...)`: best-effort, warn-and-continue, `errors.Join`

## 7. Update (In-Place Reconcile) Workflow

- [x] 7.1 `UpdateAzure` entry point + `updateAzureWithRunner` core in `pkg/installer/azure/update.go`
- [x] 7.2 Parallel discovery (monitoring configs + connections) + `azureAccountInfo` (no role-assignment RBAC advisory); `selectUpdatableConnection` requires exactly one complete connection, else abort with install/uninstall guidance
- [x] 7.3 `azureUpdatePrintPreview`: env/tenant/subscription/connection (unchanged)/configuration + per-config update (or single create) steps; `--dry-run` stop; `ConfirmProceed`
- [x] 7.4 `reconcileMonitoring`: `updateMonitoring` each existing config in place via shared `buildMonitoringConfig`, or `createMonitoring` when none exists; auth chain untouched; ingest watch on success
- [x] 7.5 `installAzureWithRunner`: when `findAllConnections` returns a complete connection (bound application ID), delegate straight to `updateAzureWithRunner` instead of aborting; incomplete/duplicated connections still abort with uninstall/reinstall guidance

## 8. CLI Wiring

- [x] 8.1 Add `installAzureCmd` (`dtwiz install azure`, `cobra.NoArgs`) in `cmd/install.go` → `azure.InstallAzure`
- [x] 8.2 Add `uninstallAzureCmd` (`dtwiz uninstall azure`, `cobra.NoArgs`) in `cmd/uninstall.go` → `azure.UninstallAzure`
- [x] 8.3 In `cmd/setup.go`: pre-check `azure.ConnectionExists`, badge the Azure entry when configured, route to `UpdateAzure` vs `InstallAzure`, and suppress the generic post-install watch for Azure (it runs its own)
- [x] 8.4 Add `updateAzureCmd` (`dtwiz update azure`, `cobra.NoArgs`) in `cmd/update.go` → `azure.UpdateAzure`, matching the `install`/`update`/`uninstall` trio otel already has (not hidden/experimental, unlike `update otel`)

## 9. Tests

- [x] 9.1 `config_test.go`: `maskToken` and config defaults
- [x] 9.2 `dtapi_test.go`: schema enum extraction, `*_essential` filtering, `cmpSemver`, create/update/find/delete via fake SDK responses
- [x] 9.3 `helpers_test.go`: `azureIssuerURL` variants, fed-cred JSON, SP-object-ID retries/403, ownership fingerprint, idempotent deletes
- [x] 9.4 `preflight_test.go`: CLI-missing, not-logged-in, RBAC advisory warn-but-continue
- [x] 9.5 `install_test.go`: full 7-step workflow with injected runner/sleeper/dtclient, existence-check delegates to update (complete connection) or aborts (incomplete/duplicated), dry-run, step-3 replacement, step-6 propagation retries, partial-failure hint
- [x] 9.6 `uninstall_test.go`: discovery, ownership-verified gathering, nothing-to-do, best-effort continue-on-failure, step count
- [x] 9.7 `update_test.go`: parallel discovery+preflight, in-place reconcile of existing config(s), create-when-missing, duplicate-config reconcile, and `selectUpdatableConnection` abort cases (no complete / multiple connections)
- [x] 9.8 `make test` and `make lint`: all pass
