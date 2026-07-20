# Proposal

## Why

The GCP integration used a fixed resource name `dtwiz-gcp` for the Dynatrace
connection, monitoring configuration, and Google Cloud service account. This caused
collisions when different Dynatrace environments pointed at the same GCP project: a
second install would create a second service account with the same name, or the
uninstaller from one environment would delete the service account belonging to another.

## What Changes

- Derive the integration name from the Dynatrace environment URL instead of using
  a fixed string. The name becomes `dtwiz-gcp-<tenant-id>`, where `<tenant-id>` is
  the first DNS label of the environment URL (for example `dtwiz-gcp-fds1499d`).
- The GCP service account ID and the Dynatrace connection and monitoring configuration
  name all use the derived name.
- Uninstall discovers resources by prefix so it finds both the new env-scoped name
  and the old fixed name `dtwiz-gcp` from previous installs.
- Uninstall separates service accounts into current (env-scoped name plus any SA
  bound to a discovered connection) and legacy (old fixed name only). Legacy cleanup
  failures are warn-only: they do not block deletion of the current integration.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `gcp-monitor-install`: Resources are created under the env-scoped name instead
  of the fixed `dtwiz-gcp` name.
- `gcp-monitor-update`: Connection and monitoring configuration lookup uses the
  env-scoped name.
- `gcp-monitor-uninstall`: Resource discovery uses prefix matching to cover both
  old and new names. Service account cleanup is split into current (fatal on error)
  and legacy (warn-only).

## Impact

- Affected code: `pkg/installer/gcp/config.go`, `install.go`, `update.go`,
  `uninstall.go`, and `dtapi.go`.
- No API changes. Name change is transparent to Dynatrace and GCP APIs.
- Existing installs under the old `dtwiz-gcp` name continue to work: uninstall
  finds and removes them, and the status check uses prefix matching.
- Rollback: revert the naming function and restore the fixed constant. Old installs
  are unaffected.
