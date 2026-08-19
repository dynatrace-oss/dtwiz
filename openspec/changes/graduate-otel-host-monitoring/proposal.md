## Why

OTel host monitoring is implemented but still hidden behind `--experimental` / `DTWIZ_EXPERIMENTAL=true`. Releasing the feature should make `dtwiz install otel` enable host monitoring by default by configuring host metrics and logs, activating the OpenTelemetry Host Monitoring extension, and setting up Smartscape-on-Grail routes.

## What Changes

- Remove the `Experimental` feature flag dependency from OTel host-monitoring install behavior.
- Make host-monitoring collector configuration, extension activation, and OpenPipeline route setup part of the default `dtwiz install otel` flow.
- Make `dtwiz uninstall otel` default to the host monitoring removal prompt so users can remove the tenant extension and routes or keep them.
- Regenerate Dynatrace OTel Collector configs with host monitoring enabled by default.
- Keep the `Experimental` feature flag for unrelated experimental commands such as `install docker`, `install demo`, and `update otel`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `otel-host-monitoring`: host monitoring is no longer gated by `Experimental`.
- `otel-extension-activation`: extension activation runs in the default install flow.
- `otel-host-monitoring-grail-routes`: route setup runs in the default install flow.
- `otel-extension-deactivation`: uninstall always offers the host monitoring removal choice.
- `otel-collector-update`: regenerated Dynatrace collector configs include host-monitoring settings by default.

## Impact

- Code paths: `pkg/installer/otel/collector.go`, `pkg/installer/otel/otel.go`, `pkg/installer/otel/uninstall.go`, `pkg/installer/otel/update_dynatrace.go`.
- Tests: OTel collector config, install flow messaging, extension activation, Grail route setup, and uninstall prompt behavior.
- Rollback: restore the `featureflags.IsEnabled(featureflags.Experimental)` guards around host-monitoring config, previews, activation, route setup, uninstall extension cleanup, and Dynatrace collector config regeneration.
