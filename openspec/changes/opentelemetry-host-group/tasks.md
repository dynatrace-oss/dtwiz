# Tasks: opentelemetry-host-group

## 1. Data Model

- [x] 1.1 Add `HostGroupID string` field to `otelConfigData` in `pkg/installer/otel/collector.go`
- [x] 1.2 Populate `HostGroupID` via `os.Hostname()` in `generateOtelConfig()` in `pkg/installer/otel/collector.go`

## 2. Template

- [x] 2.1 Add `resource/add-host-group-id` processor block to the `processors:` section of `pkg/installer/otel/otel.tmpl`
- [x] 2.2 Add `resource/add-host-group-id` to the `traces` pipeline processor list in `pkg/installer/otel/otel.tmpl`
- [x] 2.3 Add `resource/add-host-group-id` to the `metrics` pipeline processor list (non-host-monitoring path) in `pkg/installer/otel/otel.tmpl`
- [x] 2.4 Add `resource/add-host-group-id` to the `logs` pipeline processor list in `pkg/installer/otel/otel.tmpl`
- [x] 2.5 Add `resource/add-host-group-id` to the `metrics/apps` pipeline processor list (host-monitoring path) in `pkg/installer/otel/otel.tmpl`
- [x] 2.6 Add `resource/add-host-group-id` to the `metrics/host` pipeline processor list (host-monitoring path) in `pkg/installer/otel/otel.tmpl`
- [x] 2.7 Add `resource/add-host-group-id` to the `logs/host` pipeline processor list (host-monitoring path) in `pkg/installer/otel/otel.tmpl`

## 3. Tests

- [x] 3.1 Add test case to `pkg/installer/otel/collector_test.go` asserting that the generated config (standard mode) contains `resource/add-host-group-id` with the correct hostname value
- [x] 3.2 Add test case to `pkg/installer/otel/collector_test.go` asserting that all pipelines in standard mode reference `resource/add-host-group-id`
- [x] 3.3 Add test case to `pkg/installer/otel/collector_test.go` asserting that all pipelines in host-monitoring mode reference `resource/add-host-group-id`
- [x] 3.4 Add test case covering hostname resolution failure (empty string) — config generation must succeed without error

## 4. Verification

- [x] 4.1 Run `make test` and confirm all tests pass
- [x] 4.2 Run `make lint` and confirm no new lint issues
