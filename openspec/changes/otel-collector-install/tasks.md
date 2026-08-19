# Tasks: Route App Telemetry Through Local OTel Collector on Install

## 1. Update `generateBaseOtelEnvVars`

- [x] Change signature from `(apiURL, token, serviceName string)` to `(collectorEndpoint, serviceName string)` in `pkg/installer/otel/env.go`
- [x] Remove `OTEL_EXPORTER_OTLP_HEADERS` from the returned map
- [x] Set `OTEL_EXPORTER_OTLP_ENDPOINT` to `collectorEndpoint` directly (no suffix appended)
- [x] Update `generateOtelNodeEnvVars` and `generateOtelPythonEnvVars` in their respective files to match the new signature
- [x] Update `env_test.go` to reflect the new signature and assert the new values

## 2. Update the bundled flow (`install otel`)

- [x] Add `httpPort int` parameter to `createRuntimePlan` in `pkg/installer/otel/otel.go`
- [x] Build `collectorEndpoint = fmt.Sprintf("http://localhost:%d", httpPort)` inside `createRuntimePlan` and pass it to all runtime plan builders
- [x] Pass `cp.httpPort` when calling `createRuntimePlan` in `InstallOtelCollectorWithProject`
- [x] Update `buildJavaInstrumentationPlan`, `buildNodeInstrumentationPlan`, `buildPythonInstrumentationPlan` signatures to accept `collectorEndpoint` instead of `apiURL` + `token` (where used for env var generation only)

## 3. Update standalone `install otel-java`

- [x] In `InstallOtelJava` (`pkg/installer/otel/java.go`), resolve the collector HTTP port via `otlpHTTPPortFromConfig(findExistingCollectorConfig())` (fallback 4318)
- [x] Build `collectorEndpoint` from the resolved port and pass to `generateBaseOtelEnvVars`
- [x] Remove the `token` argument from `generateBaseOtelEnvVars` call sites in `java.go`
- [x] Update `DetectJavaPlan` to pass `collectorEndpoint` rather than `apiURL` + `token` to `buildJavaInstrumentationPlan`

## 4. Update standalone `install otel-node`

- [x] In `InstallOtelNode` (`pkg/installer/otel/nodejs.go`), resolve the collector HTTP port and build `collectorEndpoint`
- [x] Pass `collectorEndpoint` to `generateOtelNodeEnvVars` and `buildNodeInstrumentationPlan`
- [x] Update `DetectNodePlan` accordingly

## 5. Update standalone `install otel-python`

- [x] In `InstallOtelPython` (`pkg/installer/otel/python.go`), resolve the collector HTTP port and build `collectorEndpoint`
- [x] Pass `collectorEndpoint` to `generateOtelPythonEnvVars` and `buildPythonInstrumentationPlan`
- [x] Update `DetectPythonPlan` / `detectPythonPlanWithConfirmedRuntime` accordingly

## 6. Update Go instrumentation

- [x] In `DetectGoPlan` (`pkg/installer/otel/golang.go`), resolve the collector HTTP port and build `collectorEndpoint`
- [x] Pass `collectorEndpoint` to `generateBaseOtelEnvVars`

## 7. Update `updateOtelCollectorIfPresent` call sites

- [x] Verify that `updateOtelCollectorIfPresent` in `java.go` is still needed (it patches the collector config with the DT exporter — orthogonal to this change); leave it in place unless it becomes redundant
- [x] Remove any `apiURL`/`token` arguments from `generateBaseOtelEnvVars` call sites that were passing them purely for OTLP endpoint construction

## 8. Tests

- [x] Unit tests for `generateBaseOtelEnvVars`: assert `OTEL_EXPORTER_OTLP_ENDPOINT = http://localhost:4318`, no `OTEL_EXPORTER_OTLP_HEADERS` key present
- [x] Unit tests for `generateOtelNodeEnvVars` and `generateOtelPythonEnvVars`: same assertions plus runtime-specific keys
- [x] Update any existing tests in `java_test.go`, `nodejs_test.go`, `python_test.go`, `golang_test.go` that assert the old DT endpoint or auth header values
- [x] Test that `createRuntimePlan` correctly threads `cp.httpPort` into the generated env vars (use a non-default port, e.g. 4320, to catch hardcoded assumptions)

## 9. Fix `localhost` → `127.0.0.1` and add collector readiness warning

- [x] Replace `fmt.Sprintf("http://localhost:%d", ...)` with `fmt.Sprintf("http://127.0.0.1:%d", ...)` in all standalone installers: `InstallOtelJava` (`java.go`), `InstallOtelNode` (`nodejs.go`), `InstallOtelPython` (`python.go`), `DetectGoPlan` (`golang.go`)
- [x] Replace the same pattern in `createRuntimePlan` (`otel.go`) for the bundled flow
- [x] Replace the same pattern in `updateDynatraceCollector` (`update_dynatrace.go:155`) for consistency when retargeting connected services
- [x] After env vars are written in `InstallOtelJava`, `InstallOtelNode`, and `InstallOtelPython`: TCP-dial `127.0.0.1:<port>` (short timeout, e.g. 2s); if nothing answers, print a clear warning ("OTel Collector not reachable on port X — telemetry will not reach Dynatrace until a collector is started") and exit cleanly
- [x] Do NOT add the TCP check to `DetectJavaPlan`, `DetectNodePlan`, `DetectPythonPlan`, or `DetectGoPlan` — those are called during passive analysis (`analyze`, `recommend`) and must remain side-effect-free
- [x] Update tests: assert `127.0.0.1` in generated endpoints (replacing any remaining `localhost` assertions); add a test for the warning path when the port is not listening

## 10. Verification

- [x] `make test` — all tests pass
- [x] `make lint` — no new issues
