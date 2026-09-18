# Proposal

## Why

OTLP metric resource attributes carry cloud identity fields such as `azure.resource.id`, `aws.arn`,
and `gcp.resource.name`. These are the inputs that Dynatrace's smartscapeNode processors
(`AZURE_VM_id_computation`, `AWS_EC2_INSTANCE_id_computation`, `GCP_INSTANCE_id_computation`)
use to create cloud entity correlation edges.

The tenant-level setting `builtin:opentelemetry-metrics` has a boolean field
`enableMintV2Ingest`. When this is `false`, OTLP metric resource attributes are stripped before
OpenPipeline processes them — so the correlation processors never see the cloud identity fields
and no entity edges are created.

This setting must be `true` for cloud entity correlation to work. Without it, the extension,
OpenPipeline routes, and OTel Collector config are all in place but cloud correlation silently
produces no results.

## What Changes

- `dtwiz install otel` and `dtwiz update otel` check `builtin:opentelemetry-metrics` during the
  tenant prerequisite preview phase (alongside the extension and Grail routes checks).
- If the setting is disabled, dtwiz enables it post-confirmation (non-destructive write: only
  `enableMintV2Ingest` is patched; all other fields preserved verbatim).
- If the Settings API is unreachable or returns an error, dtwiz prints a warning and continues
  — the setting is non-fatal, matching the existing extension and Grail routes error policy.
- `buildTenantPrerequisitePreview` is refactored to return a `tenantPrereqs` struct instead of
  bare multiple return values, giving the prerequisite group room to grow without signature churn.

## Capabilities

### New Capabilities

- None — this is a prerequisite fix, not a new user-visible feature.

### Modified Capabilities

- `install otel`: tenant prerequisite preview gains a metric dimensions check and apply step.
- `update otel`: same check added alongside the existing Grail routes apply.

## Impact

- Adds `pkg/installer/otel/otel_metric_dimensions.go` and its test file.
- Modifies `pkg/installer/otel/otel.go`: introduces `tenantPrereqs` struct, updates
  `buildTenantPrerequisitePreview` signature, adds `dimPlan` wiring.
- Modifies `pkg/installer/otel/collector.go` and `pkg/installer/otel/update_dynatrace.go`:
  update callers to use `tenantPrereqs` struct and call `applyOtelMetricDimensionsPlan`.
- No new CLI flags, feature gates, or breaking changes.
- Requires `settings:objects:read` and `settings:objects:write` token scopes (already needed for
  Grail routes).
