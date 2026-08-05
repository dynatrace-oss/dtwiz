# OTel Host Monitoring: Smartscape on Grail Routes

## Why

The `add-otel-host-monitoring` change makes `install otel` ship host metrics, logs, and spans in the shape the Dynatrace **OpenTelemetry Host Monitoring** extension expects. Activating that extension creates a processing pipeline named "OpenTelemetry Host Monitoring" for each signal type (metrics, logs, spans). But the extension does **not** create the OpenPipeline **dynamic routes** that steer incoming OTLP host data into those pipelines. Per the [Smartscape on Grail docs](https://docs.dynatrace.com/docs/observe/infrastructure-observability/extensions/opentelemetry-host-monitoring#smartscape-on-grail), users must add three dynamic routes by hand (one each for metrics, logs, and spans) before `OTEL_HOST` and `OTEL_PROCESS` entities appear in Smartscape. Until they do, host data is ingested but never routed through the pipeline, so no host/process topology is formed.

This leaves a manual gap in the otherwise zero-config host-monitoring flow. To honor the core principle "if we detect it, we enable monitoring for it," `dtwiz` should create those three dynamic routes automatically after the host-monitoring collector is installed. The creation is additive and only happens when the target pipeline exists.

## What Changes

- After the OTel Collector host-monitoring install completes successfully (non-dry-run), `install otel` reconciles three OpenPipeline dynamic routes (one each for **metrics**, **logs**, and **spans**) that route OTLP host telemetry into the "OpenTelemetry Host Monitoring" pipeline the extension provides.
- **Matching conditions** are the documented ones:
  - Metrics: `matchesValue(metric.key, {"system.*", "process.*"}) AND isNotNull(host.id)`
  - Logs: `isNotNull(host.id) and isNotNull(host.name) and matchesValue(dt.openpipeline.source, "/api/v2/otlp/v1/logs")`
  - Spans: `isNotNull(host.id) and isNotNull(host.name) and matchesValue(telemetry.sdk.name, {"opentelemetry", "odin", "otel"})`
- **Additive and idempotent only.** For each signal type, dtwiz creates the route only if no route to the same pipeline already exists. It never modifies or deletes existing routes, including ones a user created by hand. Re-running `install otel` is a no-op once the routes exist.
- **Safe by default.** dtwiz resolves the target "OpenTelemetry Host Monitoring" pipeline identifier by name at runtime for each signal type. If the pipeline is not found (for example, the extension is not activated), dtwiz skips that route with an informative message and does not fail the install.
- **Preview and confirmation** follow the existing UX: the routes that will be created are listed one line each before anything is written, covered by the existing `Proceed with installation?` prompt (no separate route-only prompt); `--dry-run` shows the plan and writes nothing; `--yes`/`-y` auto-confirms.
- **Gated rollout.** The whole capability ships behind the existing `--experimental` / `DTWIZ_EXPERIMENTAL` feature flag, the same gate host monitoring itself uses. When the flag is off, `install otel` behaves exactly as it does today and touches no OpenPipeline settings.

## Capabilities

### New Capabilities

- `otel-host-monitoring-grail-routes`: after a successful host-monitoring collector install, `install otel` reconciles the three OpenPipeline dynamic routes (metrics, logs, spans) required for Smartscape on Grail, additively and idempotently, gated behind the experimental flag.

### Modified Capabilities

- `otel-host-monitoring` install flow: the host-monitoring install gains a final, optional step that creates the dynamic routes. The collector configuration, lifecycle, preview truncation, and verification from that change are unaffected.

## Impact

- **Code:** new logic under `pkg/installer/otel/` for building and reconciling the dynamic routes, reusing the existing `installer.ExtensionClient` / dtctl `settings.Handler` machinery (platform token, apps URL) already used by the Azure/GCP settings writes. The route step is invoked from the host-monitoring install flow in `pkg/installer/otel/otel.go` after `collectorPlan.execute` succeeds. The route matcher conditions are constants; both the extension-owned pipeline and its Settings objectId are resolved at runtime, never constructed or assumed (see `design.md` Decision 2).
- **Auth:** platform token, via the apps URL, the same path every settings-object write already uses. It DOES require the token to carry the **`settings:objects:read`** and **`settings:objects:write`** scopes documented in `README.md`'s token-scope table (and, if the token's IAM policy restricts those scopes to specific schema IDs, that list must include the `builtin:openpipeline.*` schemas). A token that fails either check gets a bare `403 Access denied` from the Settings API that looks identical whether the scope is missing entirely or just not permitted for this schema; dtwiz cannot distinguish the two from the response, so it SHALL enrich the error with the schema and the scope that normally applies as a starting point for investigation, not a certain diagnosis (see `design.md` Decision 6).
- **Unaffected:** the collector template, `otel-collector-uninstall`, `otel-collector-update`, and `WatchIngest` after install. The `add-otel-host-monitoring` collector config generation is untouched.
- **Out of scope (separate changes):** creating or modifying the "OpenTelemetry Host Monitoring" pipelines themselves (owned by the extension); removing the routes on `uninstall otel`; routes for signal types other than metrics/logs/spans; any non-host OpenPipeline routing.
- **Dependencies:** no new Go dependencies. Relies on the vendored dtctl SDK's Settings 2.0 handler already in use.
- **Limitations and risk:** the OpenPipeline pipeline/routing request and response shapes referenced above were validated end-to-end against two live tenants during a proof-of-concept pass; every shape, error code, and required scope captured in `design.md` reflects that live validation, not assumption. Because the step is strictly additive (appends to the entries array, preserving existing entries) and skips when the pipeline is absent, the worst case is that no route is created and the install still succeeds. No rollback is needed; routes left in place are inert without the corresponding pipeline.
- **Rollout:** gated by `featureflags.Experimental` (`--experimental` / `DTWIZ_EXPERIMENTAL`); not active by default until promoted alongside host monitoring.
- **Implementation status:** the PoC pass above validated the request/response shapes and is reflected in `design.md`; it is not itself the shipped implementation. See `tasks.md` for what remains to build against the finalized spec.
