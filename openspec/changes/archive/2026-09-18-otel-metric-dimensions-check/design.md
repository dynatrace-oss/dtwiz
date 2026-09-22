# Design

## Context

`dtwiz install otel` and `dtwiz update otel` have a tenant prerequisite phase managed by
`buildTenantPrerequisitePreview` in `pkg/installer/otel/otel.go`. It checks the OTel Host
Monitoring extension (via `buildExtensionActivationPreviewFn`) and OpenPipeline dynamic routes
(via `buildGrailRoutePlansFn`). Both are shown in a preview before confirmation; both are applied
after the user confirms.

The function originally returned `(grailRouteClient, []grailSignalPlan)`. Adding a third prereq
produced a `tenantPrereqs` struct to carry all prereq state as a named group.

## Tenant phases

The `builtin:opentelemetry-metrics` schema is not present on all tenant types:

| Phase | API behavior | dtwiz response |
|-------|-------------|----------------|
| Phase 2, `enableMintV2Ingest = false` | GET returns one object, field is false | Prompt user to enable after main confirm |
| Phase 2, `enableMintV2Ingest = true`  | GET returns one object, field is true  | Silent skip — nothing to do |
| Phase 3 | GET returns HTTP 404 | Silent skip — setting removed from tenant |

## Goals / Non-Goals

**Goals:**

- Check `builtin:opentelemetry-metrics` → `enableMintV2Ingest` during `install otel` and
  `update otel`, for phase 2 tenants where it is disabled.
- If disabled (phase 2 OFF), show a status line in the install preview ("will be enabled") alongside
  the extension and Grail routes previews. After the main `ConfirmProceed`, enable via merge PUT
  (all other fields preserved verbatim). Symmetric with extension and Grail routes — no secondary prompt.
- Phase 3 (404) and phase 2 already-enabled: produce zero output.
- Replace `buildTenantPrerequisitePreview`'s bare return tuple with a `tenantPrereqs` struct.

**Non-Goals:**

- Disabling the setting during `dtwiz uninstall otel`.
- A secondary opt-out prompt after the main confirmation.
- Cloud-provider gating — the setting applies to all OTLP metric ingestion.
- New CLI flags or feature gates.

## Decisions

### `tenantPrereqs` struct instead of bare tuple

`buildTenantPrerequisitePreview` now holds three prereqs. A struct makes callers readable
(`prereqs.grailC`, `prereqs.otlpDimensionsPlan`) and avoids positional confusion.

### Phase 3 detection

`buildOTLPMetricDimensionsPlan` returns `(nil, nil, nil)` on 404 (detected via
`errors.Is(err, httpclient.ErrNotFound)` or `*httpclient.APIError` with status 404). Callers
treat nil plan as "nothing to do" — no output, no prompt.

### Symmetric with extension and Grail routes (preview + main confirm)

Phase 2 OFF is shown in the preview section alongside extension and Grail routes ("will be
enabled"). The main `ConfirmProceed` covers all three prereqs. Execution calls
`applyOTLPMetricDimensions` — self-contained, no return, mirrors `applyGrailRoutes`. No secondary
prompt needed.

### Silent skip for phase 2 already-enabled

No status line printed. The setting being on is the expected state; surfacing it adds noise with
no action value.

### Merge PUT strategy

The `builtin:opentelemetry-metrics` schema has multiple user-configurable fields. The Settings API
rejects a PUT that omits required fields (400 validation error — confirmed on rx28105 with partial
payload). The GET response value is decoded into `map[string]any`, `enableMintV2Ingest` is set to
`true`, and the full map is PUT back. This preserves all other fields verbatim.

### Naming

Internal identifiers use `otlpMetricDimensions*`. UI-facing strings use the product term
"Advanced OTLP metric dimensions" (matching the Dynatrace UI), not the internal `MINTv2` label.

## Key Paths

| Path | Role |
|---|---|
| `pkg/installer/otel/otel_metric_dimensions.go` | Client interface, plan struct, build/apply/print functions |
| `pkg/installer/otel/otel_metric_dimensions_test.go` | Table-driven tests with httptest.Server |
| `pkg/installer/otel/otel.go` | `tenantPrereqs` struct, updated `buildTenantPrerequisitePreview`, `applyOTLPMetricDimensions` call |
| `pkg/installer/otel/collector.go` | `InstallOtelCollectorOnly`: same `applyOTLPMetricDimensions` call |
| `pkg/installer/otel/update_dynatrace.go` | Same `applyOTLPMetricDimensions` call |

## API Shape

### Settings GET

```text
GET /platform/classic/environment-api/v2/settings/objects
    ?schemaIds=builtin:opentelemetry-metrics&scopes=environment
```

Response (phase 2): one object. Fields consumed:

```json
{
  "objectId": "<id>",
  "schemaVersion": "1.6.2",
  "value": {
    "enableMintV2Ingest": false,
    "additionalAttributesToDimensionEnabled": true,
    "additionalAttributes": [...],
    "toDropAttributes": [...],
    "meterNameToDimensionEnabled": true
  }
}
```

Phase 3 returns HTTP 404 — treated as silent skip.

### Settings PUT (when disabled, user confirmed)

```text
PUT /platform/classic/environment-api/v2/settings/objects/<objectId>
```

Body: full `value` map from the GET response with `enableMintV2Ingest` set to `true`. All other
fields passed verbatim. `If-Match` set to `schemaVersion` from the GET response.

## Display Format

Phase 2 OFF — in the preview section alongside extension and Grail routes:

```text
  Advanced OTLP metric dimensions
  ─────────────────────────────────────────────────────
  Advanced OTLP metric dimensions:  will be enabled
                    <docs-url>
```

Phase 2 ON and phase 3: no output anywhere.

After execution (on success):

```text
  ✓ Advanced OTLP metric dimensions enabled
```

## Risks / Trade-offs

- Token missing `settings:objects:write` scope: PUT fails, warning printed, install continues.
  User must enable the setting manually.
- Schema adds new required fields in a future upgrade: the merge PUT round-trips only the fields
  present at GET time; new required fields would be missing and cause a 400. Low probability given
  the schema's stability across tenant versions.
- Race between GET and PUT (another actor changes the object): low probability; a warning on PUT
  failure is sufficient given the non-fatal policy.
