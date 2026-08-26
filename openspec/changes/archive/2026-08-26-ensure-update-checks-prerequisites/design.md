# Design: ensure-update-checks-prerequisites

## Context

`dtwiz install otel` (with `--experimental`) already performs two tenant-side steps after the user confirms:

1. Activates the OTel Host Monitoring extension via `activateHostMonitoringExtensionFn`.
2. Waits for pipelines, rebuilds the route plan, then applies it via `applyGrailPlan`.

Before confirmation, the preview shows both the extension status (`buildExtensionActivationPreviewFn` + `printExtensionActivationPreview`) and the route plan (`buildGrailRoutePlans` + `printGrailPlan`).

`activateHostMonitoringExtensionFn`, `buildExtensionActivationPreviewFn`, and `waitForGrailPipelinesFn` were already injectable (package-level `var` function pointers) before this change. `buildGrailRoutePlansFn` is added in this change to `grail_routes.go` to make the preview step stubbable in tests.

`updateDynatraceCollector` in `pkg/installer/otel/update_dynatrace.go` is the primary file changed. It is restructured so that when the regenerated config matches the file on disk, the function skips the config write and collector restart but still runs the prerequisite preview and post-confirmation reconciliation steps.

## Goals / Non-Goals

**Goals:**

- Show extension status and route plan in the `updateDynatraceCollector` preview, in the same order as install: extension first, then routes.
- After confirmation, activate the extension and apply any missing or disabled routes, using the same post-confirmation sequence (activate → wait → rebuild → apply) as install.
- Gate behind the same guards already used in install: `featureflags.IsEnabled(featureflags.Experimental)` and `platformToken != ""`.

**Non-Goals:**

- No changes to `updateOtelConfig` (third-party collector path).
- No new feature flags, CLI flags, or packages.
- No changes to the extension or route logic itself.

## Decisions

### Reuse install's call sites verbatim — no shared helper

**Decision:** Copy the six call sites from `InstallOtelCollectorWithProject` into `updateDynatraceCollector` rather than extracting a shared helper.

**Rationale:** The install and update flows share the same logical steps but differ enough in surrounding structure (config diff vs. collector plan preview, different confirmation text, different post-confirmation work) that a shared helper would need parameters covering both contexts. Three callers don't yet exist to justify the abstraction, and the six call sites are already self-documenting via the injectable function-pointer names. The existing test pattern (override the `*Fn` vars in tests) works identically whether the calls are in install or update.

### Preview insertion point

**Decision:** The extension + route preview block is inserted after the connected-services section and before the dry-run short-circuit, matching the install flow's position relative to the confirmation prompt.

**Rationale:** The preview must appear before the prompt. Placing it after connected services keeps the config-diff and restart sections visually grouped, and the extension + route block logically follows as "tenant-side state" — the same ordering install uses.

### Post-confirmation insertion point

**Decision:** Extension activation and route application run before the config write and collector restart.

**Rationale:** Routes require the extension's pipelines to exist. Activating first, then waiting for pipelines, then applying routes ensures the bounded-wait pattern in `waitForGrailPipelinesFn` has the best chance of finding a freshly provisioned pipeline in the same run. The collector restart is independent and can follow.

### Plan rebuild before apply

**Decision:** Rebuild the route plan immediately before applying (same as install), rather than reusing the preview-time snapshot.

**Rationale:** Extension activation is async (202 Accepted from the Hub). A pipeline that was absent at preview time may be present by the time routes are applied. Reusing the snapshot would permanently skip that signal in the same run. The rebuild is a read-only API call and is cheap.

## Risks / Trade-offs

- **Extension activation errors are advisory:** `activateHostMonitoringExtensionFn` warns and returns rather than aborting. This is the same behavior as install and is intentional — a broken extension should not prevent a credential update.
- **Route apply errors are advisory:** `applyGrailPlan` failures print a warning and do not abort. Same as install.
- **Preview API calls add latency:** `buildExtensionActivationPreviewFn` and `buildGrailRoutePlans` make API calls at preview time. If the platform token is valid, these are fast read-only calls. If the API is unavailable, both paths already print a warning and continue (same behavior as install).
- **`platformToken` is `""` for access-token-only environments:** The guard `platformToken != ""` short-circuits both preview and post-confirmation steps cleanly. No behavior change for those environments.
