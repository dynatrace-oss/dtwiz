# Tasks

## Implementation

- [x] `pkg/installer/otel/detect_cloud.go` — `detectIMDSCloudProvider()` with parallel IMDS probes and package-level URL vars
- [x] `pkg/installer/otel/collector.go` — `CloudProvider string` field on `otelConfigData`; call `detectIMDSCloudProvider()` in `generateOtelConfig()`
- [x] `pkg/installer/otel/otel.tmpl` — conditional `resource_detection/system` detectors, `transform/dt-cloud-correlation` processor, pipeline wiring

## Tests

- [x] `pkg/installer/otel/detect_cloud_test.go`
  - `TestMain` stubs all IMDS URLs to prevent live probes on cloud CI runners
  - `TestDetectIMDSCloudProvider` — table-driven: aws (PUT 200), azure (GET 200), gcp (GET 200), no-imds (404 → "")

## Validation

- [x] `make build`, `make lint`, `make test-coverage`, `make markdownlint` — all pass
- [x] End-to-end on EC2: `aws.arn` present on host metrics in `qxd2623d`
- [x] End-to-end on Azure VM: `azure.resource.id` present
- [x] End-to-end on GCP VM: `gcp.resource.name` present
- [ ] `OTEL_HOST runs_on <cloud entity>` Smartscape edge visible after extension update
