## 1. OpenSpec Artifacts

- [x] 1.1 Create proposal, design, and delta spec for ordering route setup before OTel install telemetry.

## 2. Installer Ordering

- [x] 2.1 Update `pkg/installer/otel/otel.go` so `install otel` applies dynamic routes after extension activation and before collector execution or selected project instrumentation.
- [x] 2.2 Update `pkg/installer/otel/collector.go` so `install otel-collector` applies dynamic routes before collector verification telemetry is emitted.
- [x] 2.3 Validate successfully applied dynamic routes with a bounded readback before collector execution or selected project instrumentation proceeds.

## 3. Tests

- [x] 3.1 Add unit coverage in `pkg/installer/otel/otel_test.go` for route application occurring before collector execution in the skip-project `install otel` path.
- [x] 3.2 Add unit coverage in `pkg/installer/otel/collector_test.go` or existing OTel tests for route application occurring before standalone collector verification.
- [x] 3.3 Add unit coverage in `pkg/installer/otel/grail_routes_test.go` for route validation succeeding after readback and warning conditions when routes are not visible.
- [x] 3.4 Run focused OTel installer tests and `openspec validate ensure-otel-routes-before-traffic`.