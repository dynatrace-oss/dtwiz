# Tasks: add-otel-host-monitoring

## 0. Cross-platform reference config validation

- [x] 0.1 Validate the PM-provided `host-metrics.yaml` reference config's `hostmetrics` scrapers with the same Dynatrace OTel Collector version dtwiz installs on Linux, macOS, and Windows. Run both config validation (for example the collector's validate/config-check command, if available) and a startup smoke test so receiver runtime failures are caught, not only YAML/schema errors.
- [x] 0.2 During the smoke test, confirm what metrics are actually collected on each OS and record any receiver errors or unsupported scrapers. `journald` runtime behavior on macOS/Windows is out of scope for this check since it is gated off entirely on those platforms.

## 1. Combined collector template

- [x] 1.1 Extend `pkg/installer/otel/otel.tmpl` with the host-metrics reference receivers (`hostmetrics/10s`, `hostmetrics/5m`, `hostmetrics/1h`), processors (`filter`, `resource_detection`, `transform`, `filter/delete-metrics`), and the `health_check` extension, mirroring the pinned reference config; add a source-URL comment.
- [x] 1.2 Rename the existing app metrics pipeline to `metrics/apps` (otlp to cumulative_to_delta to otlp_http) and add a `metrics/host` pipeline (hostmetrics to filter to resource_detection to transform to filter/delete-metrics to cumulative_to_delta to otlp_http) sharing the single `otlp_http` exporter.
- [x] 1.3 Add a `logs/host` pipeline (Linux only: journald to resource_detection to otlp_http); keep the existing `logs` pipeline (otlp) intact on all platforms. `logs/host` does not receive from `otlp` so OTLP logs are never duplicated.
- [x] 1.4 Add an `IncludeJournald bool` field to `otelConfigData` in `pkg/installer/otel/collector.go` and gate both the `journald` receiver definition and the `logs/host` pipeline behind that field, so the two can never drift out of sync. On non-Linux both are omitted entirely.
- [x] 1.5 Add a `HealthCheckPort` field to `otelConfigData` and template the `health_check` extension endpoint as `0.0.0.0:{{ .HealthCheckPort }}` instead of the reference config's fixed `13133`.

## 2. Config generation

- [x] 2.1 Update `generateOtelConfig` in `pkg/installer/otel/collector.go` to set the platform-aware host-log flag (true on `runtime.GOOS == "linux"`, false otherwise) and render the combined template.
- [x] 2.2 Verify the rendered YAML parses (`yaml.Unmarshal`) as part of generation and return a clear error if it does not.
- [x] 2.3 Extend `findFreePort` probing to also cover the new `HealthCheckPort`, alongside the existing grpc/http/metrics ports, so a dtwiz collector can run alongside an existing one without a port conflict.
- [x] 2.4 Gate the whole capability behind `featureflags.IsEnabled(featureflags.Experimental)` (design Decision 5), the same convention used for `install docker` / `update otel`. When disabled, `generateOtelConfig` SHALL render exactly the pre-change app-only config (no host pipelines, no `hostmetrics`/`journald`/`health_check` receivers or extensions); when enabled, render the full combined config from section 1.

## 3. Install flow

- [x] 3.1 In `pkg/installer/otel/otel.go`, replace the "follow the docs to activate host monitoring" info box (lines ~276-284) with messaging that host monitoring is enabled automatically.
- [x] 3.2 Add a privilege/limitation notice phrased per platform: on Linux and Windows, show it only when the process is not already elevated (suppressed when running as root/Administrator since no action is needed); on macOS, always show it because `system.processes.created` and `process.disk.io` are permanently unavailable regardless of privilege level.
- [x] 3.3 When the experimental flag (2.4) is disabled, display an informational box directing the user to the OpenTelemetry Host Monitoring documentation to activate host monitoring manually.

## 4. Preview, dry-run, verification

- [x] 4.1 Confirm `printConfigPreview` shows a head+tail summary with the token masked: the first lines up to (but not including) any `hostmetrics` scraper block (receiver endpoints), then a truncation note with the hidden line count directing users to `--debug`, then the `service.pipelines` block. Passing `--debug` shows the full config verbatim. A single `Apply? [Y/n]` prompt follows.
- [x] 4.2 Confirm `--dry-run` renders the combined config and makes no filesystem/process changes.
- [x] 4.3 Confirm the existing OTLP round-trip verification still runs unchanged against the shared otlp receiver / otlp_http exporter.

## 5. Tests

- [x] 5.1 Add golden-file/unit tests in `pkg/installer/otel/collector_test.go` (or `otel_test.go`) asserting the rendered Linux config contains the host receivers, all five processors in order, `metrics/host` and `logs/host` pipelines, and preserved app pipelines.
- [x] 5.2 Add a test asserting the combined render always includes a `logs/host` pipeline (all platforms) and that the `journald` receiver and its reference in `logs/host` are both present on Linux or both absent on non-Linux — never mismatched. Assert this by unmarshalling the rendered YAML and checking the `receivers` and `service.pipelines` maps directly.
- [x] 5.3 Add a test asserting the generated config unmarshals as valid YAML and that the token is masked in the preview.
- [x] 5.4 Add tests mirroring `TestInstallDockerCmd_HiddenByDefault` / `TestUpdateOtelCmd_HiddenByDefault` asserting: `generateOtelConfig` produces the app-only config by default, and the combined config only when `--experimental` or `DTWIZ_EXPERIMENTAL=true` is set.

## 6. Docs and verification

- [x] 6.1 Update `CHANGELOG.md` `[Unreleased]` with the host monitoring addition, noting it ships behind `--experimental`.
- [x] 6.2 Run `make test` and `make lint`; fix any new issues.
