# Tasks

## 1. New file: otel_metric_dimensions.go

- [x] 1.1 Create `pkg/installer/otel/otel_metric_dimensions.go` in package `otel`.
- [x] 1.2 Define `otelMetricDimensionsClient` interface with two methods:
      `getMetricDimensionsSetting(ctx) (objectID, schemaVersion string, value map[string]any, err error)` and
      `putMetricDimensionsSetting(ctx, objectID, schemaVersion string, value map[string]any) error`.
- [x] 1.3 Implement `sdkMetricDimensionsClient` backed by `installer.NewExtensionClient`; use the
      same raw HTTP approach as `sdkGrailClient.putRoutingEntries` for the PUT (avoids `CheckResponse`
      dropping `constraintViolations` detail).
- [x] 1.4 Define `otelMetricDimensionsPlan` struct with fields: `enabled bool`, `objectID string`,
      `schemaVersion string`, `value map[string]any` (full value snapshot from GET for merge PUT).
- [x] 1.5 Implement `buildOtelMetricDimensionsPlan(envURL, platformToken string) (*otelMetricDimensionsPlan, error)`:
      creates the client, calls `getMetricDimensionsSetting`, returns a plan.
- [x] 1.6 Implement `applyOtelMetricDimensionsPlan(ctx context.Context, c otelMetricDimensionsClient, plan *otelMetricDimensionsPlan) error`:
      no-ops when `plan == nil` or `plan.enabled`; otherwise clones `plan.value`, sets
      `enableMintV2Ingest: true`, and calls
      `putMetricDimensionsSetting(ctx, plan.objectID, plan.schemaVersion, mergedValue)`.
      The PUT sends the full merged value — required by the API (partial PUT returns 400, confirmed on rx28105).
- [x] 1.7 Implement `printOtelMetricDimensionsPlan(plan *otelMetricDimensionsPlan)`: prints header
      `OpenTelemetry metric dimensions`, section divider, and a single status line for `MintV2 ingest`
      (`"already enabled"` with muted color, or `"needs enabling"` with OK color).
- [x] 1.8 Add package-level `var buildOtelMetricDimensionsPlanFn = buildOtelMetricDimensionsPlan`
      for test injection, matching the pattern of `buildGrailRoutePlansFn`.

## 2. Introduce tenantPrereqs struct and refactor otel.go

- [x] 2.1 In `pkg/installer/otel/otel.go`, define:

  ```go
  type tenantPrereqs struct {
      grailC    grailRouteClient
      grailPlans []grailSignalPlan
      dimClient otelMetricDimensionsClient
      dimPlan   *otelMetricDimensionsPlan
  }
  ```

- [x] 2.2 Change `buildTenantPrerequisitePreview(envURL, platformToken string)` return type from
      `(grailRouteClient, []grailSignalPlan)` to `tenantPrereqs`.
- [x] 2.3 After the existing Grail plan block, call `buildOtelMetricDimensionsPlanFn`; on error
      call `display.PrintWarning("OTel metric dimensions", err)` and leave `dimPlan` nil; on success
      call `printOtelMetricDimensionsPlan`.
- [x] 2.4 Return the fully populated `tenantPrereqs` value.

## 3. Wire apply in collector.go

- [x] 3.1 In `pkg/installer/otel/collector.go`, update the call at line ~1078 to receive
      `tenantPrereqs` (single return value).
- [x] 3.2 After `applyGrailRoutes(prereqs.grailC, prereqs.grailPlans)` at line ~1094, add:

  ```go
  if prereqs.dimPlan != nil {
      if err := applyOtelMetricDimensionsPlan(context.Background(), prereqs.dimClient, prereqs.dimPlan); err != nil {
          display.PrintWarning("OTel metric dimensions", err)
      }
  }
  ```

## 4. Wire apply in update_dynatrace.go

- [x] 4.1 In `pkg/installer/otel/update_dynatrace.go`, update the call at line ~159 to receive
      `tenantPrereqs`.
- [x] 4.2 After `applyGrailRoutes(prereqs.grailC, prereqs.grailPlans)` at line ~185, add the
      same `applyOtelMetricDimensionsPlan` call with warning fallback.

## 5. Tests

- [x] 5.1 Create `pkg/installer/otel/otel_metric_dimensions_test.go`.
- [x] 5.2 Test case: setting already enabled — GET returns `enableMintV2Ingest: true`;
      `applyOtelMetricDimensionsPlan` must not call PUT; plan must have `enabled: true`.
- [x] 5.3 Test case: setting disabled — GET returns `enableMintV2Ingest: false` with other fields present;
      `applyOtelMetricDimensionsPlan` calls PUT with full merged value (`enableMintV2Ingest: true`,
      all other fields preserved verbatim). Verify PUT body contains all original fields.
- [x] 5.4 Test case: GET returns empty objects list — `buildOtelMetricDimensionsPlan` returns a
      non-nil error; callers treat it as a warning (tested at the unit level via the returned error).
- [x] 5.5 Test case: GET returns malformed JSON in `value` — `buildOtelMetricDimensionsPlan` returns
      a descriptive error without panicking.
- [x] 5.6 Test case: GET succeeds, PUT fails — `applyOtelMetricDimensionsPlan` returns a wrapped
      error; the caller prints a warning and continues (test the error return, not the caller).
- [x] 5.7 All test cases use `httptest.NewServer` to serve canned HTTP responses; no real network
      calls. Table-driven where cases share setup shape.
- [x] 5.8 Run `make test` and `make lint`; fix any issues before opening the PR.
