# Tasks: OTel Collector Home Directory

## 1. Core Implementation

- [x] 1.1 Add `otelCollectorInstallDir()` helper to `pkg/installer/otel_collector.go` that returns `filepath.Join(os.UserHomeDir(), "opentelemetry")` with proper error handling
- [x] 1.2 Update `prepareCollectorPlan()` in `pkg/installer/otel_collector.go` to use `otelCollectorInstallDir()` instead of `filepath.Join(os.Getwd(), "opentelemetry")`
- [x] 1.3 Update `updateOtelCollectorIfPresent()` in `pkg/installer/otel_update.go` to use `otelCollectorInstallDir()` instead of `filepath.Join(os.Getwd(), "opentelemetry")`

## 2. Testing

- [x] 2.1 Add unit test for `otelCollectorInstallDir()` verifying it returns `~/opentelemetry`
- [x] 2.2 Verify existing OTel Collector tests pass with the new install path
- [x] 2.3 Verify `updateOtelCollectorIfPresent` test coverage accounts for home-dir-based path

## 3. Documentation

- [x] 3.1 Add CHANGELOG entry under `[Unreleased]` noting the install directory change as a breaking change
