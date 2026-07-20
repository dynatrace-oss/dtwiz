## Why

Azure and GCP monitoring configuration creation depends on the Dynatrace cloud data-acquisition extension being installed in the tenant. Tenants that have not yet activated the extension can fail during schema/version lookup or monitoring configuration creation even after the cloud-side authentication chain is set up correctly.

## What Changes

- Install or activate the Azure and GCP data-acquisition extension package before monitoring configuration creation during fresh installs.
- Install or activate the same extension package before in-place monitoring configuration reconcile during updates.
- Treat an already-installed extension as success so existing tenants remain idempotent.
- Use the existing dtctl SDK extension installation API from dtwiz's shared extension client.
- Add debug logging and focused tests for successful, already-installed, and failure-before-monitoring-config paths.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `azure-monitor-install`: Fresh Azure install must activate the Azure data-acquisition extension before creating the monitoring configuration.
- `azure-monitor-update`: Azure update/reconcile must activate the Azure data-acquisition extension before creating or updating monitoring configurations.
- `gcp-monitor-install`: Fresh GCP install must activate the GCP data-acquisition extension before creating the monitoring configuration.
- `gcp-monitor-update`: GCP update/reconcile must activate the GCP data-acquisition extension before creating or updating monitoring configurations.

## Impact

- Affected code: shared extension client, Azure installer/update flow, GCP installer/update flow, and focused unit tests.
- Affected APIs: Dynatrace Extensions 2.0 install-from-Hub endpoint via the dtctl SDK, plus the existing monitoring configuration APIs.
- Behavior remains backwards compatible for tenants where the extension is already installed.
- Rollback: remove the pre-monitoring-config extension activation calls and revert the shared helper/tests; existing cloud authentication flows are otherwise unchanged.
