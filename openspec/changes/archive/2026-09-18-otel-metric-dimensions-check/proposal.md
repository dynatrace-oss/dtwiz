# Proposal

## Why

OTLP metric resource attributes carry cloud identity fields such as `azure.resource.id`, `aws.arn`,
and `gcp.resource.name`. These are the inputs that Dynatrace's smartscapeNode processors
(`AZURE_VM_id_computation`, `AWS_EC2_INSTANCE_id_computation`, `GCP_INSTANCE_id_computation`)
use to create cloud entity correlation edges.

The tenant-level setting `builtin:opentelemetry-metrics` has a boolean field
`enableMintV2Ingest`. When this is `false`, OTLP metric resource attributes are stripped before
OpenPipeline processes them — so the correlation processors never see the cloud identity fields
and no entity edges are created. The UI name for this setting is **Advanced OTLP metric
dimensions**.

This setting must be `true` for cloud entity correlation to work. Without it, the extension,
OpenPipeline routes, and OTel Collector config are all in place but cloud correlation silently
produces no results.

## Tenant phases

| Phase | Behavior |
|-------|----------|
| **Phase 2, disabled** | Setting exists, `enableMintV2Ingest = false`. dtwiz prompts to enable it after the main install confirmation. |
| **Phase 2, enabled**  | Setting exists, `enableMintV2Ingest = true`. dtwiz silently skips. |
| **Phase 3**           | Setting schema removed (GET returns 404). dtwiz silently skips. |

## What Changes

- `dtwiz install otel` and `dtwiz update otel` check `builtin:opentelemetry-metrics` during
  the tenant prerequisite phase (alongside the extension and Grail routes).
- If the setting is disabled (phase 2 off), dtwiz prompts the user **after** the main install
  confirmation with a separate `[Y/n]` prompt (default Y). On Y: enables it via merge PUT,
  preserving all other fields verbatim. On N: skips silently.
- If the schema returns 404 (phase 3 tenant), dtwiz produces no output about the setting.
- If the setting is already enabled (phase 2 on), dtwiz produces no output.
- Real API errors (auth, 5xx) are logged at debug level; no warning is shown to the user.
- `buildTenantPrerequisitePreview` is refactored to return a `tenantPrereqs` struct instead of
  bare multiple return values, giving the prerequisite group room to grow without signature churn.
- The setting is not touched during `dtwiz uninstall otel`.

## Capabilities

### New Capabilities

- None — this is a prerequisite fix, not a new user-visible feature.

### Modified Capabilities

- `install otel`: post-confirm prompt to enable Advanced OTLP metric dimensions (phase 2 off only).
- `update otel`: same prompt added alongside the existing Grail routes apply.

## Impact

- Adds `pkg/installer/otel/otel_metric_dimensions.go` and its test file.
- Modifies `pkg/installer/otel/otel.go`: introduces `tenantPrereqs` struct, updates
  `buildTenantPrerequisitePreview` signature, adds `otlpDimensionsPlan` wiring.
- Modifies `pkg/installer/otel/collector.go` and `pkg/installer/otel/update_dynatrace.go`:
  update callers to use `tenantPrereqs` struct and call `applyOTLPMetricDimensionsPlan`.
- No new CLI flags, feature gates, or breaking changes.
- Requires `settings:objects:read` and `settings:objects:write` token scopes (already needed for
  Grail routes).
