## 1. Implementation

- [x] 1.1 Update `pkg/installer/otel/collector.go` so generated OTel Collector configs always include host-monitoring settings, Linux journald support, and a health check port without requiring `featureflags.Experimental`.
- [x] 1.2 Update `pkg/installer/otel/otel.go` so install messaging, platform privilege notices, extension activation preview, OpenPipeline route preview, extension activation, and route application are default install behavior while preserving dry-run and missing-token safeguards.
- [x] 1.3 Update `pkg/installer/otel/uninstall.go` so the extension/routes preview and Delete all / Only collector / Cancel prompt are default uninstall behavior.
- [x] 1.4 Update `pkg/installer/otel/update_dynatrace.go` so regenerated Dynatrace Collector configs include host-monitoring settings by default.

## 2. Tests and Documentation

- [x] 2.1 Rewrite `pkg/installer/otel/collector_test.go` tests that assert experimental-gated host config so they assert default host-monitoring config.
- [x] 2.2 Rewrite `pkg/installer/otel/otel_test.go` install/uninstall tests that assert experimental-gated activation, messaging, route setup, and deactivation so they assert default behavior and dry-run safety.
- [x] 2.3 Update OpenSpec deltas for `otel-host-monitoring`, `otel-extension-activation`, `otel-host-monitoring-grail-routes`, `otel-extension-deactivation`, and `otel-collector-update` to remove experimental flag requirements.
- [x] 2.4 Run focused validation: `go test ./pkg/installer/otel`.
