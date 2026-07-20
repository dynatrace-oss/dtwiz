## Context

Azure and GCP monitoring configuration creation relies on tenant-local Dynatrace extension metadata and schemas. The existing cloud flows create or reconcile monitoring configurations, but tenants that have not activated the relevant data-acquisition extension can fail when dtwiz looks up extension versions or schemas.

AWS already activates its data-acquisition extension before creating the monitoring configuration. Azure and GCP should follow the same prerequisite pattern.

## Goals / Non-Goals

**Goals:**

- Activate the Azure and GCP data-acquisition extensions before monitoring configuration creation or reconcile.
- Keep install and update flows idempotent when the extension is already installed.
- Use the existing dtctl SDK extension API through dtwiz's shared extension client.
- Preserve current cloud authentication setup and monitoring configuration payload shapes.

**Non-Goals:**

- Change Azure/GCP cloud-side authentication resources, IAM/RBAC permissions, or connection schemas.
- Change extension package versions dynamically beyond the existing version/schema lookup used for monitoring config generation.
- Add a user-facing selection or feature flag for extension activation.

## Decisions

- Add a shared `InstallExtension` helper to `pkg/installer.ExtensionClient`.
  - Rationale: Azure and GCP already wrap this client for Settings and Extensions APIs, so extension activation belongs beside existing extension-version, schema, and monitoring-config helpers.
  - Alternative considered: duplicate activation code in each cloud package. Rejected because it would repeat idempotency/error handling.

- Use `dtctl/sdk/api/extension.Handler.InstallFromHub` rather than raw HTTP.
  - Rationale: dtwiz already depends on the dtctl SDK for extension version/schema and monitoring config operations, and the SDK exposes the Hub install operation.
  - Alternative considered: raw Platform API POST matching AWS. Rejected after confirming the SDK method exists.

- Treat already-installed extension responses as success.
  - Rationale: install/update should be safe for tenants where the extension is already active. This mirrors the AWS installer behavior and keeps reruns idempotent.

- Activate the extension after cloud authentication finalization but before monitoring config creation/reconcile.
  - Rationale: the extension is only needed for the Dynatrace monitoring config step. Placing it late avoids mutating Dynatrace extension state if earlier cloud prerequisite work fails.

## Risks / Trade-offs

- Broad idempotency matching could hide an unrelated API bad-request response. Mitigation: wrap errors with extension name/version, add debug logs, and keep tests covering already-installed and failure paths.
- Extension activation introduces one more Dynatrace-side mutation after cloud resources have been created. Mitigation: existing partial-failure cleanup guidance remains in place and monitoring config creation is skipped when activation fails.
