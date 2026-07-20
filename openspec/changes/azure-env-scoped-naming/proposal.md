# Proposal

## Why

The Azure integration used a fixed resource name `dtwiz-azure` for the Dynatrace
connection, monitoring configuration, and Azure App Registration. This caused
collisions when different Dynatrace environments pointed at the same Azure tenant:
a second install would find an App Registration with the same name and fail with
an Azure "Insufficient privileges" error when trying to patch it.

Additionally, when a previous install failed partway through and left an orphaned
App Registration, a retry would hit the same collision even within the same environment.

## What Changes

- Derive the integration name from the Dynatrace environment URL instead of using
  a fixed string. The name becomes `dtwiz-azure-<tenant-id>`, where `<tenant-id>`
  is the first DNS label of the environment URL (for example `dtwiz-azure-fds1499d`).
- Uninstall discovers resources by prefix so it finds both the new env-scoped name
  and the old fixed name `dtwiz-azure` from previous installs.
- Uninstall searches Azure AD under both names to clean up orphaned App Registrations
  from either naming scheme.
- Legacy resource cleanup (resources under the old `dtwiz-azure` name) is warn-only:
  failures do not block deletion of the current integration's resources.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `azure-monitor-install`: Resources are created under the env-scoped name instead
  of the fixed `dtwiz-azure` name.
- `azure-monitor-update`: Connection and monitoring configuration lookup uses the
  env-scoped name. Account lookup runs before resource discovery.
- `azure-monitor-uninstall`: Resource discovery uses prefix matching to cover both
  old and new names. Azure AD search covers both. Legacy cleanup failures are
  non-fatal warnings.

## Impact

- Affected code: `pkg/installer/azure/config.go`, `install.go`, `update.go`,
  `uninstall.go`, `dtapi.go`, and `pkg/installer/extension_client.go`.
- No API changes. Name change is transparent to Dynatrace and Azure APIs.
- Existing installs under the old `dtwiz-azure` name continue to work: uninstall
  finds and removes them, and the status check uses prefix matching.
- Rollback: revert the naming function and restore the fixed constant. Old installs
  are unaffected.
