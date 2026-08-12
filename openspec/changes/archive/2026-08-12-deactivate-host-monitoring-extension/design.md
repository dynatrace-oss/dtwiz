# Design: deactivate-host-monitoring-extension

## Context

`dtwiz install otel --experimental` installs and activates `com.dynatrace.extension.opentelemetry` via two steps: `EnsureInstalled` (hub install) and `ActivateExtension` (POST environment-configuration). The uninstall side (`UninstallOtelCollector`) is entirely local — it kills processes and removes directories — and has no API calls or knowledge of credentials.

The Dynatrace Platform Extensions v2 API requires a two-step removal sequence:

```text
DELETE /platform/extensions/v2/extensions/{name}/environment-configuration  → 204
DELETE /platform/extensions/v2/extensions/{name}/{version}                  → 202
```

The environment configuration must be deactivated before the version can be deleted; the API returns 409 if the version delete is attempted while a configuration is still active.

The dtctl SDK (`github.com/dynatrace-oss/dtctl/sdk@v0.2.0`) has no methods for either operation, so both require raw REST calls — the same gap that forced `ActivateExtension` to be a direct HTTP call in `ExtensionClient`.

The version to delete is not stored locally; it must be discovered at uninstall time via `LatestExtensionVersion`, which is already used on the install side.

## Goals / Non-Goals

**Goals:**

- Remove `com.dynatrace.extension.opentelemetry` from the tenant when `dtwiz uninstall otel --experimental` runs
- Advisory failure mode: warn on error, never abort the local cleanup
- Show the extension removal in the uninstall preview
- Keep credentials out of `UninstallOtelCollector` when experimental is off

**Non-Goals:**

- Tracking whether dtwiz was the one that installed the extension (no provenance file)
- Deleting monitoring configurations (the OTel host monitoring extension uses none)
- Removing other extension versions if multiple are installed (we only installed one)

## Decisions

**Decision 1: Two API calls — `DeactivateExtension` then `DeleteExtensionVersion`.**

The Dynatrace Extensions v2 API requires deactivating the environment configuration before the version can be deleted (409 is returned otherwise). `DeactivateExtension` (DELETE `/environment-configuration`) runs first; `DeleteExtensionVersion` (DELETE `/{version}`) follows. Both are raw REST calls since the dtctl SDK exposes neither operation.

**Decision 2: `UninstallOtelCollector` gains `envURL, platformToken string` parameters; `cmd/uninstall.go` always resolves credentials.**

The cleanest way to pass credentials into the extension removal step, which lives in `pkg/installer/otel/`. `cmd/uninstall.go` always calls `getDtEnvironment()` and passes the real values — consistent with `uninstallAzureCmd` and `uninstallGCPCmd`. `UninstallOtelCollector` internally gates the extension removal on `featureflags.Experimental`, so no conditional logic is needed in the cmd layer.

Alternative considered: resolve credentials inside `UninstallOtelCollector`. Rejected — it would import `cmd`-layer logic into `pkg/installer`, violating the existing layering.

**Decision 3: Extension removal runs after local cleanup, not before.**

Local cleanup (kill + rm -rf) is the primary operation; extension removal is secondary. Running it last means a failed API call never blocks local cleanup, and the user gets the most important part done regardless.

**Decision 3a: Grail route removal runs before extension deactivation.**

The `install otel --experimental` flow adds dynamic routing rules to OpenPipeline for metrics, logs, and spans. Each routing entry references the objectId of the pipeline provisioned by the OTel extension. When the extension is removed, the pipeline object is deleted, leaving the routing entry as a dangling reference. Removing routes first avoids orphaned entries in the customer's OpenPipeline configuration.

Route removal reuses the existing `grailRouteClient` interface from `grail_routes.go` — specifically `checkPipeline` (to find the pipeline's objectId) and `getRoutingEntries` / `putRoutingEntries` (to filter and write back the updated entry list). No new API surface is needed.

The full sequence within `deactivateHostMonitoringExtension` is:

1. Remove Grail routes for metrics, logs, spans (advisory per-signal)
2. Deactivate extension environment configuration
3. Look up latest installed version
4. Delete extension version

Failure at step 1 for one signal does not block the remaining signals or steps 2–4. The routes are Settings objects independent of each other.

**Decision 4: `deactivateHostMonitoringExtensionFn` test hook, matching install side.**

`activateHostMonitoringExtension` is stubbed via a package-level `var` for tests. The deactivation function uses the same pattern so tests can inject a no-op or a failure without making real API calls.

**Decision 5: Three-way prompt replaces yes/no when experimental is enabled.**

When `--experimental` is active, the simple `"Proceed with uninstall? [Y/n]"` prompt is replaced by a three-option choice:

- `[1] Delete all` (default): removes the collector and the OTel Host Monitoring extension from the tenant.
- `[2] Only collector`: removes the collector and all local artifacts but leaves the extension on the tenant.
- `[3] Cancel`: aborts without making any changes.

`--yes`/`AutoConfirm` selects option 1 (delete all) without prompting, preserving the current default behavior for scripted use. When experimental is off, the original yes/no prompt is shown unchanged.

The prompt result (`uninstallDecision`) drives whether `deactivateHostMonitoringExtensionFn` is called, replacing the earlier direct `featureflags.IsEnabled(Experimental)` guard on the deactivation call. `promptUninstallDecisionFn` is a package-level var so tests can inject specific decisions without reading stdin.

Alternative considered: a `--keep-extension` CLI flag. Rejected — interactive users need a clear in-flow choice, and a flag would be invisible unless the user already knows the behavior. The prompt surfaces the choice at the right moment for all users.

## Risks / Trade-offs

- **Extension was pre-existing**: If the user had the extension installed before running `dtwiz install otel --experimental`, uninstall will remove it. No mitigation — acceptable given the explicit `--experimental` gate signals awareness of experimental behavior.

- **Version mismatch**: `LatestExtensionVersion` may return a different version than the one originally installed if the user upgraded the extension externally. Deleting the latest installed version is the best we can do without provenance tracking.

- **Token permissions**: `DeactivateExtension`, `DeleteExtensionVersion`, and route removal via `putRoutingEntries` all require write scopes (`extensions.write`, `settings:objects:write`). Users whose platform token lacks these scopes will see per-operation advisory warnings; local cleanup proceeds normally.

- **Routes added externally**: If the user or another tool added additional routing entries referencing the OTel pipeline, those entries will also be removed since we filter by pipeline objectId. Acceptable — the pipeline itself is being deleted, so any reference to it would become dangling regardless.
