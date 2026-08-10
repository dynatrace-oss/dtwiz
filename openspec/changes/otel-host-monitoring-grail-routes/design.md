# OTel Host Monitoring: Smartscape on Grail Routes Design

## Context

The `add-otel-host-monitoring` change configures the managed OTel Collector to emit host metrics, logs, and spans formatted for the Dynatrace **OpenTelemetry Host Monitoring** extension. When that extension is active in the environment, it provisions a processing pipeline named "OpenTelemetry Host Monitoring" for each signal type. Example pipeline objects observed on a live tenant (`builtin:openpipeline.{metrics,spans,logs}.pipelines`) carry the extension-owned `value.customId` entries `extension.opentelemetry-metrics`, `extension.opentelemetry-spans`, and `extension.opentelemetry-logs`, each with the display name "OpenTelemetry Host Monitoring". Their `externalId` is platform-assigned and follows the pattern `com.dynatrace.extension.opentelemetry_extension.opentelemetry-<signal>`, always starting with the extension name followed by `_extension.`. This makes it a more reliable ownership signal than the `extension_name` entry inside `value.metadataList`, which is extension-internal metadata the extension writes into its own payload.

What the extension does **not** provision is the OpenPipeline **dynamic routes** (Settings > Process and contextualize > OpenPipeline > Dynamic routes, one table per signal type: `/openpipeline-metrics/dynamic-routes`, `/openpipeline-logs/dynamic-routes`, `/openpipeline-spans/dynamic-routes`). Without a dynamic route matching host telemetry and pointing at the "OpenTelemetry Host Monitoring" pipeline, incoming OTLP host data lands on the default pipeline and never produces `OTEL_HOST` / `OTEL_PROCESS` Smartscape entities. The [docs](https://docs.dynatrace.com/docs/observe/infrastructure-observability/extensions/opentelemetry-host-monitoring#smartscape-on-grail) instruct users to create these three routes manually.

**Per-signal schema reference.** The three signals and the exact schema IDs and matcher constants involved, wherever `<signal>` is used as a placeholder elsewhere in this document:

| Signal | Pipeline schema | Routing schema | Matcher constant |
|--------|------------------|-----------------|-------------------|
| metrics | `builtin:openpipeline.metrics.pipelines` | `builtin:openpipeline.metrics.routing` | `grailMatcherMetrics` |
| logs | `builtin:openpipeline.logs.pipelines` | `builtin:openpipeline.logs.routing` | `grailMatcherLogs` |
| spans | `builtin:openpipeline.spans.pipelines` | `builtin:openpipeline.spans.routing` | `grailMatcherSpans` |

This table is the single source of truth for the per-signal schema pairing. The implementation MUST mirror it exactly (for example, as a `grailSignals` slice under `pkg/installer/otel/`, task 1.1), so a schema change shows up as a one-line diff here instead of requiring every `<signal>`-templated mention below to be re-derived.

Each routing table is backed by a single environment-scoped settings object under its routing schema above (confirmed for all three signals against a live tenant), whose `value.routingEntries` is an ordered array. An entry created via the UI reads:

```json
{
  "enabled": true,
  "pipelineType": "custom",
  "pipelineId": "<objectId of the OpenTelemetry Host Monitoring logs pipeline>",
  "matcher": "isNotNull(host.id) and isNotNull(host.name) and matchesValue(dt.openpipeline.source, \"/api/v2/otlp/v1/logs\")",
  "description": "OpenTelemetry Host Monitoring"
}
```

The `pipelineId` equals the Settings **objectId** of the extension-owned pipeline (the one whose `value.customId` is `extension.opentelemetry-logs`) from the `builtin:openpipeline.logs.pipelines` list. The routing schema declares `pipelineId` as a Settings reference (`"type": "setting"`), so only the objectId (never the `customId` string) satisfies it; a route created with the `customId` string is rejected with `400 Constraints violated … Must be of type setting`.

dtwiz already writes Settings 2.0 objects elsewhere through `installer.ExtensionClient` ([pkg/installer/extension_client.go](../../../pkg/installer/extension_client.go)) and the vendored dtctl `settings.Handler`; the Azure and GCP installers ([pkg/installer/gcp/dtapi.go](../../../pkg/installer/gcp/dtapi.go)) are the reference examples for create/get/update-with-`If-Match`. This change reuses that machinery to reconcile the three dynamic routes.

## Goals / Non-Goals

**Goals:**

- After a successful host-monitoring collector install, ensure a dynamic route exists for metrics, logs, and spans that routes host OTLP telemetry into the "OpenTelemetry Host Monitoring" pipeline, using the documented matching conditions.
- Don't create a route that already exists, and never modify or delete existing routes; running `install otel` again after the routes exist makes no changes.
- Skip safely (do not fail the install) when the target pipeline cannot be resolved.
- Reuse the existing settings-write machinery, preview/confirm UX, and experimental gate.

**Non-Goals:**

- Creating, editing, or version-pinning the "OpenTelemetry Host Monitoring" pipelines. Those are owned by the extension.
- Removing or reconciling routes on `uninstall otel` or `update otel`.
- Reconciling drifted routes back to the documented condition. The chosen safety model is additive-only; see Decision 3.
- Routing for signal types other than metrics, logs, and spans.
- Standalone command surface. The reconciliation runs only as a step inside the host-monitoring install flow; see Decision 1.

## Decisions

### Decision 1: Run as a final step of the host-monitoring install flow

The reconciliation is invoked from the host-monitoring install path in [pkg/installer/otel/otel.go](../../../pkg/installer/otel/otel.go) after `collectorPlan.execute(...)` returns success, in the non-dry-run branch, and only when `featureflags.IsEnabled(featureflags.Experimental)` is true. It receives the already-resolved `envURL` and `platformToken` that the collector install threads through, and constructs an `installer.ExtensionClient` via `installer.NewExtensionClient(envURL, platformToken)`.

Why over a standalone command: the routes are meaningless without the host-monitoring collector and extension, so binding them to that flow keeps a single zero-config action. A separate command would add surface area and a second auth/preview path for a step that has no independent use. An earlier draft kept a standalone `ReconcileGrailRoutes` function around for a hypothetical future `update otel` entry point; it was removed as dead code once no caller ever used it (see Decision 9). If that entry point is wanted later, it can be built from `buildGrailRoutePlans` and `applyGrailPlan`, the same building blocks the install flow already uses.

### Decision 2: Discover the pipeline by listing the schema and matching extension ownership; never construct or assume an identifier

The matcher conditions are hardcoded constants. The pipeline itself is **not** identified by a constructed or assumed identifier; dtwiz lists every object in the signal's pipeline schema and picks the one owned by the OpenTelemetry Host Monitoring extension:

```text
GET /platform/classic/environment-api/v2/settings/objects?schemaIds=builtin:openpipeline.<signal>.pipelines&scopes=environment&fields=objectId,scope,schemaId,schemaVersion,externalId,summary,value,modificationInfo
```

(issued via the dtctl SDK's `Settings.ListObjects`, which already requests exactly this field set, so no raw HTTP call is needed here.) The live response for the metrics schema on a live tenant, trimmed to the fields this change reads (the full object also carries a large `smartscapeNodeExtraction` processor block that is extension-internal and irrelevant here):

```json
{
  "items": [{
    "objectId": "<settings-object-id>",
    "externalId": "com.dynatrace.extension.opentelemetry_extension.opentelemetry-metrics",
    "summary": "OpenTelemetry Host Monitoring",
    "scope": "environment",
    "schemaId": "builtin:openpipeline.metrics.pipelines",
    "schemaVersion": "1.70",
    "value": {
      "metadataList": [
        {"entryKey": "extension_name", "entryValue": "com.dynatrace.extension.opentelemetry"},
        {"entryKey": "extension_version", "entryValue": "3.1.1"}
      ],
      "customId": "extension.opentelemetry-metrics",
      "displayName": "OpenTelemetry Host Monitoring"
    }
  }],
  "totalCount": 1
}
```

Note `externalId` follows the pattern `<extension-name>_extension.<object-name>`, always prefixed with `otelExtensionName` (constant `com.dynatrace.extension.opentelemetry`). For each returned object, dtwiz checks whether `obj.ExternalID` starts with `otelExtensionName+"_"` (i.e. `com.dynatrace.extension.opentelemetry_`) to identify extension ownership. The first match is the target pipeline; its Settings **objectId** (`obj.ObjectID`), not its `customId`, is what gets threaded into the routing entry (Decision 4), because the routing schema declares `pipelineId` as a Settings reference (`"type": "setting"`) that only accepts an objectId.

Three earlier approaches were tried and rejected. They are recorded here so they are not tried again:

1. **Filtering the Settings list by `externalIds=extension.opentelemetry-<signal>`.** This assumed the extension's `value.customId` string was also the Settings-object `externalId`. The actual `externalId` follows the longer pattern `com.dynatrace.extension.opentelemetry_extension.opentelemetry-<signal>`, not the short customId form, so the filter always returned an empty list. Even with the corrected value, this approach requires hardcoding a per-signal suffix that is extension-internal. The prefix check (`strings.HasPrefix(obj.ExternalID, otelExtensionName+"_")`) avoids both problems without needing to know the per-signal suffix.
2. **Matching on `value.metadataList` for `{"entryKey": "extension_name", "entryValue": "com.dynatrace.extension.opentelemetry"}`.** `extension_name` is a metadata value the extension writes into its own `value` payload. It is extension-internal and carries no stronger guarantee of accuracy than any other field the extension populates. The `externalId` is platform-assigned from the extension's registered name, so it is a more reliable reference that does not depend on what the extension writes inside `value`.
3. **Matching on `value.customId == "extension.opentelemetry-<signal>"` after listing unfiltered.** This correctly finds the pipeline (the customId is genuinely present), but then using that same customId string as the routing entry's `pipelineId` fails at write time. The exact request and response, captured live (metrics signal):

   ```text
   POST /platform/classic/environment-api/v2/settings/objects
   [{"schemaId":"builtin:openpipeline.metrics.routing","scope":"environment","value":{"routingEntries":[{"enabled":true,"pipelineType":"custom","pipelineId":"extension.opentelemetry-metrics","matcher":"...","description":"OpenTelemetry Host Monitoring"}]}}]

   → 400 Bad Request
   [{"code":400,"error":{"code":400,"message":"Constraints violated.","constraintViolations":[{"path":"builtin:openpipeline.metrics.routing/0/routingEntries/0/pipelineId","message":"Must be of type setting","parameterLocation":"PAYLOAD_BODY","location":null}]},"invalidValue":{"routingEntries":[{"enabled":true,"pipelineType":"custom","pipelineId":"extension.opentelemetry-metrics","matcher":"...","description":"OpenTelemetry Host Monitoring"}]}}]
   ```

   because the field expects a Settings objectId, not a customId string.

The `externalId` prefix lookup avoids re-introducing any of these mistakes: it does not require knowing the per-signal suffix, does not depend on extension-internal metadata, and naturally yields the objectId needed for the write. If the extension is configured, its pipeline is found by the platform-assigned identity, not by guessing what the extension wrote into its value.

An empty result (no object in the schema is extension-owned) means the extension is not yet active for that signal; dtwiz skips the route with an informational line. Any API error is propagated as a warning (see Decision 5).

`Settings.Get(ctx, id)` is not used here: that endpoint expects a Settings objectId and 404s for anything else, so it cannot itself locate the pipeline; it would only be useful once the objectId is already known, which is what the list-and-match above produces.

A missing pipeline never fails the install. If the extension is activated later, a second `install otel` run will reconcile the skipped routes.

### Decision 3: Additive and re-enabling only -- never modify conditions or delete routes

For each signal type, dtwiz reads the current routing object's `routingEntries` and looks for an entry whose `pipelineId` matches the resolved pipeline. Three outcomes are possible:

- **Entry absent:** dtwiz appends a new entry and preserves all existing entries unchanged.
- **Entry present and `enabled: true`:** no-op.
- **Entry present and `enabled: false`:** dtwiz sets `enabled: true` on that entry and writes the object back. The `matcher`, `description`, `pipelineType`, and all other fields are left exactly as they are. All other entries in `routingEntries` are preserved unchanged.

The re-enable case exists because a disabled route silently prevents Smartscape entities from forming. If dtwiz found the route and did nothing, the user would have no signal that routing is broken. Re-enabling is the helpful action and it touches only the one boolean field, so it is safe.

The idempotency check matches on the resolved `pipelineId`. This means a user who broadened the `matcher` by hand (the docs note this is supported) is still recognized as already routed and not duplicated. A match means "an entry to this pipeline exists," not "a byte-identical matcher exists." This avoids creating a duplicate entry next to a user's customized one.

dtwiz never modifies `matcher`, `description`, `pipelineType`, or any other field on an existing entry. It never deletes entries. Re-running after the route is enabled is a no-op.

### Decision 4: Append to the single per-signal routing object with optimistic concurrency

The dynamic-routing table for a signal type is a **single environment-scoped settings object** under schema `builtin:openpipeline.<signal>.routing` (`multiObject: false, maxObjects: 1` per the schema definition), whose `value.routingEntries` is an ordered array of route entries. Reconciliation is a read-modify-write of that one object when it exists, or a create when it doesn't:

1. List the schema for the signal (schema `builtin:openpipeline.<signal>.routing`, scope `environment`). Per the Dynatrace extensions team, this routing singleton is **never** auto-provisioned on any tenant; it exists only once a user (or dtwiz) creates the first route by hand or via this reconciliation. So on any tenant where no dynamic route has ever been configured for that signal, the list returns zero items. This is a valid, expected state (not an error): dtwiz treats it as an empty `routingEntries` with no object to update yet.
2. If no entry already targets the resolved pipeline (Decision 3), build the new entry to append (or, if the routing object doesn't exist, the sole first entry).
3. If the routing object existed, PUT the modified object back with the object's `version`/`schemaVersion` sent as `If-Match`, which causes a concurrent change to fail cleanly rather than produce a lost update. If it didn't exist, POST a new object for the schema (scope `environment`) with `value.routingEntries` set to the one new entry; no `If-Match` applies to a create.

A route entry has the confirmed shape:

```json
{
  "enabled": true,
  "pipelineType": "custom",
  "pipelineId": "<resolved OpenTelemetry Host Monitoring pipeline objectId>",
  "matcher": "<documented matching condition for the signal>",
  "description": "OpenTelemetry Host Monitoring"
}
```

Confirmed against a live tenant for all three signals (schemas `builtin:openpipeline.{logs,metrics,spans}.routing`; on this tenant the routing objects existed because the three routes had already been created manually, by hand, following the docs' instructions, before this investigation): every OTel entry's `matcher` equals the documented condition (metrics uses uppercase `AND`; logs/spans use lowercase `and`), `description` is `"OpenTelemetry Host Monitoring"`, and `pipelineId` equals the extension-owned pipeline's Settings objectId resolved in Decision 2. The spans object also carried an unrelated user route next to the OTel entry. This confirms `routingEntries` can hold multiple entries and that the append MUST preserve them. On a second tenant the routing objects did **not** exist for any signal even though the extension was active, matching the extensions team's confirmation that these objects are never auto-provisioned: absent manual route creation, this is the default state on every tenant, not a rare edge case the create path merely happens to also handle.

The write reuses the dtctl `settings.Handler` on `ExtensionClient` (platform token, apps URL): `Settings.ListObjects` for the read, a raw HTTP PUT (mirroring [pkg/installer/gcp/dtapi.go](../../../pkg/installer/gcp/dtapi.go)'s get-then-PUT-with-`If-Match` pattern) for the update, and the SDK's `Settings.Create` for the singleton-absent case. Constraint-violation response bodies are parsed and surfaced on the PUT path (the shared `checkResponse` path discards them), reusing the `constraintViolation` helper pattern already established in the GCP/Azure installers; this is how the `pipelineId`-must-be-a-setting-reference rejection in Decision 2 was diagnosed. The Dynatrace UI reads/writes these objects via the Platform settings API (`/platform/settings/v1/objects`); the dtctl handler's classic path (`/platform/classic/environment-api/v2/settings/objects`) returns and accepts the same objects.

A whole-object PUT replaces `routingEntries`, so the appended array MUST carry every pre-existing entry unchanged. Dropping or reordering entries would silently disable users' existing routes.

Why over the thin `pkg/client` typed client: every existing settings-object write in the repo goes through `ExtensionClient` / dtctl, which already handles the apps-URL mapping, auth scheme, pagination, and retries. Reusing it keeps this consistent with Azure/GCP and avoids reimplementing Settings 2.0 plumbing.

### Decision 5: Route plan shown in the install preview; no separate confirmation prompt

The route plan is built during the install preview phase (before the existing "Proceed with installation?" prompt) and shown as an unnumbered section below the auto-instrumentation plan. After the user confirms the single install prompt, the collector is installed and the routes are applied. No second confirmation prompt is shown for routes.

- **Gate:** the entire step is behind `featureflags.IsEnabled(featureflags.Experimental)` and requires a non-empty `platformToken`. When off, nothing about `install otel` changes and no OpenPipeline API call is made.
- **Preview:** `buildGrailRoutePlans(envURL, platformToken)` is called during the preview phase; `printGrailPlan` prints one line per signal (create / re-enable / already configured / pending). Routes that are pending or already configured are still shown so the user can see the full picture. The plan shown here is a snapshot as of preview time, before extension activation has run; it is rebuilt right before being applied (Decision 8). Since a `grailActionSkip` here is never a final decision (the plan is always re-evaluated after activation), `printGrailPlan` labels it "pending — extension not active yet" rather than "skip", reserving "skip" for `printGrailApplyResults`, where it reflects the actual outcome after dtwiz has tried to install and activate the extension.
- **Plan-build failures:** if `buildGrailRoutePlans` fails (API error, auth failure), a warning is printed and the routes section is omitted from the preview. The install continues normally.
- **Apply failures:** if `applyGrailPlan` fails for a signal after confirmation, a warning is printed for that signal. The failure does not affect the install result. The user can re-run `install otel` to retry.
- **Dry-run:** `--dry-run` builds and prints the plan but returns before the confirmation prompt, so no routes are applied.
- **Auto-confirm:** `--yes`/`-y` sets `installer.AutoConfirm` on the single install confirmation prompt; this covers the route apply step as well.
- **No standalone entry point:** an earlier draft kept a `ReconcileGrailRoutes(envURL, platformToken, dryRun bool)` function around, with its own plan/preview/confirm/apply cycle, for potential future use from `update otel`. It was removed once nothing ever called it; see Decision 9.

### Decision 6: On 401/403, surface diagnostic context rather than a confident diagnosis

Every OpenPipeline call in this change (pipeline list, routing list, routing update, routing create) uses the same platform token as every other settings-object write in dtwiz. Live testing against a live tenant with an under-permissioned token produced:

```text
GET /platform/classic/environment-api/v2/settings/objects?schemaIds=builtin:openpipeline.metrics.pipelines&scopes=environment&fields=...
→ 403 Forbidden
{"error":{"code":403,"message":"Access denied"}}
```

`README.md`'s platform-token scope table lists dozens of scopes across several API groups; a bare "Access denied" gives the user no way to know where to start looking. The two scopes that normally cover this kind of call are:

| Operation | Scope that normally covers it |
|-----------|-------------------------------|
| List/read pipelines or routing objects (`checkPipeline`, `getRoutingEntries`) | `settings:objects:read` |
| Create/update the routing object (`putRoutingEntries`, `createRoutingObject`) | `settings:objects:write` |

**Important limitation dtwiz cannot resolve from the response alone:** Dynatrace IAM policies can grant `settings:objects:read`/`write` restricted to a specific list of schema IDs. A token can genuinely hold the scope and still be denied here if its policy's schema list doesn't include `builtin:openpipeline.<signal>.pipelines`/`.routing`. The 403 body is identical in both cases (missing scope entirely, vs. scope present but schema-restricted), so dtwiz **cannot** distinguish "you don't have this scope" from "you have this scope but not for this schema"; asserting either as the cause would sometimes be a wrong diagnosis. The right approach is to give the user everything needed to check both possibilities, not to guess which one it is.

dtwiz detects a 401/403 via the dtctl SDK's sentinel errors (`errors.Is(err, httpclient.ErrForbidden)` / `ErrUnauthorized`; `APIError.Unwrap()` already exposes these) and enriches the error with the schema/operation and the scope that normally applies, phrased as a starting point rather than a verdict, e.g.:

```text
check metrics pipeline: list pipelines for builtin:openpipeline.metrics.pipelines: list settings objects for schema "builtin:openpipeline.metrics.pipelines": API error (403): Access denied (schema: builtin:openpipeline.metrics.pipelines, normally requires the "settings:objects:read" scope; if the token already has it, check whether the token's policy restricts that scope to a schema list that excludes this one)
```

This still follows the precedent already in the codebase for the same class of problem: `pkg/installer/otel/collector.go`'s DQL verification names `storage:logs:read` on a 401/403, and `pkg/installer/aws_lambda.go` names a specific `fleet-management:*` scope on each of its three 403 call sites. Neither of those call sites is known to have schema-scoped policy restrictions, though, so they can state the scope name as the fix; this change cannot make that same claim, so its message is phrased accordingly (a starting point, not a verdict).

Errors that are not 401/403 (e.g. the 400 constraint violation in Decision 2/4, or a 5xx) pass through unchanged; the scope hint would be misleading for those.

### Decision 7: Preview the extension activation status too, ordered before the route plan

The install preview shows two related but distinct install steps that run after confirmation, in this order: extension activation (`activateHostMonitoringExtensionFn`, existing behavior from the `otel-extension-activation` capability), then the route plan from this change. Originally only the route plan appeared in the preview; the extension step's outcome was only visible after confirmation, when it ran. Since a route is meaningless without the pipeline the extension provisions, and the extension activation runs first, its preview line is shown first too, so the preview mirrors execution order end to end.

`buildExtensionActivationPreview(envURL, platformToken)` determines the status via `installer.ExtensionClient.GetStatus(extensionName)`, a single read-only call added to the shared `ExtensionClient` (`pkg/installer/extension_client.go`) alongside `EnsureInstalled`/`ActivateExtension`/`IsExtensionActive`, rather than as otel-specific logic. `GetStatus` makes one `Extension.Get` call and returns `ExtensionNotInstalled`, `ExtensionInstalledInactive`, or `ExtensionInstalledActive`: a single round trip, where combining the existing `LatestExtensionVersion` (installed or not) and `IsExtensionActive` (active or not) helpers would have made two, since both already call the same underlying endpoint independently. It makes no install or activation call itself; the actual `EnsureInstalled` + `ActivateExtension` sequence still runs only after confirmation, unchanged. Living on `ExtensionClient` also makes the same three-state check available to the Azure/GCP/AWS installers, which already depend on this client for their own install/activate flow, should a similar preview be wanted there later.

**Gate:** identical to the route plan's gate (Decision 5): `featureflags.IsEnabled(featureflags.Experimental)` and a non-empty `platformToken`. This also keeps the preview free of a real API call when no platform token is available to authenticate it (relevant for tests and any future path that lacks one at preview time).

**Preview-check failures are warnings, not blockers:** if `GetStatus` fails (auth or API error), a warning is printed for the extension section only; the rest of the preview (route plan, config, confirmation prompt) proceeds exactly as before. This matches the existing failure handling for the route plan (Decision 5) and for the real activation step itself, which already treats every failure as advisory.

**Dry-run still runs the check:** like the route plan, `--dry-run` builds and prints this preview because the preview phase (including this check) runs before the dry-run early return. No install or activation call is made either way, so this is consistent with "dry-run performs read-only checks but writes nothing."

**The version-list `active` flag is not a reliable activation signal for this extension, so `ExtensionInstalledActive` and `ExtensionInstalledInactive` are shown identically.** Live testing against a real tenant found the extension's `builtin:openpipeline.{metrics,logs,spans}.pipelines` objects fully provisioned (real `smartscapeNodeExtraction` rules, matching Decision 2's discovery), while both signals for "is it active" agreed it wasn't: `GET /extensions/{name}` returned the item with no `active` field (false), and `GET /extensions/{name}/{version}/environmentConfiguration` (the resource `ActivateExtension`'s `POST .../environment-configuration` writes to) returned 404. This means the extension's pipelines provision on Hub install, not on environment-configuration activation, so a tenant can read as inactive while host monitoring already works end to end. Since the preview can't tell a genuinely-inactive tenant apart from this case, `printExtensionActivationPreview` collapses `ExtensionInstalledActive` and `ExtensionInstalledInactive` into a single "already installed" message instead of claiming a specific activation outcome it cannot confirm. `ExtensionNotInstalled` is unambiguous and keeps its distinct "will be installed and activated" message. The real `activateHostMonitoringExtension` step (which still runs unconditionally after confirmation, unchanged) is unaffected: `ActivateExtension` is idempotent (a 409 on an already-active version is treated as success), so calling it when the extension turns out to already be active is a harmless no-op.

### Decision 8: Rebuild the route plan right before applying, instead of reusing the preview-time snapshot

**The bug:** the route plan (`grailPlans`) was built once during the install preview, before the user confirms and before `activateHostMonitoringExtensionFn` runs. On a tenant where the extension had never been installed or activated, every signal's pipeline is absent at preview time, so `buildGrailPlans` marks every signal `grailActionSkip`. The apply step (Decision 5) then reused that same `grailPlans` slice after confirmation, even though extension activation had just run in between and may have made the pipelines exist. The result: on the very first `install otel --experimental` run for a tenant, the routes could never be created in that run, no matter what happened during activation. The user would always need a second run, even immediately after a successful first one, which defeats the "if we detect it, we enable monitoring for it" zero-config goal this change exists for.

**The fix:** right before the apply step, `InstallOtelCollectorWithProject` reuses `grailC` (the `grailRouteClient` already held from the preview phase, so no new client construction is needed) to do three things in order:

1. Unconditionally call `waitForGrailPipelines` (`pkg/installer/otel/grail_routes.go`).
2. Call `buildGrailPlans(ctx, grailC)` again to get a fresh plan.
3. Apply that fresh plan instead of the stale `grailPlans` from before confirmation.

If step 2 itself fails (API error), the preview snapshot is applied as a fallback rather than aborting the apply step entirely, consistent with this change's existing best-effort posture (Decision 5's apply-failure handling).

There is deliberately no branching on whether the extension was freshly installed this run versus already present: `waitForGrailPipelines` is a bounded poll (`installer.ExtensionActiveMaxAttempts` × `installer.ExtensionActiveRetryDelay`, the same constants Azure/GCP/AWS use for their own post-install activation wait) for any one of the three signals' pipelines to become listable via `checkPipeline`. When the pipeline is already there (the common case, since pipelines provision on extension install, not on activation, per the finding above), the very first check inside the poll succeeds and the wait returns immediately, at the cost of one extra `checkPipeline` call. When the extension was hub-installed fresh by this same run (`EnsureInstalled` calling `InstallExtension`, which is asynchronous, `202 Accepted`; see `ExtensionClient.IsExtensionActive`'s doc comment), that same unconditional wait is what gives the pipeline time to appear before the rebuild. Any single signal appearing is treated as readiness, since the extension provisions all three together. Tracking "was this freshly installed" through `activateHostMonitoringExtension` and branching on it was considered and rejected: it would have made the caller responsible for a distinction that doesn't change what needs to happen (wait, then rebuild) and doesn't meaningfully change the cost of the common case (one already-satisfied `checkPipeline` call either way, since the rebuild itself was going to call `checkPipeline` again regardless).

A wait timeout is advisory: it's logged under `--debug` and the rebuild proceeds anyway, so a signal that still isn't ready falls through to the skip path below rather than blocking the install.

**What a remaining skip means, before vs. after this fix:** before Decision 8, the plan was checked before the extension was even installed, so on a completely fresh tenant every signal was skipped on the first run every single time, whether or not the activation that followed actually succeeded. The skip didn't reflect anything real; a second run was always required, unconditionally.

After this fix, the plan is checked again only after dtwiz has installed the extension, activated it, and waited (bounded) for its pipeline to show up. So if a signal is still skipped at that point, it means one of those steps genuinely didn't finish in time: activation failed, or the pipeline is taking longer to appear than the wait allows. The apply-results skip message was reworded from `"skip — pipeline not found (activate the OpenTelemetry Host Monitoring extension first)"` to `"skip — pipeline not found (re-run install otel once the extension is active)"`, because the old wording told the user to take a manual action dtwiz already attempts automatically whenever this code path is reachable (the whole route step is gated behind the same `--experimental` flag that gates extension activation, so telling the user to "activate first" never described a step they needed to do themselves). The preview's equivalent state is worded differently still (see the Preview bullet above, Decision 5): it says "pending", not "skip", since at preview time nothing has actually failed yet.

### Decision 9: Remove the standalone `ReconcileGrailRoutes` entry point as dead code

Earlier drafts of this change (Decision 1, Decision 5) kept `ReconcileGrailRoutes` (and its internal `reconcileGrailRoutes`) as a standalone function with its own plan/preview/confirm/apply cycle, reasoning that a future `update otel` command might want to call it directly without going through the install flow. In practice, nothing ever called it: the install flow in `otel.go` uses `buildGrailRoutePlans` and `applyGrailPlan` directly instead, and no `update otel` command materialized. The only callers were its own tests.

Both functions and their four tests (`TestReconcileGrailRoutes_DryRun/Decline/AutoConfirm_WritesRoutes/MixedPlan`) were deleted, along with the test-only `suppressOutput`/`suppressOutputPipe` helpers that existed solely to support them. Building for a hypothetical caller that never arrived just added ~130 lines nothing exercised outside of its own tests. If a standalone entry point is genuinely needed later, `buildGrailPlans`, `applyGrailPlan`, and `printGrailPlan` are the same building blocks already used by the install flow, and a thin wrapper equivalent to the deleted `reconcileGrailRoutes` can be reconstructed from them in a few lines when there's an actual caller to justify it.

## Risks / Trade-offs

- **Schema shape (resolved):** the routing schema IDs and value layout are confirmed for all three signals against two live tenants (one with routing objects present, one with them absent). Residual risk is that other tenants/versions differ; the reconciliation validates against the schema and parses constraint violations so a shape mismatch fails loudly during development rather than silently mis-writing.
- **Routing singleton does not exist by default:** per the Dynatrace extensions team, the `builtin:openpipeline.<signal>.routing` object is never auto-provisioned, by the extension or otherwise; it only exists once a route has been created for that signal. This is the default state on every tenant until someone creates the first route (manually or via this reconciliation), not a tenant-specific quirk. Mitigation: this is treated as a valid empty state (Decision 4) and the first route write POST-creates the object instead of erroring; confirmed live on a live tenant.
- **Whole-object PUT clobbers sibling routes:** because a signal's routes live in one object, a PUT that omits existing `routingEntries` would disable unrelated user routes. The live spans object already contains an unrelated user route next to ours. Mitigation: append-only construction that carries every prior entry, plus a unit test asserting sibling entries survive (task 6.1); optimistic `If-Match` guards against concurrent edits.
- **Extension-ownership via `externalId` prefix:** the implementation relies on the extension's pipeline objects carrying an `externalId` starting with `com.dynatrace.extension.opentelemetry_`, a platform-assigned field that does not depend on what the extension writes inside `value` (see Decision 2 for the three rejected alternatives). If a future extension rename or re-registration changes this prefix, `checkPipeline` finds no owned object and the route is skipped safely.
- **Eventual consistency after extension activation:** `waitForGrailPipelines` (Decision 8) unconditionally polls, bounded (`installer.ExtensionActiveMaxAttempts` × `installer.ExtensionActiveRetryDelay`, `pkg/installer/retry.go`), for a pipeline to become listable before the route plan is rebuilt and applied. This covers a hub-installed-from-scratch extension (Hub installs are asynchronous) without needing to know whether this run's own activation attempt actually installed anything: an already-provisioned pipeline satisfies the very first check at negligible cost. Residual risk is narrower than the original design: only a delay longer than that bound (plus the collector install and auto-instrumentation steps that also run first) results in a skip, which is shown with a message pointing at a re-run rather than telling the user to activate the extension by hand.
- **Additive-only leaves drift uncorrected:** if a user later narrows their route condition so host data no longer matches, dtwiz will not fix it (it sees a route to the pipeline and treats it as satisfied). This is an accepted trade-off of the safety model (Decision 3); drift correction is explicitly a non-goal.
- **Rollback:** none required. The routes are inert without the corresponding pipeline and are left in place on `uninstall otel` (removal is out of scope). Re-running the install never duplicates them.
- **Insufficient permissions surface as an unclear 403 (partially mitigated):** the Settings API returns a bare `{"error":{"code":403,"message":"Access denied"}}` whether the token is missing the scope entirely or holds the scope but is denied by a schema-restricted IAM policy. dtwiz cannot tell these two cases apart from the response, so it cannot name a single definitive cause. Mitigation: `withGrailScopeHint` (Decision 6) enriches the message with the schema/operation and the scope that normally covers it, framed as a starting point for the user to check rather than a certain diagnosis.
