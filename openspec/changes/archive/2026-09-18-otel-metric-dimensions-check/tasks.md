# Tasks

## 1. New file: otel_metric_dimensions.go

- [x] 1.1 Create `pkg/installer/otel/otel_metric_dimensions.go` in package `otel`.
- [x] 1.2 Define `otlpMetricDimensionsClient` interface with two methods:
      `getOTLPMetricDimensionsSetting(ctx) (objectID, schemaVersion string, value map[string]any, err error)` and
      `putOTLPMetricDimensionsSetting(ctx, objectID, schemaVersion string, value map[string]any) error`.
- [x] 1.3 Implement `sdkOTLPMetricDimensionsClient` backed by `installer.NewExtensionClient`; use the
      same raw HTTP approach as `sdkGrailClient.putRoutingEntries` for the PUT (avoids `CheckResponse`
      dropping `constraintViolations` detail).
- [x] 1.4 Define `otlpMetricDimensionsPlan` struct with fields: `enabled bool`, `objectID string`,
      `schemaVersion string`, `value map[string]any` (full value snapshot from GET for merge PUT).
- [x] 1.5 Implement `buildOTLPMetricDimensionsPlan(envURL, platformToken string) (otlpMetricDimensionsClient, *otlpMetricDimensionsPlan, error)`:
      creates the client, calls `getOTLPMetricDimensionsSetting`, returns a plan. On HTTP 404 returns
      `nil, nil, nil` (phase 3 tenant — silently skip). On other errors returns the error.
- [x] 1.6 Implement `applyOTLPMetricDimensionsPlan(ctx context.Context, c otlpMetricDimensionsClient, plan *otlpMetricDimensionsPlan) error`:
      no-ops when `plan == nil` or `plan.enabled`; otherwise clones `plan.value`, sets
      `enableMintV2Ingest: true`, and calls `putOTLPMetricDimensionsSetting`.
- [x] 1.7 Implement `applyOTLPMetricDimensions(ctx, c, plan)`: self-contained, no return value.
      Nil plan is a no-op. Apply failure calls `display.PrintWarning`; success prints
      `✓ Advanced OTLP metric dimensions enabled`. Mirrors the `applyGrailRoutes` pattern.
- [x] 1.8 Implement `printOTLPMetricDimensionsPlan(plan)`: prints preview status line for phase 2 OFF.
      Nil plan is a no-op (phase 2 ON and phase 3 produce no output).
- [x] 1.9 Add package-level `var buildOTLPMetricDimensionsPlanFn = buildOTLPMetricDimensionsPlan`
      for test injection.

## 2. Introduce tenantPrereqs struct and refactor otel.go

- [x] 2.1 In `pkg/installer/otel/otel.go`, define:

  ```go
  type tenantPrereqs struct {
      grailC               grailRouteClient
      grailPlans           []grailSignalPlan
      otlpDimensionsClient otlpMetricDimensionsClient
      otlpDimensionsPlan   *otlpMetricDimensionsPlan
  }
  ```

- [x] 2.2 Change `buildTenantPrerequisitePreview(envURL, platformToken string)` return type to
      `tenantPrereqs`.
- [x] 2.3 Call `buildOTLPMetricDimensionsPlanFn` after the Grail plan block. Store client and plan in
      `tenantPrereqs`. On error: log at debug, leave plan nil. On success with `enabled=true`: set plan
      to nil (nothing to do). Call `printOTLPMetricDimensionsPlan(otlpPlan)` to show status in preview
      (phase 2 OFF only — nil plan is a no-op).
- [x] 2.4 After `installer.ConfirmProceed` returns true, call
      `applyOTLPMetricDimensions(ctx, prereqs.otlpDimensionsClient, prereqs.otlpDimensionsPlan)`.
      Self-contained, no return value. No secondary prompt.

## 3. Wire apply in collector.go

- [x] 3.1 In `pkg/installer/otel/collector.go` (`InstallOtelCollectorOnly`), after
      `installer.ConfirmProceed` returns true, call `applyOTLPMetricDimensions` (same pattern).

## 4. Wire apply in update_dynatrace.go

- [x] 4.1 In `pkg/installer/otel/update_dynatrace.go`, after `installer.ConfirmProceed` returns true,
      call `applyOTLPMetricDimensions` (same pattern).

## 5. Tests

- [x] 5.1 Create `pkg/installer/otel/otel_metric_dimensions_test.go`.
- [x] 5.2 Test case: setting already enabled — GET returns `enableMintV2Ingest: true`;
      `applyOTLPMetricDimensionsPlan` must not call PUT; plan must have `enabled: true`.
- [x] 5.3 Test case: setting disabled — GET returns `enableMintV2Ingest: false` with other fields present;
      `applyOTLPMetricDimensionsPlan` calls PUT with full merged value (`enableMintV2Ingest: true`,
      all other fields preserved verbatim). Verify PUT body contains all original fields.
- [x] 5.4 Test case: GET returns HTTP 404 (phase 3 tenant) — `buildOTLPMetricDimensionsPlan` returns
      `nil, nil, nil`; no error.
- [x] 5.5 Test case: GET returns malformed JSON — `getOTLPMetricDimensionsSetting` returns an error.
- [x] 5.6 Test case: GET succeeds, PUT fails — `applyOTLPMetricDimensionsPlan` returns a wrapped error.
- [x] 5.7 All test cases use `httptest.NewServer`; no real network calls. Table-driven where cases
      share setup shape.
