# Tasks

## Implementation

- [x] `pkg/installer/otel/detect_cloud.go` — `detectIMDSCloudProvider()` with parallel IMDS probes; const URLs; buffered channel first-wins; no `sync.Once`
- [x] `pkg/installer/otel/collector.go` — `otelConfigOpt` / `withCloudProvider`; `generateOtelConfig` pure (no detection); detection in `prepareCollectorPlan`; `cloudProvider` stored on `collectorPlan`
- [x] `pkg/installer/otel/otel.tmpl` — conditional `resource_detection/system` detectors, `transform/dt-cloud-correlation` processor, pipeline wiring

## Tests

- [x] `pkg/installer/otel/collector_test.go` — `TestGenerateOtelConfig_Combined_AWS/Azure/GCP` snapshot tests via `withCloudProvider`; assert detector line, OTTL statement, processor order; `assertProcessorOrder` uses `slices.Index`

## Validation

- [x] `make build`, `make lint`, `make test-coverage`, `make markdownlint` — all pass
- [x] End-to-end on EC2: `aws.arn` present on host metrics in `qxd2623d`
- [x] End-to-end on Azure VM: `azure.resource.id` present
- [x] End-to-end on GCP VM: `gcp.resource.name` present
- [ ] `OTEL_HOST runs_on <cloud entity>` Smartscape edge visible after extension update
