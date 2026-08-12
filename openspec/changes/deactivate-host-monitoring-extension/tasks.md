# Tasks: deactivate-host-monitoring-extension

## 1. ExtensionClient: add DeactivateExtension and DeleteExtensionVersion

- [x] 1.1 Add `DeactivateExtension(extensionName string) error` to `pkg/installer/extension_client.go` — raw REST `DELETE /platform/extensions/v2/extensions/{name}/environment-configuration`; treat 404 as success
- [x] 1.2 Add unit tests for `DeactivateExtension` in `pkg/installer/extension_client_test.go`: success (204), 404 treated as success, non-404 error propagated
- [x] 1.3 Add `DeleteExtensionVersion(extensionName, version string) error` to `pkg/installer/extension_client.go` — raw REST `DELETE /platform/extensions/v2/extensions/{name}/{version}`; treat 404 as success
- [x] 1.4 Add unit tests for `DeleteExtensionVersion` in `pkg/installer/extension_client_test.go`: success (202), 404 treated as success, non-404 error propagated

## 2. otel.go: add deactivation helper

- [x] 2.1 Add `deactivateHostMonitoringExtensionFn` package-level var (test hook) and `deactivateHostMonitoringExtension(envURL, platformToken string)` to `pkg/installer/otel/otel.go`, mirroring the existing `activateHostMonitoringExtension` pattern
- [x] 2.2 Inside `deactivateHostMonitoringExtension`: call `DeactivateExtension`, then `LatestExtensionVersion`, then `DeleteExtensionVersion`; on any error print a warning and return (advisory)

## 3. UninstallOtelCollector: signature change, prompt, and extension removal

- [x] 3.1 Update `UninstallOtelCollector` signature in `pkg/installer/otel/uninstall.go` from `(dryRun bool)` to `(envURL, platformToken string, dryRun bool)`
- [x] 3.2 Add extension removal to the preview section of `UninstallOtelCollector` when `featureflags.Experimental` is enabled: print the extension name that will be deleted
- [x] 3.3 Add `uninstallDecision` type, `promptUninstallDecision()` function, and `promptUninstallDecisionFn` test hook to `pkg/installer/otel/uninstall.go`. When experimental is enabled, replace the yes/no `ConfirmProceed` call with the three-option prompt: `[1] Delete all` (default), `[2] Only collector`, `[3] Cancel`. `AutoConfirm` selects option 1.
- [x] 3.4 After local cleanup, call `deactivateHostMonitoringExtensionFn(envURL, platformToken)` only when the user selected `uninstallAll` (not when `uninstallCollectorOnly` or experimental is off)

## 4. cmd/uninstall.go: credential resolution

- [x] 4.1 Update `uninstallOtelCmd.RunE` in `cmd/uninstall.go` to always call `getDtEnvironment()` and pass `envURL` and `platformToken` to `UninstallOtelCollector`, matching the pattern used by `uninstallAzureCmd` and `uninstallGCPCmd`

## 5. Tests

- [x] 5.1 Add tests to `pkg/installer/otel/otel_test.go` covering: extension removed when user selects Delete all (experimental on), extension not removed when user selects Only collector, no extension removal when experimental off, advisory warning when deletion fails, dry-run skips deletion but shows preview line. Stub `promptUninstallDecisionFn` where needed to avoid stdin reads.
- [x] 5.2 Verify all existing `UninstallOtelCollector` call sites compile after the signature change (only `cmd/uninstall.go` calls it)

## 6. Grail route removal

- [x] 6.1 Add `removeGrailRoutes(ctx context.Context, c grailRouteClient) []error` to `pkg/installer/otel/grail_routes.go`. For each signal in `grailSignals`: call `checkPipeline` to find the pipeline objectId; if absent, skip (success). Call `getRoutingEntries`; find the entry whose `PipelineID` matches the pipeline objectId via `findRoutingEntry`; if absent, skip. Build a new entry slice with that entry removed and call `putRoutingEntries`. Return one error per signal (nil on success or skip).
- [x] 6.2 Add `removeHostMonitoringGrailRoutesFn` test hook and `removeHostMonitoringGrailRoutes(envURL, platformToken string)` to `pkg/installer/otel/otel.go`. Create an `sdkGrailClient`, call `removeGrailRoutes`, and print an advisory warning for each non-nil error. Mirror the advisory pattern of `deactivateHostMonitoringExtension`.
- [x] 6.3 In `deactivateHostMonitoringExtension` in `pkg/installer/otel/otel.go`, call `removeHostMonitoringGrailRoutesFn(envURL, platformToken)` as the first step, before `DeactivateExtension`.
- [x] 6.4 Update the uninstall preview block in `UninstallOtelCollector` (when `featureflags.Experimental` is enabled) to also show the Grail routes that will be removed alongside the extension line.
- [x] 6.5 Add unit tests for `removeGrailRoutes` in `pkg/installer/otel/grail_routes_test.go` (or the existing test file): route removed successfully, route entry absent (treated as success), pipeline not found (treated as success), `putRoutingEntries` fails (error propagated per-signal).
- [x] 6.6 Add tests for the `removeHostMonitoringGrailRoutes` path via `removeHostMonitoringGrailRoutesFn` stub in `otel_test.go`.
- [x] 6.7 In `removeHostMonitoringGrailRoutes` in `pkg/installer/otel/otel.go`, print `"  ✓ OpenPipeline <DisplayName> route removed"` (using `display.ColorOK`) for each signal whose removal succeeded (i.e. `removeGrailRoutes` returned nil for that index). No output for signals that were skipped (pipeline or entry absent).
- [x] 6.8 Add a unit test in `otel_test.go` verifying the per-signal success line is printed when route removal succeeds, and is absent when skipped.

## 7. Verify

- [x] 7.1 Run `make build` — confirm no compile errors
- [x] 7.2 Run `make test` — confirm all tests pass
- [x] 7.3 Run `make lint` — confirm no new lint issues
