# Proposal

## Why

`dtwiz install otel` creates an `OTEL_HOST` Smartscape entity with no link to the underlying cloud VM. The `OTEL_HOST runs_on <cloud entity>` edge requires a cloud-specific correlation attribute (`aws.arn`, `azure.resource.id`, or `gcp.resource.name`) on host metrics. Without it, cloud topology context is invisible next to the host.

## What Changes

- Probe IMDS at install time to detect the cloud provider (AWS/Azure/GCP).
- Inject the matching cloud resource detector into `resource_detection/system`.
- Add a `transform/dt-cloud-correlation` processor that derives the Dynatrace cloud-correlation attribute from enriched resource attributes and sets it on every host metric.

## Capabilities

### New Capabilities

- `otel-cloud-entity-correlation`: OTel Collector configs generated on cloud VMs now include the correct cloud detector and OTTL transform, enabling the `OTEL_HOST runs_on <cloud entity>` Smartscape edge without manual configuration.

### Modified Capabilities

- `generateOtelConfig`: calls `detectIMDSCloudProvider()` and populates `CloudProvider` in template data.
- `otel.tmpl`: `resource_detection/system`, `transform/dt-cloud-correlation`, and `metrics/host` pipeline are conditional on `CloudProvider`.

## Impact

- Two new files: `detect_cloud.go`, `detect_cloud_test.go`.
- At most 150 ms added to install time on cloud VMs; non-cloud machines see no delay.
- No new CLI flags, no new Dynatrace API calls.
