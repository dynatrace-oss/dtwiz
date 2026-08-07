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

- **Token permissions**: Both `DeactivateExtension` and `DeleteExtensionVersion` require `extensions.write` scope. Users whose platform token lacks this scope will see an advisory warning; local cleanup proceeds normally.
