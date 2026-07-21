# Proposal

## Why

The Dynatrace hub extension install endpoint returns **202 Accepted** — the activation
is asynchronous. On a tenant where the extension has never been installed, dtwiz called
`installExtension()` and immediately attempted to create the monitoring configuration.
The extension was not yet active, so the DT Extensions API returned a 400 validation
error. The install appeared to fail at step 7, even though all cloud-side resources were
correctly set up.

Users who ran `dtwiz uninstall azure` and then `dtwiz install azure` again succeeded
because the extension was already installed (and fully active) from the previous attempt
— `installExtension()` skipped the hub call and returned immediately.

## What Changes

- `installExtension()` on the `dtclient` interface (Azure and GCP) now returns
  `(bool, error)` — `true` when the extension was freshly installed, `false` when it
  was already present.
- A new `isExtensionActive() (bool, error)` method is added to both `dtclient`
  interfaces. It checks the `Active` field on the extension version list.
- `IsExtensionActive(extensionName string) (bool, error)` is added to the shared
  `ExtensionClient` in `pkg/installer/`.
- When a fresh install is detected, both the Azure and GCP install flows poll
  `isExtensionActive()` (every 5 s, up to 60 s) before proceeding to monitoring
  configuration creation. Progress is surfaced to the user:
  - `"Extension freshly installed — waiting for it to become active..."`
  - `"✓ Extension is active"` on success, or a debug log and graceful proceed on timeout.
- The `update` flows ignore the fresh-install signal (`_, err`) — extensions are already
  active by the time an update runs.

## Capabilities

### Modified Capabilities

- `azure-monitor-install`: After a fresh extension hub install, poll for `Active == true`
  before creating the monitoring configuration.
- `gcp-monitor-install`: Same as above for the GCP extension.

## Impact

- Affected code: `pkg/installer/extension_client.go`, `pkg/installer/azure/dtapi.go`,
  `pkg/installer/azure/install.go`, `pkg/installer/gcp/dtapi.go`,
  `pkg/installer/gcp/install.go`, and both packages' test helpers.
- The `dtclient` interface in both packages gains two methods; all fakes are updated.
- Worst-case added latency on a fresh install: 60 s (12 × 5 s). In practice the
  extension becomes active in a few seconds and the wait terminates early.
- No change to behavior when the extension is already installed (the common case for
  updates and re-installs after a partial failure).
- Rollback: revert the interface signature changes and remove the `waitForExtensionActive`
  call; the direct `createMonitoring` call is restored.
