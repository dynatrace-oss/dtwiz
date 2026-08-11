# Proposal: deactivate-host-monitoring-extension

## Why

`dtwiz install otel --experimental` installs and activates the `com.dynatrace.extension.opentelemetry` extension on the tenant, but `dtwiz uninstall otel --experimental` does not remove it. The install and uninstall flows are asymmetric, leaving the extension behind on the tenant after the collector is removed.

## What Changes

- Add `DeactivateExtension(extensionName string) error` to `ExtensionClient` in `pkg/installer/extension_client.go`, calling `DELETE /platform/extensions/v2/extensions/{name}/environment-configuration`.
- Add `DeleteExtensionVersion(extensionName, version string) error` to `ExtensionClient` in `pkg/installer/extension_client.go`, calling `DELETE /platform/extensions/v2/extensions/{name}/{version}`.
- Add `deactivateHostMonitoringExtension(envURL, platformToken string)` to `pkg/installer/otel/otel.go`, mirroring `activateHostMonitoringExtension`. Advisory: warns on failure, never aborts.
- Change `UninstallOtelCollector` signature from `(dryRun bool)` to `(envURL, platformToken string, dryRun bool)` so the uninstall flow can make API calls.
- Update `cmd/uninstall.go` to call `getDtEnvironment()` and pass credentials to `UninstallOtelCollector`, consistent with other uninstall commands (azure, gcp, aws).
- Show the extension removal step in the uninstall preview when `--experimental` is enabled.
- Replace the simple yes/no confirmation with a three-way prompt when `--experimental` is enabled: **Delete all** (default, removes collector, Grail routes, and extension), **Only collector** (removes collector, keeps extension and routes on tenant), and **Cancel**.
- When **Delete all** is selected: remove Grail OpenPipeline dynamic routes for metrics, logs, and spans before deactivating the extension.
- Gate the three-way prompt, route removal, and extension removal behind `featureflags.Experimental`, consistent with the install side.

## Capabilities

### New Capabilities

- `otel-extension-deactivation`: During `dtwiz uninstall otel --experimental`, remove the `com.dynatrace.extension.opentelemetry` extension version from the tenant after the collector and runtime artifacts are cleaned up.

### Modified Capabilities

- `otel-collector-uninstall`: `UninstallOtelCollector` gains `envURL` and `platformToken` parameters; the command layer resolves credentials when experimental is enabled.

## Impact

- `pkg/installer/extension_client.go`: new `DeactivateExtension` and `DeleteExtensionVersion` methods (raw REST, no SDK equivalents)
- `pkg/installer/otel/otel.go`: new `deactivateHostMonitoringExtension`, new `deactivateHostMonitoringExtensionFn` test hook
- `pkg/installer/otel/grail_routes.go`: new `removeGrailRoutes(ctx, grailRouteClient) []error` — removes the OTel routing entry for each of the three signals; reuses the existing `grailRouteClient` interface
- `pkg/installer/otel/otel.go`: new `removeHostMonitoringGrailRoutes(envURL, platformToken string)` (and `removeHostMonitoringGrailRoutesFn` test hook); called as the first step of `deactivateHostMonitoringExtension`
- `pkg/installer/otel/uninstall.go`: `UninstallOtelCollector` signature change; `promptUninstallDecision` replaces the yes/no confirmation when experimental is enabled; `promptUninstallDecisionFn` test hook added; extension removal gated on prompt result; preview updated to show route removal
- `cmd/uninstall.go`: `uninstallOtelCmd` resolves credentials via `getDtEnvironment()` and passes them to `UninstallOtelCollector`
- `pkg/installer/extension_client_test.go`: unit tests for `DeactivateExtension` and `DeleteExtensionVersion`
- `pkg/installer/otel/grail_routes_test.go`: unit tests for `removeGrailRoutes`
- `pkg/installer/otel/otel_test.go`: tests for the deactivation path and prompt decision routing
- Gate: `featureflags.Experimental` (existing flag, no new flag needed)
- No new dependencies; no breaking changes to external interfaces
