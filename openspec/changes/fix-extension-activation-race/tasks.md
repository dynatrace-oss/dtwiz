# Implementation Tasks

## Shared

- [x] 1.1 Add `IsExtensionActive(extensionName string) (bool, error)` to `pkg/installer/extension_client.go`. Calls `Extension.Get()` and returns `true` if any item has `Active == true`.

## Azure

- [x] 2.1 Change `dtclient.installExtension()` signature in `pkg/installer/azure/dtapi.go` to `(bool, error)`. Add `isExtensionActive() (bool, error)` to the interface.
- [x] 2.2 Update `sdkDTClient.installExtension()`: return `(false, nil)` when already installed, `(true, nil)` after a fresh `InstallFromHub` call.
- [x] 2.3 Add `sdkDTClient.isExtensionActive()` delegating to `e.IsExtensionActive(extensionName)`.
- [x] 2.4 Add `extensionActiveMaxAttempts` / `extensionActiveRetryDelay` constants and `waitForExtensionActive(dtc, sleeper)` in `pkg/installer/azure/install.go`.
- [x] 2.5 Update `runInstallSteps` in `pkg/installer/azure/install.go`: capture `freshlyInstalled` from `installExtension()`; if true, print wait message, call `waitForExtensionActive`, print `"✓ Extension is active"` on success.
- [x] 2.6 Update `pkg/installer/azure/update.go`: discard the bool (`_, err`).
- [x] 2.7 Update `fakeDTClient` and `noopDTClient` in `pkg/installer/azure/helpers_test.go` for new signatures. `fakeDTClient.installExtension()` returns `(false, err)` (no wait in tests); `isExtensionActive()` returns `(true, nil)`.
- [x] 2.8 `go test ./pkg/installer/azure/...` passes.

## GCP

- [x] 3.1 Change `dtclient.installExtension()` signature in `pkg/installer/gcp/dtapi.go` to `(bool, error)`. Add `isExtensionActive() (bool, error)` to the interface.
- [x] 3.2 Update `sdkDTClient.installExtension()` and add `sdkDTClient.isExtensionActive()` (same pattern as Azure).
- [x] 3.3 Add `extensionActiveMaxAttempts` / `extensionActiveRetryDelay` constants and `waitForExtensionActive(dtc, sleeper)` in `pkg/installer/gcp/install.go`.
- [x] 3.4 Update `runInstallSteps` in `pkg/installer/gcp/install.go` with the same fresh-install wait pattern.
- [x] 3.5 Update `pkg/installer/gcp/update.go`: discard the bool (`_, err`).
- [x] 3.6 Update `fakeDTClient` and `noopDTClient` in `pkg/installer/gcp/helpers_test.go` for new signatures.
- [x] 3.7 `go test ./pkg/installer/gcp/...` passes.
