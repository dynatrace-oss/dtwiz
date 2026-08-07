# Tasks: deactivate-host-monitoring-extension

## 1. ExtensionClient: add DeactivateExtension and DeleteExtensionVersion

- [ ] 1.1 Add `DeactivateExtension(extensionName string) error` to `pkg/installer/extension_client.go` — raw REST `DELETE /platform/extensions/v2/extensions/{name}/environment-configuration`; treat 404 as success
- [ ] 1.2 Add unit tests for `DeactivateExtension` in `pkg/installer/extension_client_test.go`: success (204), 404 treated as success, non-404 error propagated
- [ ] 1.3 Add `DeleteExtensionVersion(extensionName, version string) error` to `pkg/installer/extension_client.go` — raw REST `DELETE /platform/extensions/v2/extensions/{name}/{version}`; treat 404 as success
- [ ] 1.4 Add unit tests for `DeleteExtensionVersion` in `pkg/installer/extension_client_test.go`: success (202), 404 treated as success, non-404 error propagated

## 2. otel.go: add deactivation helper

- [ ] 2.1 Add `deactivateHostMonitoringExtensionFn` package-level var (test hook) and `deactivateHostMonitoringExtension(envURL, platformToken string)` to `pkg/installer/otel/otel.go`, mirroring the existing `activateHostMonitoringExtension` pattern
- [ ] 2.2 Inside `deactivateHostMonitoringExtension`: call `DeactivateExtension`, then `LatestExtensionVersion`, then `DeleteExtensionVersion`; on any error print a warning and return (advisory)

## 3. UninstallOtelCollector: signature change, prompt, and extension removal

- [ ] 3.1 Update `UninstallOtelCollector` signature in `pkg/installer/otel/uninstall.go` from `(dryRun bool)` to `(envURL, platformToken string, dryRun bool)`
- [ ] 3.2 Add extension removal to the preview section of `UninstallOtelCollector` when `featureflags.Experimental` is enabled: print the extension name that will be deleted
- [ ] 3.3 Add `uninstallDecision` type, `promptUninstallDecision()` function, and `promptUninstallDecisionFn` test hook to `pkg/installer/otel/uninstall.go`. When experimental is enabled, replace the yes/no `ConfirmProceed` call with the three-option prompt: `[1] Delete all` (default), `[2] Only collector`, `[3] Cancel`. `AutoConfirm` selects option 1.
- [ ] 3.4 After local cleanup, call `deactivateHostMonitoringExtensionFn(envURL, platformToken)` only when the user selected `uninstallAll` (not when `uninstallCollectorOnly` or experimental is off)

## 4. cmd/uninstall.go: credential resolution

- [ ] 4.1 Update `uninstallOtelCmd.RunE` in `cmd/uninstall.go` to always call `getDtEnvironment()` and pass `envURL` and `platformToken` to `UninstallOtelCollector`, matching the pattern used by `uninstallAzureCmd` and `uninstallGCPCmd`

## 5. Tests

- [ ] 5.1 Add tests to `pkg/installer/otel/otel_test.go` covering: extension removed when user selects Delete all (experimental on), extension not removed when user selects Only collector, no extension removal when experimental off, advisory warning when deletion fails, dry-run skips deletion but shows preview line. Stub `promptUninstallDecisionFn` where needed to avoid stdin reads.
- [ ] 5.2 Verify all existing `UninstallOtelCollector` call sites compile after the signature change (only `cmd/uninstall.go` calls it)

## 6. Verify

- [ ] 6.1 Run `make build` — confirm no compile errors
- [ ] 6.2 Run `make test` — confirm all tests pass
- [ ] 6.3 Run `make lint` — confirm no new lint issues
