# Design

## Context

`dtwiz install otel` and `dtwiz update otel` have a tenant prerequisite phase managed by
`buildTenantPrerequisitePreview` in `pkg/installer/otel/otel.go`. It currently checks two things:
the OTel Host Monitoring extension (via `buildExtensionActivationPreviewFn`) and OpenPipeline
dynamic routes (via `buildGrailRoutePlansFn`). Both are shown in a preview before confirmation;
both are applied after the user confirms.

The function returns `(grailRouteClient, []grailSignalPlan)`. Adding a third prereq would produce
a third bare return value — a pattern that does not scale. The design introduces a `tenantPrereqs`
struct to carry all prereq state as a named group.

## Goals / Non-Goals

**Goals:**

- Check `builtin:opentelemetry-metrics` → `enableMintV2Ingest` during the prereq preview for
  every `install otel` and `update otel`, regardless of cloud provider.
- If disabled, enable it post-confirmation. All other fields preserved verbatim (merge PUT).
- Non-fatal: GET or PUT failure prints a warning and does not abort the install.
- Replace `buildTenantPrerequisitePreview`'s bare return tuple with a `tenantPrereqs` struct.

**Non-Goals:**

- Cloud-provider gating — the setting applies to all OTLP metric ingestion.
- New CLI flags or feature gates.
- Changing the extension or Grail routes logic.

## Decisions

### `tenantPrereqs` struct instead of bare tuple

`buildTenantPrerequisitePreview` grows to three prereqs with this change. A struct makes callers
readable (`prereqs.grailC`, `prereqs.dimPlan`) and avoids positional confusion as the set expands.

Alternative considered: add a third return value `*otelMetricDimensionsPlan`. Rejected — three
unnamed returns is already hard to follow; a struct is strictly cleaner.

### Interface-based client for testability

`otelMetricDimensionsClient` abstracts the two Settings API calls (GET and PUT). The real
implementation uses the same `dtctl` SDK HTTP client as `grailRouteClient` (via
`installer.NewExtensionClient`). Tests inject an `httptest.Server`-backed stub — the same pattern
used in `grail_routes_test.go`.

### Merge PUT strategy

The `builtin:opentelemetry-metrics` schema has multiple user-configurable fields:
`additionalAttributes` (array), `toDropAttributes` (array), `meterNameToDimensionEnabled`,
`additionalAttributesToDimensionEnabled`, and `enableMintV2Ingest`. The Settings API rejects a
PUT that omits required fields with a 400 validation error — confirmed by live API test with a
partial payload on rx28105.

The GET response value is decoded into `map[string]any`, `enableMintV2Ingest` is set to `true`,
and the full map is PUT back. This preserves all other fields verbatim and satisfies the schema
validator. Verified on rx28105: `additionalAttributes` (34 entries) and `toDropAttributes`
survived intact.

### Non-fatal on API error

Consistent with extension and Grail routes: `display.PrintWarning(...)` and continue. The
install is not blocked because the metric dimensions setting is a correlation enhancement, not a
data delivery prerequisite.

## Key Paths

| Path | Role |
|---|---|
| `pkg/installer/otel/otel_metric_dimensions.go` | New: client interface, plan struct, build/apply/print functions |
| `pkg/installer/otel/otel_metric_dimensions_test.go` | New: table-driven tests with httptest.Server |
| `pkg/installer/otel/otel.go` | Modified: `tenantPrereqs` struct, updated `buildTenantPrerequisitePreview` |
| `pkg/installer/otel/collector.go` | Modified: use `tenantPrereqs`, call `applyOtelMetricDimensionsPlan` |
| `pkg/installer/otel/update_dynatrace.go` | Modified: same caller update |

## API Shape

### Settings GET

```text
GET /platform/classic/environment-api/v2/settings/objects
    ?schemaIds=builtin:opentelemetry-metrics&scopes=environment
```

Response: one object. Fields consumed:

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

### Settings PUT (when disabled)

```text
PUT /platform/classic/environment-api/v2/settings/objects/<objectId>
```

Body: full `value` map from the GET response with `enableMintV2Ingest` set to `true`. All other
fields passed verbatim. `If-Match` set to `schemaVersion` from the GET response.

## Display Format

Preview (before confirmation):

```text
  OpenTelemetry metric dimensions
  ─────────────────────────────────────────────────────
  MintV2 ingest:  needs enabling
```

or

```text
  OpenTelemetry metric dimensions
  ─────────────────────────────────────────────────────
  MintV2 ingest:  already enabled
```

## Risks / Trade-offs

- Token missing `settings:objects:write` scope: PUT fails, warning printed, install continues.
  User must enable the setting manually.
- Schema adds new required fields in a future upgrade: the merge PUT round-trips only the fields
  present at GET time; new required fields would be missing and cause a 400. Low probability given
  the schema's stability across tenant versions.
- Race between GET and PUT (another actor changes the object): low probability; a warning on PUT
  failure is sufficient given the non-fatal policy.
