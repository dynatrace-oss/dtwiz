## 1. Extend otelConfigData struct

- [ ] 1.1 Add `Debug bool` field to the `otelConfigData` struct in `pkg/installer/otel/collector.go`
- [ ] 1.2 In `generateOtelConfig`, set `Debug: logger.IsDebug()` when constructing the `otelConfigData` value

## 2. Update the OTel config template

- [ ] 2.1 In `pkg/installer/otel/otel.tmpl`, add a conditional `debug` exporter block after the `otlp_http` exporter in the `exporters:` section (verbosity: normal, wrapped in `{{- if .Debug }}`)
- [ ] 2.2 Add `debug` to the exporters list of the `traces` pipeline (conditional on `.Debug`)
- [ ] 2.3 Add `debug` to the exporters list of the `metrics/apps` pipeline (host-monitoring path, conditional on `.Debug`)
- [ ] 2.4 Add `debug` to the exporters list of the `metrics/host` pipeline (host-monitoring path, conditional on `.Debug`)
- [ ] 2.5 Add `debug` to the exporters list of the `metrics` pipeline (non-host-monitoring path, conditional on `.Debug`)
- [ ] 2.6 Add `debug` to the exporters list of the `logs` pipeline (conditional on `.Debug`)

## 3. Tests

- [ ] 3.1 In `pkg/installer/otel/otel_test.go`, add a test case for `renderOtelTemplate` with `Debug: true` (non-host-monitoring): verify `debug:` exporter and `verbosity: normal` appear, and `debug` is in every pipeline's exporter list
- [ ] 3.2 Add a test case for `renderOtelTemplate` with `Debug: true` and `HostMonitoring: true`: verify `debug` appears in all four pipelines (traces, metrics/apps, metrics/host, logs)
- [ ] 3.3 Add a test case (or assert in existing) for `Debug: false`: verify `debug` exporter and `debug` pipeline entries are absent

## 4. Verification

- [ ] 4.1 Run `make test` and confirm all tests pass
- [ ] 4.2 Run `make lint` and confirm no new lint issues
