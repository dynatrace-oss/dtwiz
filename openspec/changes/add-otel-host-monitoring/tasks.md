## 0. Cross-platform reference config validation

- [ ] 0.1 Validate the PM-provided `host-metrics.yaml` reference config with the same Dynatrace OTel Collector version dtwiz installs on Linux, macOS, and Windows. Run both config validation (for example the collector's validate/config-check command, if available) and a startup smoke test so receiver runtime failures are caught, not only YAML/schema errors.
- [ ] 0.2 During the smoke test, confirm what signals are actually collected on each OS: host metrics, host logs, and any receiver errors or unsupported scrapers. Record whether the Linux-specific `journald` receiver is accepted, ignored, noisy, or fatal on macOS and Windows.
- [ ] 0.3 If the reference config does not work cleanly and usefully on all supported OSes as-is, ask the Dynatrace OTel Collector / OpenTelemetry Host Monitoring experts for the supported per-OS host-log configuration before implementation. Capture the decision explicitly: Linux receiver, Windows receiver/source, macOS receiver/source, required privileges, and any resource attributes/transforms needed for the Host Monitoring extension.
- [ ] 0.4 Update this design and task list from the validation result before implementing the template: either document that the PM-provided config is cross-platform as-is, or replace the Linux-only `journald` assumption with the approved per-OS config plan.

## 1. Combined collector template

- [ ] 1.1 Extend `pkg/installer/otel/otel.tmpl` with the host-metrics reference receivers (`hostmetrics/10s`, `hostmetrics/5m`, `hostmetrics/1h`), processors (`filter`, `resource_detection`, `transform`, `filter/delete-metrics`), and the `health_check` extension, mirroring the pinned reference config; add a source-URL comment.
- [ ] 1.2 Rename the existing app metrics pipeline to `metrics/apps` (otlp to cumulativetodelta to otlp_http) and add a `metrics/host` pipeline (hostmetrics to filter to resource_detection to transform to filter/delete-metrics to cumulativetodelta to otlp_http) sharing the single `otlp_http` exporter.
- [ ] 1.3 Add a Linux-only `logs/host` pipeline (otlp and journald to resource_detection to otlp_http) and the `journald` receiver; keep app traces and logs pipelines intact.
- [ ] 1.4 Add an `IncludeJournald`/`IncludeHostLogs` field to `otelConfigData` in `pkg/installer/otel/collector.go` and gate both the `journald` receiver definition and its reference in the `logs/host` pipeline behind the same field, so the two can never drift out of sync.
- [ ] 1.5 Add a `HealthCheckPort` field to `otelConfigData` and template the `health_check` extension endpoint as `0.0.0.0:{{ .HealthCheckPort }}` instead of the reference config's fixed `13133`.

## 2. Config generation

- [ ] 2.1 Update `generateOtelConfig` in `pkg/installer/otel/collector.go` to set the platform-aware host-log flag (true on `runtime.GOOS == "linux"`, false otherwise) and render the combined template.
- [ ] 2.2 Verify the rendered YAML parses (`yaml.Unmarshal`) as part of generation and return a clear error if it does not.
- [ ] 2.3 Extend `findFreePort` probing to also cover the new `HealthCheckPort`, alongside the existing grpc/http/metrics ports, so a dtwiz collector can run alongside an existing one without a port conflict.

## 3. Install flow and foreign-collector conflict detection

- [ ] 3.1 In `pkg/installer/otel/otel.go`, replace the "follow the docs to activate host monitoring" info box (lines ~276-284) with messaging that host monitoring is enabled automatically.
- [ ] 3.2 For each foreign collector found via `findDynatraceOtelCollectors` / `findAllRunningOtelCollectors` with a resolvable config path, read (never write) its config and check for an existing `hostmetrics`/`hostmetrics/*` receiver.
- [ ] 3.3 When a `hostmetrics` receiver is found, search the foreign config's exporters for an endpoint that looks like a Dynatrace ingest URL and extract its tenant ID with `installer.ExtractTenantID`; compare against the tenant ID of the environment being configured. Treat a templated/env-var endpoint (unresolvable statically) as "tenant undetermined," not as a match or a non-match.
- [ ] 3.4 Implement the three outcomes. No hostmetrics or a different tenant: proceed silently as today. Same tenant with hostmetrics already present: warn and prompt the user to skip host monitoring this run (recommended) or proceed anyway with dtwiz's own separate collector, and mention `dtwiz update otel` as the tool to use if they'd rather consolidate onto the existing collector themselves. Tenant undetermined: print a note that the check was inconclusive and proceed by default (no blocking prompt).
- [ ] 3.5 Confirm this logic never opens the foreign config file for writing, and never invokes any merge/patch code path (e.g. `PatchConfigFile`, `mergeExporterIntoYAML`) against it. Those remain exclusive to the `update otel` command.
- [ ] 3.6 Add a one-line notice that some host metrics/logs may require elevated privileges to be collected in full, phrased per platform: root or `systemd-journal` group on Linux, Administrator/Debug privilege on Windows, and a note that some per-process metrics (e.g. process disk IO) are unavailable on macOS regardless of privilege level.

## 4. Preview, dry-run, verification

- [ ] 4.1 Confirm `printConfigPreview` shows the full combined config inline with the token masked and a single `Apply? [Y/n]` prompt.
- [ ] 4.2 Confirm `--dry-run` renders the combined config and makes no filesystem/process changes.
- [ ] 4.3 Confirm the existing OTLP round-trip verification still runs unchanged against the shared otlp receiver / otlp_http exporter.

## 5. Tests

- [ ] 5.1 Add golden-file/unit tests in `pkg/installer/otel/collector_test.go` (or `otel_test.go`) asserting the rendered Linux config contains the host receivers, all five processors in order, `metrics/host` and `logs/host` pipelines, and preserved app pipelines.
- [ ] 5.2 Add a test asserting the macOS/Windows render includes host metrics but excludes `journald` and the host logs pipeline. Assert this by unmarshalling the rendered YAML and checking the `receivers` and `service.pipelines` maps directly, not by checking that the substring `journald` is absent, so a receiver-defined-but-unreferenced (or referenced-but-undefined) drift between the two guards is caught.
- [ ] 5.3 Add a test asserting the generated config unmarshals as valid YAML and that the token is masked in the preview.
- [ ] 5.4 Add a test asserting dtwiz's own managed collector deploys with all four ports (including `HealthCheckPort`) non-conflicting when another OTel Collector process is already running on the host.
- [ ] 5.5 Add tests for the three conflict-detection outcomes: (a) foreign config has no `hostmetrics` receiver or targets a different tenant, so no warning is shown and it proceeds silently; (b) foreign config has `hostmetrics` and the same tenant, so a warning is shown with a skip-or-proceed prompt and no write to the foreign file; (c) foreign config's exporter endpoint is an unresolvable env-var placeholder (matching the reference config's `${env:DT_ENDPOINT}` style), so the tenant is treated as undetermined, an inconclusive note is shown, and it proceeds by default without blocking.
- [ ] 5.6 Add a test asserting the foreign config file is never opened for writing and that no merge/patch function is called during `install otel`, regardless of which of the three outcomes above is hit.

## 6. Docs and verification

- [ ] 6.1 Update `CHANGELOG.md` `[Unreleased]` with the host monitoring addition.
- [ ] 6.2 Run `make test` and `make lint`; fix any new issues.
- [ ] 6.3 Manually verify (or dry-run) `install otel` on Linux and one non-Linux platform to confirm platform-aware config and preview output.
