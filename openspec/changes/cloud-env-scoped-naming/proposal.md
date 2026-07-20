# Proposal

## Why

Both the GCP (`dtwiz-gcp`) and Azure (`dtwiz-azure`) integrations used a hardcoded
name for all created resources. This caused two classes of problems:

1. **Multi-environment collision**: Two Dynatrace environments pointing at the same
   cloud account each try to create a resource with the same name. The second install
   fails or silently damages the first environment's integration.

2. **Partial-install retry collision**: A failed install leaves an orphaned resource.
   A retry hits the same conflict even though the user has full ownership.

## What Changes

- The integration name is derived from the Dynatrace environment URL instead of a
  fixed string: `dtwiz-gcp-<tenant-id>` and `dtwiz-azure-<tenant-id>`, where
  `<tenant-id>` is the first DNS label of the environment URL.
- Uninstall discovers resources by prefix so it finds both the new env-scoped name
  and the old fixed name from previous installs.
- Legacy resource cleanup (resources under the old fixed name) is warn-only; failures
  do not block removal of the current integration.

## Capabilities

### Modified Capabilities

- `gcp-monitor-install`, `gcp-monitor-update`, `gcp-monitor-uninstall`
- `azure-monitor-install`, `azure-monitor-update`, `azure-monitor-uninstall`

## Impact

- Affected code: `pkg/installer/gcp/` and `pkg/installer/azure/` (config, install,
  update, uninstall, dtapi).
- No API changes. Name changes are transparent to Dynatrace and cloud provider APIs.
- Existing installs under the old fixed names continue to work: uninstall finds and
  removes them, and status checks use prefix matching.
- Rollback: revert the naming functions and restore the fixed constants.
