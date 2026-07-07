# Design

## Context

The Dynatrace GCP integration spans two control planes that must be mutated in a precise order:

1. **Dynatrace Platform**: a `builtin:hyperscaler-authentication.connections.gcp` settings object (the "connection") and a `com.dynatrace.extension.da-gcp` monitoring configuration.
2. **Google Cloud**: a service account, a project-level `roles/viewer` binding on that service account, and a `roles/iam.serviceAccountTokenCreator` binding granting the Dynatrace principal impersonation rights on the service account.

The coupling is what makes this hard: the Dynatrace connection can only be finalized with the service account's email *after* the service account and its impersonation binding exist, and Dynatrace's live impersonation check (triggered by finalizing the connection) can fail for up to a couple of minutes while GCP IAM propagates the binding. GCP is eventually consistent in the same way Azure is, so several steps must tolerate "not yet propagated" errors — but the specific propagation hazards, error shapes, and retry windows differ enough from Azure's that they are handled with GCP-specific classification logic layered on shared retry infrastructure.

All GCP mutations go through the `gcloud` CLI (the standard tool on operator machines). All Dynatrace calls go through the `dtctl` SDK against the Platform URL with a Bearer platform token, via the same `ExtensionClient` the Azure installer uses.

## Goals / Non-Goals

**Goals:**

- One command installs the full integration; one command removes exactly what it created.
- No long-lived credential file: authentication is service-account impersonation, granted to Dynatrace's own principal.
- Safe to re-run after an interrupted install: a partial connection (created but not yet finalized) is resumed rather than treated as a conflict.
- Defaults (feature sets) sourced from the live extension schema, not hardcoded.
- Fully unit-testable without real `gcloud` or API calls.
- Share retry, concurrency, and Dynatrace-API-client infrastructure with the Azure installer rather than re-implementing it.

**Non-Goals:**

- A standalone `dtwiz update gcp` subcommand (update is reached through `dtwiz setup`).
- Multi-project scope (always the active `gcloud` project).
- Configuring non-default feature sets (zero-config: all `*_essential` feature sets).
- An advisory, non-blocking permissions preflight check like Azure's RBAC `checkAccess` — GCP has no equivalent cheap capability-check API, so permission problems are instead surfaced reactively via per-step hints (see Decision 6).
- Integration tests against a real GCP project.

## Decisions

### 1. Service-account impersonation, no key files

The Dynatrace principal is granted `roles/iam.serviceAccountTokenCreator` on a service account dtwiz creates; Dynatrace impersonates that service account to read project data. No service-account key file is ever created, downloaded, or stored — the same "no long-lived secret" property Azure's federated identity credential provides, achieved with GCP's native impersonation primitive instead.

### 2. Fixed resource name `dtwiz-gcp` as the identity key

The DT connection, DT monitoring configuration, and GCP service account (as its ID, which becomes the local part of its email) all share the constant name `dtwiz-gcp`. Unlike Azure — where the App Registration name is merely a discovery aid and ownership is verified separately via a federated-credential fingerprint — the GCP service account's email is *deterministic* (`dtwiz-gcp@<project>.iam.gserviceaccount.com`), so a service-account "already exists" response during creation is unambiguous evidence of dtwiz's own prior run and is reused directly rather than requiring a separate ownership check.

### 3. Dependency injection for testability

Exported entry points (`InstallGCP`, `UninstallGCP`, `UpdateGCP`) are thin wrappers that build a real `gcloud` runner, a real sleep function, and a real Dynatrace API client, then delegate to an internal testable core (`installGCPWithRunner`, `uninstallGCPWithRunner`, `updateGCPWithRunner`). The Dynatrace client interface (`dtclient`) abstracts every Platform API call; the runner (`cmdRunner`) abstracts `gcloud`; the sleep function abstracts backoff. Tests drive the full workflow with fakes and zero I/O. This mirrors Azure's pattern exactly.

### 4. Shared retry, concurrency, and API-client infrastructure with Azure

`pkg/installer/cmdrunner.go` (`CmdRunner` type, `RealRunner`, `IsNotFoundErr`), `pkg/installer/concurrent.go` (`RunConcurrently`), `pkg/installer/retry.go` (`Retry`, `RetryConfig`, `Jitter`), and `pkg/installer/extension_client.go` (`ExtensionClient`: Settings/Extension handlers, `DeleteConnection`, `FindAllMonitoringConfigs`, `DeleteMonitoringConfiguration`, schema/version lookups) were extracted from what was originally Azure-only code so GCP could reuse them instead of duplicating them. `pkg/installer/azure/*` was refactored onto the same infrastructure in this same change. Each cloud's package still owns its own schema-specific logic (connection shape, monitoring-config body, retry *classification*) — only the mechanical parts (run a command and capture output, run N functions concurrently and join their errors, retry-with-backoff-and-jitter, talk to the Settings/Extensions API) are shared.

### 5. Defaults read from the live extension schema

The installer fetches the latest `da-gcp` extension version, then its monitoring-configuration schema, and reads the `FeatureSetsType` enum, keeping only values ending in `_essential`. Hardcoding these would drift as the extension evolves; an empty result is a hard error rather than a silent partial config. Unlike Azure, GCP's monitoring configuration is scoped to a single project (`projectFiltering`) rather than a set of locations, so there is no separate location enum to read.

### 6. Reactive, not advisory-preflight, permission errors

GCP has no cheap equivalent of Azure's ARM `checkAccess` API to advise on likely permission gaps before mutating anything. Instead, `gcpPermissionHint` inspects the error from any `gcloud`-driven step (enable APIs, create service account, grant project IAM, grant SA IAM) for permission-denied signals and appends a targeted hint naming the specific IAM role likely missing (e.g. `roles/serviceusage.serviceUsageAdmin` for step 1, `roles/iam.serviceAccountAdmin` for step 3). This is strictly reactive — it never blocks or delays a step that would otherwise succeed, and it only fires after `gcloud` itself has already rejected the call.

### 7. Resumable partial installs instead of an install/update deadlock

Connections found by `findAllConnections` are split into *complete* (carry a bound service-account email — `splitConnectionsByCompleteness`) and *incomplete* (created in step 2 but never finalized in step 6, from an interrupted prior run):

- **Install** (`gcpResumableConnection`): a complete connection aborts the install (guidance: uninstall first); more than one incomplete connection aborts as ambiguous (guidance: uninstall then reinstall for a clean slate); exactly one incomplete connection is *resumed* — its object ID is reused in step 2 instead of creating a duplicate, and the workflow continues through steps 3–7 normally.
- **Update / `ConnectionExists`**: only a complete connection counts as "configured". An incomplete connection is treated as not-yet-installed, so `dtwiz setup` badges it `[install]` (which resumes it) rather than `[update]` (which would reject it for lacking a service account and point back to install — the deadlock this design avoids).

Without this, a connection left behind by an interrupted install could cause `dtwiz setup` to route to update, update to reject and point to install, and install to reject because the (incomplete) connection already exists — a dead end only escapable by manually running `dtwiz uninstall gcp` first.

### 8. Retry classification keyed to the verified signal, not a broad keyword

The Dynatrace connection-finalization retry (`updateConnectionWithRetry`, step 6) retries only on an error containing `"Constraints violated"` — the verified shape of the live impersonation-propagation failure — while excluding `"Unknown property"` (a permanent schema mismatch that also happens to be reported as a constraint violation). It deliberately does **not** treat every error containing the word "permission" as retryable: a permanent Dynatrace-side authorization failure (for example, a platform token lacking write scope) also often contains that word, and retrying it for the full ~3-minute budget would only delay an error that can never resolve by waiting. The same principle applies to `gcloud`-step retries (`IsNotFoundErr`, shared with Azure): only "not yet propagated" signals are retried, everything else fails immediately.

### 9. Jittered backoff on every fixed-delay retry

All three retry loops (`gcloud`-step propagation, Dynatrace connection-finalization, and Azure's Service Principal lookup) add `Jitter` (up to 50% extra random delay) on top of their base interval. Fixed delays decorrelate poorly under concurrent installs — e.g. a CI matrix installing several projects at once — where every instance would otherwise sleep for exactly the same interval and retry in lockstep, turning a single propagation delay into a synchronized burst against the same `gcloud`/Dynatrace endpoint.

### 10. Update = in-place monitoring-config reconcile (auth untouched)

Update reconciles **only** the `da-gcp` monitoring configuration to the latest schema-derived defaults and leaves the entire authentication chain unchanged: connection, service account, Viewer binding, and impersonation binding. It discovers the existing configuration(s) and the one complete connection concurrently, and rewrites the configuration with a single update call per existing config (or creates one when none exists), sharing the same body-builder logic with install. This mirrors Azure's update design and rationale exactly: no teardown of working auth, no propagation-retry hazard (nothing that requires waiting is touched), and a single atomic write as the only failure mode.

`selectUpdatableConnection` requires exactly one *complete* connection (via the same `splitConnectionsByCompleteness` split used for install's resume decision) and aborts with install/uninstall guidance when zero or many are found.

## Risks / Trade-offs

- **Best-effort uninstall can leave residue.** Deletion steps log warnings and continue rather than aborting on the first failure, so one stuck resource doesn't strand the rest. Trade-off: a returned error may coexist with mostly-successful cleanup; the user is told which step failed. Concurrent discovery errors from `RunConcurrently` are now joined (not just the first reported), so both failures are visible when they co-occur.
- **Resumable-partial-install relies on the deterministic name being unique.** If more than one incomplete connection is ever found, the system refuses to guess which one to resume and asks for a clean slate instead — correct, but means two interrupted installs in a row require a manual `dtwiz uninstall gcp` before a third attempt can succeed automatically.
- **No advisory permissions preflight.** Compared to Azure's non-blocking RBAC check, GCP permission problems are only discovered when a step actually fails. Mitigated by per-step, role-specific hints so the failure is still immediately actionable.
- **Propagation retries add latency.** Worst case is around 3 minutes on step 6 (30s initial + up to 30 × 5s, jittered) plus up to a minute across the `gcloud` steps (up to 12 × 5s each, jittered). Mitigated by fast-fail on permanent errors (schema mismatches, non-propagation failures) and bounded attempts.
- **Schema-derived defaults depend on the extension being available.** If the `da-gcp` extension has no versions or the schema yields no `*_essential` feature sets, the install fails fast with a clear error rather than creating a misconfigured monitoring configuration.
