# Proposal: otel-host-monitoring-extension

## Why

`dtwiz install otel` with `--experimental` configures the OTel Collector to send host metrics and logs in the shape the Dynatrace OpenTelemetry Host Monitoring extension expects, but it does not ensure that extension is installed and active on the tenant. Without it, the data arrives but host and process entities are never created, and Infrastructure & Operations visualizations never appear.

## What Changes

- Add `ActivateExtension(extensionName, version string) error` to `ExtensionClient` in `pkg/installer/extension_client.go`, directly calling `POST /platform/extensions/v2/extensions/{name}/environment-configuration` (not exposed by the dtctl SDK).
- In `InstallOtelCollectorWithProject` (`pkg/installer/otel/otel.go`), after the user confirms but before the collector is installed, gate behind `featureflags.IsEnabled(featureflags.Experimental)`: ensure `com.dynatrace.extension.opentelemetry` is installed and activated. Behavior is advisory: if activation fails or times out, log a warning and proceed rather than aborting the install.
- Scoped to `install otel` only; `update otel` is out of scope for this change.

## Capabilities

### New Capabilities

- `otel-extension-activation`: During `dtwiz install otel --experimental`, ensure the Dynatrace OpenTelemetry Host Monitoring extension (`com.dynatrace.extension.opentelemetry`) is installed from the hub and its environment configuration is activated before the collector starts.

### Modified Capabilities

## Impact

- `pkg/installer/extension_client.go`: new `ActivateExtension(extensionName, version string) error` method making a direct REST call
- `pkg/installer/otel/otel.go`: extension install + activate step inserted after `ConfirmProceed()`, before `cp.execute()`
- `test/e2e/otel_test.go`: `TestOTelHostMonitoring` may need to account for the new activation step
- Gate: `featureflags.Experimental` (existing flag, no new flag needed)
- No new dependencies; no breaking changes
