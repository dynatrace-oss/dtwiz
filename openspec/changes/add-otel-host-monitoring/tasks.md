# Tasks: add-otel-host-monitoring

## 0. Cross-platform reference config validation

- [ ] 0.1 Validate the PM-provided `host-metrics.yaml` reference config's `hostmetrics` scrapers with the same Dynatrace OTel Collector version dtwiz installs on Linux, macOS, and Windows. Run both config validation (for example the collector's validate/config-check command, if available) and a startup smoke test so receiver runtime failures are caught, not only YAML/schema errors.
- [ ] 0.2 During the smoke test, confirm what metrics are actually collected on each OS and record any receiver errors or unsupported scrapers. `journald` runtime behavior on macOS/Windows is out of scope for this check since it is gated off entirely on those platforms.
- [ ] 0.3 When a new Dynatrace OTel Collector distribution is released, check its [manifest.yaml](https://github.com/Dynatrace/dynatrace-otel-collector/blob/main/manifest.yaml) for `windowseventlogreceiver` (Windows) and `macosunifiedloggingreceiver` (macOS). When either appears, extend the template and add platform coverage for host logs on that OS.

## 1. Combined collector template

- [ ] 1.1 Extend `pkg/installer/otel/otel.tmpl` with the host-metrics reference receivers (`hostmetrics/10s`, `hostmetrics/5m`, `hostmetrics/1h`), processors (`filter`, `resource_detection`, `transform`, `filter/delete-metrics`), and the `health_check` extension, mirroring the pinned reference config; add a source-URL comment.
- [ ] 1.2 Rename the existing app metrics pipeline to `metrics/apps` (otlp to cumulativetodelta to otlp_http) and add a `metrics/host` pipeline (hostmetrics to filter to resource_detection to transform to filter/delete-metrics to cumulativetodelta to otlp_http) sharing the single `otlp_http` exporter.
- [ ] 1.3 Add a `logs/host` pipeline (otlp and, on Linux, journald to resource_detection to otlp_http) on all platforms; keep app traces and logs pipelines intact.
- [ ] 1.4 Add an `IncludeJournald bool` field to `otelConfigData` in `pkg/installer/otel/collector.go` and gate only the `journald` receiver definition and its reference in the `logs/host` pipeline behind that field, so the two can never drift out of sync. The pipeline itself is always emitted.
- [ ] 1.5 Add a `HealthCheckPort` field to `otelConfigData` and template the `health_check` extension endpoint as `0.0.0.0:{{ .HealthCheckPort }}` instead of the reference config's fixed `13133`.

## 2. Config generation

- [ ] 2.1 Update `generateOtelConfig` in `pkg/installer/otel/collector.go` to set the platform-aware host-log flag (true on `runtime.GOOS == "linux"`, false otherwise) and render the combined template.
- [ ] 2.2 Verify the rendered YAML parses (`yaml.Unmarshal`) as part of generation and return a clear error if it does not.
- [ ] 2.3 Extend `findFreePort` probing to also cover the new `HealthCheckPort`, alongside the existing grpc/http/metrics ports, so a dtwiz collector can run alongside an existing one without a port conflict.
- [ ] 2.4 Gate the whole capability behind `featureflags.IsEnabled(featureflags.Experimental)` (design Decision 5), the same convention used for `install docker` / `update otel`. When disabled, `generateOtelConfig` SHALL render exactly the pre-change app-only config (no host pipelines, no `hostmetrics`/`journald`/`health_check` receivers or extensions); when enabled, render the full combined config from section 1.

## 3. Install flow

- [ ] 3.1 In `pkg/installer/otel/otel.go`, replace the "follow the docs to activate host monitoring" info box (lines ~276-284) with messaging that host monitoring is enabled automatically.
- [ ] 3.2 Add a one-line notice that some host metrics/logs may require elevated privileges to be collected in full, phrased per platform: root or `systemd-journal` group on Linux (also required for `process.disk.io`, which is dropped without privileged access), Administrator/Debug privilege on Windows, and a note that `system.processes.created` is Linux-only and some per-process metrics are unavailable on macOS regardless of privilege level.
- [ ] 3.3 When the experimental flag (2.4) is disabled, print a single-line notice that host monitoring can be enabled with `--experimental` or `DTWIZ_EXPERIMENTAL=true`, instead of silently omitting it.

## 4. Preview, dry-run, verification

- [ ] 4.1 Confirm `printConfigPreview` shows the full combined config inline with the token masked and a single `Apply? [Y/n]` prompt.
- [ ] 4.2 Confirm `--dry-run` renders the combined config and makes no filesystem/process changes.
- [ ] 4.3 Confirm the existing OTLP round-trip verification still runs unchanged against the shared otlp receiver / otlp_http exporter.

## 5. Tests

- [ ] 5.1 Add golden-file/unit tests in `pkg/installer/otel/collector_test.go` (or `otel_test.go`) asserting the rendered Linux config contains the host receivers, all five processors in order, `metrics/host` and `logs/host` pipelines, and preserved app pipelines.
- [ ] 5.2 Add a test asserting the macOS/Windows render includes host metrics and the `logs/host` pipeline but excludes the `journald` receiver. Assert this by unmarshalling the rendered YAML and checking the `receivers` and `service.pipelines` maps directly, not by checking that the substring `journald` is absent, so a receiver-defined-but-unreferenced (or referenced-but-undefined) mismatch between the two guards is caught.
- [ ] 5.3 Add a test asserting the generated config unmarshals as valid YAML and that the token is masked in the preview.
- [ ] 5.4 Add tests mirroring `TestInstallDockerCmd_HiddenByDefault` / `TestUpdateOtelCmd_HiddenByDefault` asserting: `generateOtelConfig` produces the app-only config by default, and the combined config only when `--experimental` or `DTWIZ_EXPERIMENTAL=true` is set.

## 6. Docs and verification

- [ ] 6.1 Update `CHANGELOG.md` `[Unreleased]` with the host monitoring addition, noting it ships behind `--experimental`.
- [ ] 6.2 Run `make test` and `make lint`; fix any new issues.
- [ ] 6.3 Manually verify (or dry-run) `install otel --experimental` on Linux and one non-Linux platform to confirm platform-aware config and preview output, and separately verify `install otel` without the flag is unchanged from before this change.
- [ ] 6.4 Once tasks 0–6 are complete and verified, remove the `--experimental` gate (2.4) so host monitoring is unconditionally enabled, matching the zero-config goal, and update this task list, design.md, and proposal.md accordingly.
