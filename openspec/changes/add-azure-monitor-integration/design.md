# Design

## Context

The Dynatrace Azure Monitor integration spans two control planes that must be mutated in a precise order:

1. **Dynatrace Platform** — a `builtin:hyperscaler-authentication.connections.azure` settings object (the "connection") and a `com.dynatrace.extension.da-azure` monitoring configuration.
2. **Microsoft Entra / Azure** — an App Registration and Service Principal, a federated identity credential (workload identity, no secret), and a Monitoring Reader role assignment.

The coupling is what makes this hard: the federated credential's subject is bound to the DT connection ID, so the connection must exist *before* the Azure credential is created; and the connection can only be finalized with the tenant and application IDs *after* the Azure Service Principal exists. Azure is eventually consistent, so several steps must tolerate "not yet propagated" errors.

All Azure mutations go through the `az` CLI (the standard tool on operator machines). All Dynatrace calls go through the `dtctl` SDK against the Platform URL with a Bearer platform token.

## Goals / Non-Goals

**Goals:**

- One command installs the full integration; one command removes exactly what it created.
- Secret-free auth (federated workload identity only).
- Safe to re-run after an interrupted install; the install-time stale-credential and orphaned-app handling recovers from the "leftover credential" problem.
- Defaults (locations, feature sets) sourced from the live extension schema, not hardcoded.
- Fully unit-testable without real `az` or API calls.

**Non-Goals:**

- A standalone `dtwiz update azure` subcommand (update is reached through `dtwiz setup`).
- Multi-subscription or management-group scope (always subscription scope).
- Configuring non-default feature sets or location subsets (zero-config: all locations, all `*_essential` feature sets).
- Integration tests against a real Azure tenant.

## Decisions

### 1. Federated identity credential, never a client secret

`az ad sp create-for-rbac` is called with `--create-password false`. Trust is established by a federated credential whose issuer is derived from the Dynatrace environment URL and whose subject is bound to the DT connection object ID. This means dtwiz never handles, prints, or stores an Azure client secret.

### 2. Fixed resource name `dtwiz-azure` as the ownership key

The DT connection, DT monitoring configuration, and Azure App Registration all share the constant name `dtwiz-azure`. The federated credential is named `dtwiz-azure-Federated-Credential`. A fixed name makes discovery for uninstall and update trivial and unambiguous, and anchors ownership verification during cleanup.

### 3. Dependency injection for testability

Exported entry points (`InstallAzure`, `UninstallAzure`, `UpdateAzure`) are thin wrappers that build a real `az` runner, a real sleep function, and a real Dynatrace API client, then delegate to an internal testable core. The Dynatrace client interface abstracts every Platform API call; the runner abstracts `az`; the sleep function abstracts backoff. Tests drive the full workflow with fakes and zero I/O.

### 4. Defaults read from the live extension schema

The installer fetches the latest extension version, then its monitoring-configuration schema, and reads the enum values: locations from the `dynatrace.datasource.azure:location` enum and feature sets from the `FeatureSetsType` enum (keeping only `*_essential`). Hardcoding these would drift as the extension evolves; an empty result is a hard error rather than a silent partial config.

The `*_essential` feature sets are the zero-config default for `da-azure` — they are the recommended baseline the extension ships for "enable monitoring for everything we detect" without opting into high-cardinality or high-cost feature sets (which are meant to be turned on deliberately per workload). Selecting all `*_essential` sets honors the project's "all defaults on" principle; enabling every feature set unconditionally would generate cost and data volume a user has not asked for.

### 5. Find all matches so cleanup heals partial state

Resource lookups return *all* objects with the matching name, not just the first. A healthy environment has one of each, but interrupted runs can leave duplicates; returning all lets uninstall and update remove every leftover.

### 6. Ownership verification before deleting Azure apps

Entra display names are not unique. During cleanup, an app is trusted for deletion only if it is bound to a discovered dtwiz connection, **or** (when found only by display-name lookup) it carries dtwiz's federated credential fingerprint — a credential with the expected name and issuer. Unverified same-name apps are skipped with a warning, never deleted. Deleting the App Registration cascades to its Service Principal and federated credentials, so one delete call cleans up all Entra artifacts.

### 7. Retry strategy for Azure propagation delays

Three distinct propagation hazards, each handled where it surfaces:

- **Step 3 (federated credential create):** an "already exists" error means a stale credential from a prior partial run — delete it and retry once.
- **Step 4 (SP object ID lookup):** retried up to 5 times with 3-second backoff when the SP is not yet visible; a permissions error fails immediately.
- **Step 6 (DT connection update):** retried up to 10 times with 5-second backoff only when the error indicates Azure has not yet propagated the federated credential; any other error stops retrying immediately.

### 8. Advisory, non-blocking permissions preflight

The installer checks whether the signed-in user can create role assignments at subscription scope. This check only ever warns — if the check cannot be completed or reports insufficient access, the install proceeds, and the actual role-assignment step surfaces the definitive error. This avoids false negatives blocking users whose accounts cannot read authorization data.

### 9. Update = in-place monitoring-config reconcile (auth untouched)

Update reconciles **only** the `da-azure` monitoring configuration to the latest schema-derived defaults and leaves the entire authentication chain — connection, Service Principal, federated credential, and Monitoring Reader role assignment — untouched. It discovers the existing configuration and the bound connection, resolves the active subscription, and rewrites the configuration with a single update call (or creates one when none exists), sharing the same body-builder logic with install.

This is the safest update strategy with respect to partial failure:

- **No teardown of working auth.** A reinstall would delete and recreate the connection and Service Principal, opening a window with no monitoring and risking a half-rebuilt integration if a later step fails — exactly the mixed state we want to avoid.
- **The propagation-retry hazard cannot occur.** That retry loop exists only because a reinstall recreates the Service Principal against a reused App Registration. An in-place update never recreates the SP, so that class of error cannot arise.
- **Blast radius is a single atomic write.** The update call either succeeds (new config live) or fails (prior config still live); there is no intermediate broken state, and the auth chain is never in scope.

The trade-off is that an update does **not** self-heal broken auth (for example, a manually deleted federated credential or revoked role). That is rare and user-caused; the explicit recovery path is `dtwiz uninstall azure` followed by `dtwiz install azure`. The update therefore requires exactly one complete connection (one that already carries its bound application ID) and aborts with install or uninstall guidance when zero or many are found.

### 10. Issuer URL derived from the environment URL

The federated credential issuer mirrors the apps URL's subdomain qualifier: `*.apps.dynatrace.com` → `https://token.dynatrace.com`; `*.dev.apps.dynatracelabs.com` → `https://dev.token.dynatracelabs.com`; `*.sprint.apps.dynatracelabs.com` → `https://sprint.token.dynatracelabs.com`. The audience is `<apps-host>/svc-id/com.dynatrace.da`.

## Risks / Trade-offs

- **Best-effort uninstall can leave residue.** Deletion steps log warnings and continue rather than aborting on the first failure, so one stuck resource doesn't strand the rest. Trade-off: a returned error may coexist with mostly-successful cleanup; the user is told which step failed.
- **Ownership check is fingerprint-based, not perfect.** A non-dtwiz app sharing the `dtwiz-azure` name but lacking the federated credential is correctly skipped; the residual risk is a stale dtwiz app whose credential was manually altered — it would be skipped and need manual cleanup. This is safer than deleting an app dtwiz may not own.
- **Propagation retries add latency.** Worst case is around 50 seconds on step 6 (10 retries × 5s) plus around 15 seconds on step 4. Mitigated by fast-fail on permanent errors and bounded attempts.
- **Schema-derived defaults depend on the extension being available.** If the `da-azure` extension has no versions or the schema yields no locations or no `*_essential` feature sets, the install fails fast with a clear error rather than creating a misconfigured monitoring configuration.
