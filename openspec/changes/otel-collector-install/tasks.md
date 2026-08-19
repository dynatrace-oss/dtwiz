# Tasks: Route App Telemetry Through Local OTel Collector on Install

## 1. Update `generateBaseOtelEnvVars`

- [ ] Change signature from `(apiURL, token, serviceName string)` to `(collectorEndpoint, serviceName string)` in `pkg/installer/otel/env.go`
- [ ] Remove `OTEL_EXPORTER_OTLP_HEADERS` from the returned map
- [ ] Set `OTEL_EXPORTER_OTLP_ENDPOINT` to `collectorEndpoint` directly (no suffix appended)
- [ ] Update `generateOtelNodeEnvVars` and `generateOtelPythonEnvVars` in their respective files to match the new signature
- [ ] Update `env_test.go` to reflect the new signature and assert the new values

## 2. Update the bundled flow (`install otel`)

- [ ] Add `httpPort int` parameter to `createRuntimePlan` in `pkg/installer/otel/otel.go`
- [ ] Build `collectorEndpoint = fmt.Sprintf("http://localhost:%d", httpPort)` inside `createRuntimePlan` and pass it to all runtime plan builders
- [ ] Pass `cp.httpPort` when calling `createRuntimePlan` in `InstallOtelCollectorWithProject`
- [ ] Update `buildJavaInstrumentationPlan`, `buildNodeInstrumentationPlan`, `buildPythonInstrumentationPlan` signatures to accept `collectorEndpoint` instead of `apiURL` + `token` (where used for env var generation only)

## 3. Update standalone `install otel-java`

- [ ] In `InstallOtelJava` (`pkg/installer/otel/java.go`), resolve the collector HTTP port via `otlpHTTPPortFromConfig(findExistingCollectorConfig())` (fallback 4318)
- [ ] Build `collectorEndpoint` from the resolved port and pass to `generateBaseOtelEnvVars`
- [ ] Remove the `token` argument from `generateBaseOtelEnvVars` call sites in `java.go`
- [ ] Update `DetectJavaPlan` to pass `collectorEndpoint` rather than `apiURL` + `token` to `buildJavaInstrumentationPlan`

## 4. Update standalone `install otel-node`

- [ ] In `InstallOtelNode` (`pkg/installer/otel/nodejs.go`), resolve the collector HTTP port and build `collectorEndpoint`
- [ ] Pass `collectorEndpoint` to `generateOtelNodeEnvVars` and `buildNodeInstrumentationPlan`
- [ ] Update `DetectNodePlan` accordingly

## 5. Update standalone `install otel-python`

- [ ] In `InstallOtelPython` (`pkg/installer/otel/python.go`), resolve the collector HTTP port and build `collectorEndpoint`
- [ ] Pass `collectorEndpoint` to `generateOtelPythonEnvVars` and `buildPythonInstrumentationPlan`
- [ ] Update `DetectPythonPlan` / `detectPythonPlanWithConfirmedRuntime` accordingly

## 6. Update Go instrumentation

- [ ] In `DetectGoPlan` (`pkg/installer/otel/golang.go`), resolve the collector HTTP port and build `collectorEndpoint`
- [ ] Pass `collectorEndpoint` to `generateBaseOtelEnvVars`

## 7. Update `updateOtelCollectorIfPresent` call sites

- [ ] Verify that `updateOtelCollectorIfPresent` in `java.go` is still needed (it patches the collector config with the DT exporter — orthogonal to this change); leave it in place unless it becomes redundant
- [ ] Remove any `apiURL`/`token` arguments from `generateBaseOtelEnvVars` call sites that were passing them purely for OTLP endpoint construction

## 8. Tests

- [ ] Unit tests for `generateBaseOtelEnvVars`: assert `OTEL_EXPORTER_OTLP_ENDPOINT = http://localhost:4318`, no `OTEL_EXPORTER_OTLP_HEADERS` key present
- [ ] Unit tests for `generateOtelNodeEnvVars` and `generateOtelPythonEnvVars`: same assertions plus runtime-specific keys
- [ ] Update any existing tests in `java_test.go`, `nodejs_test.go`, `python_test.go`, `golang_test.go` that assert the old DT endpoint or auth header values
- [ ] Test that `createRuntimePlan` correctly threads `cp.httpPort` into the generated env vars (use a non-default port, e.g. 4320, to catch hardcoded assumptions)

## 9. Verification

- [ ] `make test` — all tests pass
- [ ] `make lint` — no new issues
