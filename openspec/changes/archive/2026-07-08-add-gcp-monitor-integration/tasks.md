# Tasks

## 1. Shared Installer Infrastructure (extracted from Azure, reused by GCP)

- [x] 1.1 Extract `pkg/installer/cmdrunner.go`: `CmdRunner` type, `ExecLookPath` var, `RealRunner`, `IsNotFoundErr`
- [x] 1.2 Extract `pkg/installer/concurrent.go`: `RunConcurrently` (fan out, join all errors with `errors.Join`)
- [x] 1.3 Extract `pkg/installer/retry.go`: `Retry`, `RetryConfig`, `Jitter` (up to 50% extra random delay to decorrelate concurrent retries)
- [x] 1.4 Extract `pkg/installer/extension_client.go`: `ExtensionClient` (Settings/Extension handlers), `DeleteConnection` (404-as-success), `FindAllMonitoringConfigs`, `DeleteMonitoringConfiguration` (404-as-success), `ExtensionSchema`/`EnumValues`, `FetchExtensionSchema`, `LatestExtensionVersion`/`cmpSemver`
- [x] 1.5 Refactor `pkg/installer/azure/*` onto the shared infrastructure (`sdkDTClient` embeds `*installer.ExtensionClient`; `azureGetSPObjectID` uses `installer.Retry` + `installer.Jitter`)

## 2. Package Scaffolding & Shared Types

- [x] 2.1 Create `pkg/installer/gcp/config.go`: `gcpConfig` struct (+ `serviceAccountEmail()` fallback method), `cmdRunner` type alias, `execLookPath`/`realRunner` aliases, fixed `integrationName` / `serviceAccountName` (`dtwiz-gcp`), `serviceAccountDisplayName`, `viewerRole` (`roles/viewer`), `tokenCreatorRole` (`roles/iam.serviceAccountTokenCreator`), `requiredAPIs` (compute, cloudresourcemanager, cloudasset, monitoring)

## 3. Dynatrace Platform API Client

- [x] 3.1 Define the `dtclient` interface and `connRef` (+ `splitConnectionsByCompleteness`) in `pkg/installer/gcp/dtapi.go`: `createConnection`, `updateConnection`, `createMonitoring`, `updateMonitoring`, `findAllConnections`, `deleteConnection`, `findAllMonitoringConfigs`, `deleteMonitoring`
- [x] 3.2 Implement `sdkDTClient` via `newSDKDTClient(envURL, platformToken)` embedding `*installer.ExtensionClient`
- [x] 3.3 Implement `createConnection`/`updateConnection` against schema `builtin:hyperscaler-authentication.connections.gcp` with `type: serviceAccountImpersonation` and consumer `SVC:com.dynatrace.da`; `updateConnection` bypasses the SDK's `settings.Update` for a raw PUT so `constraintViolations` detail survives (needed to distinguish a propagation delay from a permanent schema mismatch)
- [x] 3.4 Implement `findAllConnections` without a `scopes` filter (confirmed live: `scopes=environment` returns zero results for this schema even for environment-scoped objects) and `dtServiceAccount` (resolves the Dynatrace principal from the read-only `gcp-dynatrace-principal` schema by scanning for a Google service-account email shape, since the field name is environment-managed)
- [x] 3.5 Implement `buildMonitoringConfig` (shared body builder): select highest extension version, fetch schema, populate `featureSets` from the `FeatureSetsType` enum keeping `*_essential`; error when empty; `googleCloud.projectFiltering` = active project, one federated credential entry referencing the connection object ID and service-account email. `createMonitoring` POSTs it; `updateMonitoring` PUTs it in place to an existing config ID

## 4. GCP Helpers (gcloud CLI)

- [x] 4.1 `gcpServiceAccountEmail`: deterministic `<name>@<project>.iam.gserviceaccount.com`
- [x] 4.2 `findServiceAccountEmail`/`isServiceAccountEmail`: recursive scan of a decoded settings value for a Google service-account email shape
- [x] 4.3 `gcpCreateServiceAccount`: `gcloud iam service-accounts create`; an "already exists" error reuses the deterministic email instead of failing
- [x] 4.4 `gcpDeleteServiceAccount`, `gcpRemoveProjectBinding`: idempotent deletes treating not-found as success (`installer.IsNotFoundErr`)
- [x] 4.5 `serviceAccountMember`: formats an email as a `gcloud` IAM member (`serviceAccount:<email>`)

## 5. Preflight

- [x] 5.1 `gcpAccountInfo` in `pkg/installer/gcp/preflight.go`: `gcloud` on `PATH`, `gcloud config get-value project`/`account` parsed via `analyzer.CleanGCloudConfigValue` (strips the Cloud Shell "active configuration" notice line so the parsed value matches what `dtwiz analyze` already detected); aborts with actionable messages when `gcloud` is missing, the user isn't logged in, or no project is active

## 6. Install Workflow

- [x] 6.1 `InstallGCP` entry point + `installGCPWithRunner` testable core (injected runner/sleeper/dtclient) in `pkg/installer/gcp/install.go`
- [x] 6.2 `gcpResumableConnection`: >1 incomplete connections aborts as ambiguous; exactly one incomplete connection is resumed (reused in step 2 instead of creating a duplicate). In `installGCPWithRunner`, before calling `gcpResumableConnection`, check for exactly one complete connection via `selectUpdatableConnection`: if found, print "prerequisites already exist — running update instead of a fresh install" and redirect to `updateGCPWithRunner` without any gcloud mutations
- [x] 6.3 `gcpConfig` assembly, `gcpPrintPreview`, `--dry-run` stop, `ConfirmProceed`
- [x] 6.4 `runInstallSteps`: the 7 ordered steps (enable APIs → create/resume connection → create SA → grant Viewer → grant impersonation → finalize connection → create monitoring config), tracking `completedSteps` for partial-failure reporting
- [x] 6.5 `gcpRunStep`: shared retry wrapper (`installer.Retry` + `installer.Jitter`, up to 12 attempts / 5s base) for the `gcloud` steps (enable APIs, grant Viewer, grant impersonation), retrying only `installer.IsNotFoundErr`
- [x] 6.6 `gcpPermissionHint`: per-step (1, 3, 4, 5), role-specific hint appended when a step's error indicates a permission problem
- [x] 6.7 `gcpConnectionConflictHint`: explains a step-2 "name already taken" conflict when `findAllConnections` found nothing (object hidden in a different app/user context)
- [x] 6.8 `updateConnectionWithRetry`/`updateConnectionRetryable`: retries step 6 only on `"Constraints violated"` (excluding the permanent `"Unknown property"` case), 30s jittered initial delay + up to 30 × 5s jittered retries; does **not** treat a bare "permission" match as retryable (a permanent Dynatrace-side authorization failure must fail fast, not burn the full retry budget)
- [x] 6.9 `gcpPartialFailureHint`: print created resources + cleanup commands on partial failure
- [x] 6.10 `gcpWatchIngest`: tail ingest from start time; skip when start time is zero

## 7. Uninstall Workflow

- [x] 7.1 `UninstallGCP` entry point + `uninstallGCPWithRunner` core; `ConnectionExists`/`connectionExistsWithClient` in `pkg/installer/gcp/uninstall.go` — reports true only for a *complete* connection (an incomplete one is treated as not-yet-installed so `dtwiz setup` resumes it via install instead of rejecting it via update)
- [x] 7.2 Concurrent discovery of monitoring configs + connections (`installer.RunConcurrently`); `gcpGatherServiceAccounts` (bound SA emails from connections ∪ the deterministic email, when a project is active), de-duplicated and sorted
- [x] 7.3 Nothing-to-do short-circuit; `gcpUninstallPrintPreview` + `gcpUninstallBuildSteps`; `--dry-run` stop; `ConfirmProceed`
- [x] 7.4 `uninstallStepCount` (1 per monitoring config + 2 per service account + 1 per connection) and `runUninstallSteps`: best-effort, warn-and-continue, `errors.Join`

## 8. Update (In-Place Reconcile) Workflow

- [x] 8.1 `UpdateGCP` entry point + `updateGCPWithRunner` core in `pkg/installer/gcp/update.go`
- [x] 8.2 Concurrent discovery (monitoring configs + connections + `gcpAccountInfo`) via `installer.RunConcurrently`; `selectUpdatableConnection` (via `splitConnectionsByCompleteness`) requires exactly one complete connection, else abort with install/uninstall guidance
- [x] 8.3 `gcpUpdatePrintPreview`: env/project/service-account/connection (unchanged)/configuration + per-config update (or single create) steps; `--dry-run` stop; `ConfirmProceed`
- [x] 8.4 `reconcileMonitoring`: `updateMonitoring` each existing config in place via shared `buildMonitoringConfig`, or `createMonitoring` when none exists; auth chain untouched; ingest watch on success

## 9. CLI Wiring

- [x] 9.1 Add `installGcpCmd` (`dtwiz install gcp`, `cobra.NoArgs`) in `cmd/install.go` → `gcp.InstallGCP`
- [x] 9.2 Add `uninstallGcpCmd` (`dtwiz uninstall gcp`, `cobra.NoArgs`) in `cmd/uninstall.go` → `gcp.UninstallGCP`
- [x] 9.3 In `cmd/setup.go`: pre-check `azure.ConnectionExists` and `gcp.ConnectionExists` concurrently (`installer.RunConcurrently`), badge the GCP entry when a complete connection exists, route to `UpdateGCP` vs `InstallGCP`, and suppress the generic post-install watch for GCP (it runs its own)
- [x] 9.4 `pkg/recommender/recommender.go`: drop `ComingSoon`/"(coming soon)" for the GCP recommendation now that it is actionable
- [x] 9.5 Add `updateGcpCmd` (`dtwiz update gcp`, `cobra.NoArgs`) in `cmd/update.go` → `gcp.UpdateGCP`; import `pkg/installer/gcp`; validate platform token via `checkPlatformToken` before calling update; register with `updateCmd.AddCommand`

## 10. Tests

- [x] 10.1 `config.go` / `helpers_test.go`: `gcpServiceAccountEmail`, `findServiceAccountEmail`/`isServiceAccountEmail`, `gcpCreateServiceAccount` reuse-on-exists, idempotent deletes
- [x] 10.2 `dtapi_test.go`: schema enum extraction, `*_essential` filtering, constraint-violation parsing, create/update/find/delete via fake SDK responses, `splitConnectionsByCompleteness`
- [x] 10.3 `preflight_test.go` (via `install_test.go`/`uninstall_test.go` runners): CLI-missing, not-logged-in, no-active-project, Cloud Shell banner stripping
- [x] 10.4 `install_test.go`: full 7-step workflow with injected runner/sleeper/dtclient, existing-complete-connection redirects to update (`TestGCPInstallRedirectsToUpdateWhenConnectionExists`: note printed, no new connection created, 0 mutating gcloud calls), ambiguous-incomplete-connections abort, partial-connection resume (no duplicate `createConnection` call, IDs threaded through), dry-run, step-1/3/4/5 permission hints, step-6 propagation retries, step-6 fail-fast on permanent permission error, partial-failure hint
- [x] 10.5 `uninstall_test.go`: discovery, `connectionExistsWithClient` complete/incomplete/none, nothing-to-do, best-effort continue-on-failure, step count
- [x] 10.6 `update_test.go`: concurrent discovery+preflight, in-place reconcile of existing config(s), create-when-missing, `selectUpdatableConnection` abort cases (no complete / multiple connections)
- [x] 10.7 `pkg/installer`: `concurrent_test.go` (all-succeed, joined-errors), `retry_test.go` (`Jitter` bounds)
- [x] 10.8 `pkg/analyzer/detect_gcp_test.go`: `CleanGCloudConfigValue` (plain value, Cloud Shell notice, unset-with-notice)
- [x] 10.9 `make test` and `make lint`: all pass
- [x] 10.10 `cmd/update_test.go`: `TestUpdateGcpCmd_Registered` (gcp subcommand registered under update), `TestUpdateGcpCmd_RunE_ValidatesPlatformToken` (platform token validated before update runs)
