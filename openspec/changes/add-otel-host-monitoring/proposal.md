## Why

`dtwiz install otel` deploys a Dynatrace OTel Collector that only receives OTLP from instrumented applications. It collects no host-level signals. Today the flow just prints a docs link telling users to wire up host monitoring by hand ([otel.go:276-284](../../../pkg/installer/otel/otel.go)). To honor the core principle "if we detect it, we enable monitoring for it," the same collector should also collect host metrics and logs and ship them in the shape the Dynatrace **OpenTelemetry Host Monitoring** extension expects, with zero extra steps.

## What Changes

- `install otel` additionally configures the managed collector for host monitoring, deploying the receivers, processors, and pipelines from the Dynatrace [host-metrics reference config](https://github.com/Dynatrace/dynatrace-otel-collector/blob/main/config_examples/host-metrics.yaml): tiered `hostmetrics` scrapers (10s/5m/1h), `journald` logs, and the `filter`, `resource_detection`, `transform`, `filter/delete-metrics`, and `cumulativetodelta` processors that format data for the Host Monitoring extension.
- **One collector, combined config.** Host monitoring pipelines are merged into dtwiz's own managed config alongside the existing app pipelines. The app metrics pipeline is renamed (`metrics/apps`), and a `metrics/host` pipeline plus a host-logs pipeline are added. The exporter, endpoint, and auth are shared with the existing config (the same `/api/v2/otlp` target).
- **Cross-platform scope.** Full signal set on Linux (metrics plus `journald` logs). macOS and Windows collect host metrics only; host log collection is omitted where `journald` is unavailable.
- Lifecycle is unchanged: the collector runs as a detached background process, exactly as the current service-monitoring collector does. The install preview shows the full combined config, and the config round-trip verification is unchanged.
- The collector-install preview and info box are updated: host monitoring is now enabled automatically rather than referred out to docs.
- **Gated rollout.** The whole capability ships behind the existing `--experimental` / `DTWIZ_EXPERIMENTAL` feature flag, the same convention already used for `install docker` and `update otel`. Until this change is fully implemented, tested, and promoted, `install otel` behaves exactly as it does today unless the flag is enabled.

## Capabilities

### New Capabilities

- `otel-host-monitoring`: `install otel` deploys host metrics and host logs collection through the managed OTel Collector, using the Dynatrace Host Monitoring reference configuration, with platform-aware signal selection.

### Modified Capabilities

<!-- No existing spec captures the base collector install; uninstall/update specs are unaffected because the managed collector location and exporter are unchanged. -->

## Impact

- **Code:** `pkg/installer/otel/otel.tmpl` (combined template with host pipelines), `pkg/installer/otel/collector.go` (`generateOtelConfig`, `collectorPlan`, preview), `pkg/installer/otel/otel.go` (install flow and info box). New OS-specific templating for `journald` (Linux) versus metrics-only (macOS and Windows).
- **Unaffected:** `otel-collector-uninstall` and `otel-collector-update` specs. The managed collector directory (`~/opentelemetry`) and the shared `/api/v2/otlp` exporter do not change. `WatchIngest` after install is unchanged.
- **Dependencies:** no new Go dependencies. Relies on the existing Dynatrace OTel Collector distribution, which bundles the `hostmetrics`, `journald`, `filter`, `transform`, and `resource_detection` components.
- **Limitations and risk:** some `hostmetrics` scrapers and the `journald` receiver require elevated privileges (root or `systemd-journal` group) on Linux. When the collector runs as an unprivileged user, affected metrics and logs may be incomplete. This is called out in design. No rollback is needed beyond `uninstall otel`, which already removes the managed collector.
- **Rollout:** gated by `featureflags.Experimental` (`--experimental` / `DTWIZ_EXPERIMENTAL`); not active by default until promoted (see design Decision 5).
