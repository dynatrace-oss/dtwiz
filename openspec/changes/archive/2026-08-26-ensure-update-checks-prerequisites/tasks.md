# Tasks: ensure-update-checks-prerequisites

## 1. Preview block in `updateDynatraceCollector`

- [x] 1.1 In `pkg/installer/otel/update_dynatrace.go`, after the connected-services section and before the dry-run short-circuit, add the extension activation preview: call `buildExtensionActivationPreviewFn` and `printExtensionActivationPreview` under the same guards as install (`featureflags.IsEnabled(featureflags.Experimental) && platformToken != ""`); print a warning via `display.PrintWarning` on error, matching install behavior
- [x] 1.2 After the extension preview block, build and print the OpenPipeline route plan: call `buildGrailRoutePlans` and `printGrailPlan`, capturing `(grailC, grailPlans)` for use after confirmation; print a warning on error, matching install behavior

## 2. Post-confirmation block in `updateDynatraceCollector`

- [x] 2.1 After the confirmation check and before the config write, call `activateHostMonitoringExtensionFn(envURL, platformToken)` under the same guard; this is a no-op when the flag is disabled or the token is empty
- [x] 2.2 After extension activation and before the config write, apply the route plan: wait via `waitForGrailPipelinesFn`, rebuild the plan via `buildGrailPlans`, then apply via `applyGrailPlan` and `printGrailApplyResults`, matching the install post-confirmation sequence in `pkg/installer/otel/otel.go`

## 3. Tests

- [x] 3.1 In `pkg/installer/otel/update_dynatrace_test.go`, add a test covering the preview path: override `buildExtensionActivationPreviewFn` and `buildGrailRoutePlans` (via its injectable parts), assert that both preview sections appear in output when `--experimental` is enabled and a platform token is provided
- [x] 3.2 Add a test for the guard: assert neither preview section appears when `--experimental` is disabled or the platform token is empty
- [x] 3.3 Add a test covering the post-confirmation path: override `activateHostMonitoringExtensionFn`, `waitForGrailPipelinesFn`, and `buildGrailPlans`/`applyGrailPlan` stubs, assert they are called after confirmation when the flag and token are present
- [x] 3.4 Add a test covering the dry-run path: assert no activation or route apply occurs, only the preview is shown
- [x] 3.5 Add a test for preview API failure: override `buildExtensionActivationPreviewFn` to return an error; assert a warning is printed and the preview + confirmation continue normally

## 4. Verify

- [x] 4.1 Run `make build` — confirm no compile errors
- [x] 4.2 Run `make test` — confirm all tests pass
- [x] 4.3 Run `make lint` — confirm no new lint issues
