# Design

## Context

The `meta-opentelemetry-host` extension creates `OTEL_HOST runs_on <cloud entity>` in Smartscape when one of these attributes is present on host metrics:

| Cloud | Attribute | Format |
|---|---|---|
| AWS | `aws.arn` | `arn:aws:ec2:<region>:<account>:instance/<id>` |
| Azure | `azure.resource.id` | `/subscriptions/.../providers/microsoft.compute/virtualmachines/<name>` (lowercase) |
| GCP | `gcp.resource.name` | `//compute.googleapis.com/projects/<project>/zones/<zone>/instances/<name>` |

The OTel `resourcedetectionprocessor` enriches resource attributes via cloud-specific detectors (`ec2`, `azure`, `gcp`). An OTTL `transform` processor then derives the correlation attribute from the enriched data.

## Goals / Non-Goals

**Goals:** detect cloud provider at install time, generate a config that sets the correct correlation attribute on all host metrics, zero impact on non-cloud machines.

**Non-Goals:** runtime detection, Kubernetes node correlation, VMSS support.

## IMDS Detection

All three probes run in parallel with a 150 ms timeout. First 200 response wins via `sync.Once`.

| Cloud | Method | Endpoint |
|---|---|---|
| AWS | PUT | `http://169.254.169.254/latest/api/token` + `X-aws-ec2-metadata-token-ttl-seconds: 21600` |
| Azure | GET | `http://169.254.169.254/metadata/instance?api-version=2021-02-01` + `Metadata: true` |
| GCP | GET | `http://metadata.google.internal/computeMetadata/v1/` + `Metadata-Flavor: Google` |

AWS uses IMDSv2 PUT so that Azure's `169.254.169.254` (which returns 404 for `/latest/api/token`) is never a false positive.

## Template Changes

`resource_detection/system` selects the cloud detector based on `CloudProvider`. GCP requires `gcp.gce.instance.name` enabled explicitly.

`transform/dt-cloud-correlation` is generated only when `CloudProvider` is non-empty. `error_mode: ignore` — enrichment is best-effort; a missing cloud attribute must not block export.

`metrics/host` pipeline: `transform/dt-cloud-correlation` is inserted after `resource_detection/system` and before `transform`.

## Testability

`awsIMDSURL`, `azureIMDSURL`, `gcpIMDSURL` are package-level vars. `TestMain` stubs all three to `http://localhost:0/` before any test runs — prevents live IMDS probes on Azure-hosted CI runners where the Azure endpoint responds.

## Key Decisions

- **IMDSv2 PUT**: plain GET to `169.254.169.254/latest/meta-data/` returns 401 on IMDSv2-only EC2 and 404 on Azure — ambiguous. PUT to `/latest/api/token` returns 200 only on EC2.
- **Install-time detection**: keeps the collector config self-contained; a machine does not change cloud providers post-deployment.
- **`error_mode: ignore`**: partial ARN from missing IAM permissions should not block metric export.
