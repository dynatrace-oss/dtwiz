# OTel Host Monitoring: Smartscape on Grail Routes Design

## Context

The `add-otel-host-monitoring` change configures the managed OTel Collector to emit host metrics, logs, and spans formatted for the Dynatrace **OpenTelemetry Host Monitoring** extension. When that extension is active in the environment, it provisions a processing pipeline named "OpenTelemetry Host Monitoring" for each signal type. Example pipeline objects observed on a live tenant (`builtin:openpipeline.{metrics,spans,logs}.pipelines`) carry the extension-owned `value.customId` entries `extension.opentelemetry-metrics`, `extension.opentelemetry-spans`, and `extension.opentelemetry-logs`, each with the display name "OpenTelemetry Host Monitoring" and a `value.metadataList` entry `{"entryKey": "extension_name", "entryValue": "com.dynatrace.extension.opentelemetry"}` identifying the owning extension. That `customId` is a fixed, extension-defined string, not a per-tenant identifier; it never appears as the Settings-object `externalId`, which the extension leaves empty.

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

dtwiz already writes Settings 2.0 objects elsewhere through `installer.ExtensionClient` ([pkg/installer/extension_client.go](../../../pkg/installer/extension_client.go)) and the vendored dtctl `settings.Handler`; the Azure and GCP installers ([pkg/installer/gcp/dtapi.go](../../../pkg/installer/gcp/dtapi.go)) are the canonical create/get/update-with-`If-Match` examples. This change reuses that machinery to reconcile the three dynamic routes.

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

Why over a standalone command: the routes are meaningless without the host-monitoring collector and extension, so binding them to that flow keeps a single zero-config action. A separate command would add surface area and a second auth/preview path for a step that has no independent use. If a standalone `update` entry point is wanted later, the reconciliation function is written to be callable on its own so it can be lifted without rework.

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
    "externalId": "",
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

Note `externalId` is empty even though the object is clearly extension-owned, confirming in a real payload why filtering by `externalIds` (rejected approach 1, below) can never work. For each returned object, dtwiz inspects `value.metadataList` for an entry `{"entryKey": "extension_name", "entryValue": "com.dynatrace.extension.opentelemetry"}` (constant `otelExtensionName`). The first match is the target pipeline; its Settings **objectId** (`obj.ObjectID`), not its `customId` and not the empty Settings-object `externalId`, is what gets threaded into the routing entry (Decision 4), because the routing schema declares `pipelineId` as a Settings reference (`"type": "setting"`) that only accepts an objectId.

This was arrived at after two rejected approaches, both worth recording so they aren't retried:

1. **Filtering the Settings list by `externalIds=extension.opentelemetry-<signal>`.** This assumed the extension's `value.customId` string was also the Settings-object `externalId`. It isn't: the extension leaves `externalId` empty, so the filter always returned an empty list (`{"items":[],"totalCount":0,"pageSize":100}`) and the pipeline was reported as absent even when the extension was active and the pipeline existed.
2. **Matching on `value.customId == "extension.opentelemetry-<signal>"` after listing unfiltered.** This correctly finds the pipeline (the customId is genuinely present), but then using that same customId string as the routing entry's `pipelineId` fails at write time. The exact request and response, captured live (metrics signal):

   ```text
   POST /platform/classic/environment-api/v2/settings/objects
   [{"schemaId":"builtin:openpipeline.metrics.routing","scope":"environment","value":{"routingEntries":[{"enabled":true,"pipelineType":"custom","pipelineId":"extension.opentelemetry-metrics","matcher":"...","description":"OpenTelemetry Host Monitoring"}]}}]

   → 400 Bad Request
   [{"code":400,"error":{"code":400,"message":"Constraints violated.","constraintViolations":[{"path":"builtin:openpipeline.metrics.routing/0/routingEntries/0/pipelineId","message":"Must be of type setting","parameterLocation":"PAYLOAD_BODY","location":null}]},"invalidValue":{"routingEntries":[{"enabled":true,"pipelineType":"custom","pipelineId":"extension.opentelemetry-metrics","matcher":"...","description":"OpenTelemetry Host Monitoring"}]}}]
   ```

   because the field expects a Settings objectId, not a customId string.

The ownership-based lookup (metadataList `extension_name`) avoids re-introducing either mistake: it does not require the customId to equal any particular thing, and it naturally yields the objectId needed for the write. It also removes the last hardcoded assumption about the extension's naming scheme: if the extension is configured, its pipeline is found by who owns it, not by guessing its name.

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

The write reuses the dtctl `settings.Handler` on `ExtensionClient` (platform token, apps URL): `Settings.ListObjects` for the read, a raw HTTP PUT (mirroring [pkg/installer/gcp/dtapi.go](../../../pkg/installer/gcp/dtapi.go)'s get-then-PUT-with-`If-Match` pattern) for the update, and the SDK's `Settings.Create` for the singleton-absent case. Constraint-violation response bodies are parsed and surfaced on the PUT path (the shared `checkResponse` path discards them), reusing the `constraintViolation` helper pattern already established in the GCP/Azure installers; this is how the `pipelineId`-must-be-a-setting-reference rejection in Decision 2 was diagnosed. The Dynatrace UI reads/writes these objects via the Platform settings API (`/platform/settings/v1/objects`); task 0.2 confirms the dtctl handler's classic path (`/platform/classic/environment-api/v2/settings/objects`) returns and accepts the same objects.

A whole-object PUT replaces `routingEntries`, so the appended array MUST carry every pre-existing entry unchanged. Dropping or reordering entries would silently disable users' existing routes.

Why over the thin `pkg/client` typed client: every existing settings-object write in the repo goes through `ExtensionClient` / dtctl, which already handles the apps-URL mapping, auth scheme, pagination, and retries. Reusing it keeps this consistent with Azure/GCP and avoids reimplementing Settings 2.0 plumbing.

### Decision 5: Route plan shown in the install preview; no separate confirmation prompt

The route plan is built during the install preview phase (before the existing "Proceed with installation?" prompt) and shown as an unnumbered section below the auto-instrumentation plan. After the user confirms the single install prompt, the collector is installed and the routes are applied. No second confirmation prompt is shown for routes.

- **Gate:** the entire step is behind `featureflags.IsEnabled(featureflags.Experimental)` and requires a non-empty `platformToken`. When off, nothing about `install otel` changes and no OpenPipeline API call is made.
- **Preview:** `buildGrailRoutePlans(envURL, platformToken)` is called during the preview phase; `printGrailPlan` prints one line per signal (create / re-enable / already configured / skip). Routes that are skipped or already configured are still shown so the user can see the full picture.
- **Plan-build failures:** if `buildGrailRoutePlans` fails (API error, auth failure), a warning is printed and the routes section is omitted from the preview. The install continues normally.
- **Apply failures:** if `applyGrailPlan` fails for a signal after confirmation, a warning is printed for that signal. The failure does not affect the install result. The user can re-run `install otel` to retry.
- **Dry-run:** `--dry-run` builds and prints the plan but returns before the confirmation prompt, so no routes are applied.
- **Auto-confirm:** `--yes`/`-y` sets `installer.AutoConfirm` on the single install confirmation prompt; this covers the route apply step as well.
- **Standalone use:** `ReconcileGrailRoutes(envURL, platformToken, dryRun bool)` remains as a standalone function with its own plan/preview/confirm/apply cycle for potential future use from `update otel`.

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

**Important limitation dtwiz cannot resolve from the response alone:** Dynatrace IAM policies can grant `settings:objects:read`/`write` restricted to a specific list of schema IDs. A token can genuinely hold the scope and still be denied here if its policy's schema list doesn't include `builtin:openpipeline.<signal>.pipelines`/`.routing`. The 403 body is identical in both cases (missing scope entirely, vs. scope present but schema-restricted), so dtwiz **cannot** distinguish "you don't have this scope" from "you have this scope but not for this schema"; asserting either as the cause would sometimes be a wrong diagnosis. The right move is to hand the user everything needed to check both possibilities, not to guess which one it is.

dtwiz detects a 401/403 via the dtctl SDK's sentinel errors (`errors.Is(err, httpclient.ErrForbidden)` / `ErrUnauthorized`; `APIError.Unwrap()` already exposes these) and enriches the error with the schema/operation and the scope that normally applies, phrased as a starting point rather than a verdict, e.g.:

```text
check metrics pipeline: list pipelines for builtin:openpipeline.metrics.pipelines: list settings objects for schema "builtin:openpipeline.metrics.pipelines": API error (403): Access denied (schema: builtin:openpipeline.metrics.pipelines, normally requires the "settings:objects:read" scope; if the token already has it, check whether the token's policy restricts that scope to a schema list that excludes this one)
```

This still follows the precedent already in the codebase for the same class of problem: `pkg/installer/otel/collector.go`'s DQL verification names `storage:logs:read` on a 401/403, and `pkg/installer/aws_lambda.go` names a specific `fleet-management:*` scope on each of its three 403 call sites. Neither of those call sites is known to have schema-scoped policy restrictions, though, so they can state the scope name as the fix; this change cannot make that same claim, so its message is phrased accordingly (a starting point, not a verdict).

Errors that are not 401/403 (e.g. the 400 constraint violation in Decision 2/4, or a 5xx) pass through unchanged; the scope hint would be misleading for those.

## Risks / Trade-offs

- **Schema shape (resolved):** the routing schema IDs and value layout are confirmed for all three signals against a live tenant (task 0.1) and, separately, against a live tenant (routing objects absent; pipelines present). Residual risk is that other tenants/versions differ; the reconciliation validates against the schema and parses constraint violations so a shape mismatch fails loudly during development rather than silently mis-writing.
- **Routing singleton does not exist by default:** per the Dynatrace extensions team, the `builtin:openpipeline.<signal>.routing` object is never auto-provisioned, by the extension or otherwise; it only exists once a route has been created for that signal. This is the default state on every tenant until someone creates the first route (manually or via this reconciliation), not a tenant-specific quirk. Mitigation: this is treated as a valid empty state (Decision 4) and the first route write POST-creates the object instead of erroring; confirmed live on a live tenant.
- **Whole-object PUT clobbers sibling routes:** because a signal's routes live in one object, a PUT that omits existing `routingEntries` would disable unrelated user routes. The live spans object already contains an unrelated user route next to ours. Mitigation: append-only construction that carries every prior entry, plus a unit test asserting sibling entries survive (task 6.1); optimistic `If-Match` guards against concurrent edits.
- **Extension-ownership metadata stability:** the implementation relies on the extension continuing to tag its pipeline objects with a `value.metadataList` entry `{"entryKey": "extension_name", "entryValue": "com.dynatrace.extension.opentelemetry"}` (Decision 2). If a future extension version drops or renames this metadata, `checkPipeline` finds no owned object and the route is skipped (safe) rather than misrouted. The coupling is documented in a comment with the doc source URL. Earlier drafts of this change assumed the pipeline could be found or referenced by a constructed identifier (`extension.opentelemetry-<signal>`, tried as both a Settings `externalId` filter and, once found, directly as the routing `pipelineId`); both failed against a live tenant (empty list; then a `400 Must be of type setting` on write) and were replaced by the ownership-based lookup.
- **Eventual consistency after extension activation:** if the extension was just activated, its pipelines may not be immediately listable when the route step runs. Mitigation: the step is best-effort and skips on absence; the user can re-run `install otel` to reconcile once the pipeline appears. A bounded retry may be added if this proves common, consistent with `pkg/installer/retry.go`.
- **Additive-only leaves drift uncorrected:** if a user later narrows their route condition so host data no longer matches, dtwiz will not fix it (it sees a route to the pipeline and treats it as satisfied). This is an accepted trade-off of the safety model (Decision 3); drift correction is explicitly a non-goal.
- **Rollback:** none required. The routes are inert without the corresponding pipeline and are left in place on `uninstall otel` (removal is out of scope). Re-running the install never duplicates them.
- **Insufficient permissions surface as an opaque 403 (partially mitigated):** the Settings API returns a bare `{"error":{"code":403,"message":"Access denied"}}` whether the token is missing the scope entirely or holds the scope but is denied by a schema-restricted IAM policy (confirmed live on a live tenant with an under-permissioned token). dtwiz cannot tell these two cases apart from the response, so it cannot name a single definitive cause. Mitigation: `withGrailScopeHint` (Decision 6) detects 401/403 via the SDK's sentinel errors and enriches the message with the schema/operation and the scope that normally covers it, framed as a starting point for the user to check rather than a certain diagnosis.
