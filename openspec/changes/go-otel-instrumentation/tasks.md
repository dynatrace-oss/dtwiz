# Tasks

## 1. Spike: verify otelc v1.0.1 on real project shapes and all platforms

- [x] 1.1 Workspace spike with otelc v1.0.1 (done: results are in `design.md` Context and justify decision 13). Still open: repeat on a module that already has a `go.sum` and confirm it is byte-identical after the build; record the result in `design.md` Context.
- [ ] 1.2 Kill an otelc build mid-way (SIGKILL) and record the state of `go.mod`/`go.sum`. Verify the result confirms or changes the leftover-backup handling in design decision 5.
- [ ] 1.3 Run the same tiny-module build on Linux and Windows (CI runners or machines) and confirm a server span is emitted. Verify by a green CI job or recorded output for each OS.
- [ ] 1.4 Confirm how delta metric temporality is requested for an otelc-built binary (env var) and that `environment.GenerateBaseOtelEnvVars` in `pkg/installer/otel/environment/environment.go` sets it. Verify with a unit test asserting the variable, adding it if missing.

## 2. Detection, entrypoints and registration

- [ ] 2.0 Register the `GoInstrumentation` flag in `pkg/featureflags/` (`--go-instrumentation`, `DTWIZ_GO_INSTRUMENTATION`, default off). Verify with a registry test (default off, env var and CLI override resolve).
- [ ] 2.1 Gate the new Go flow on `GoInstrumentation` in `pkg/installer/otel/otel.go`: when on, detect Go, infer it from `go.mod` in `inferRuntimeFromPath` and drop it from `manualLanguages`; when off, keep current behavior unchanged. Verify with tests in `pkg/installer/otel/otel_test.go` (flag on: Go detected without `AllRuntimes`, `--project` selects Go; flag off: unchanged behavior, no new files) and a notice test naming Go only with the flag. Make sure the flag is also applied for `dtwiz uninstall otel` (`ApplyCLIOverrides` in the uninstall command's `PersistentPreRun`, so `--go-instrumentation` is accepted there).
- [ ] 2.2 Add `pkg/installer/otel/golang_entrypoints.go`: `package main` discovery (depth 5, `testdata`/ignored/nested-module skipping, selected module only) and service naming with collision handling. Verify with table tests in `golang_entrypoints_test.go` covering every scenario in `go-project-detection` (root main, `cmd/*`, depth limit, nested module, library only, name collision).
- [ ] 2.3 Extend process association in `pkg/installer/otel/golang.go` so compiled Go binaries as well as `go run` are matched to projects, with Unix and Windows variants split by build tags. Verify with unit tests using fake process lists (`go run`, compiled binary, scan failure).
- [ ] 2.4 Implement the "already uses OTel SDK" skip (direct `otel/sdk*` or `otel/exporters/*` requirement, ignoring `// indirect`) with a message naming skipped services and the collector endpoints. Verify with tests for skip-all, skip-some and the indirect case in `golang_test.go`.
- [ ] 2.5 Detect Go workspace projects (`go.work` in the project directory or any parent) and route them to the manual plan with the explanation (collector installed, nothing built); the project must still be listed. Verify with tests for `go.work` at the root, in a parent directory, absent, and `--project` pointing at a workspace project.
- [ ] 2.6 Update the supported-runtimes notice text to include Go in the `otel-project-scan` flow. Verify with a test asserting the notice names Go.

## 3. Tier 1: compile-time instrumentation (otelc)

- [ ] 3.1 Add `pkg/installer/otel/golang_otelc.go` with the single `otelcVersion = "v1.0.1"` constant, Go ≥ 1.25 gate, PATH lookup, isolated install into `~/.opentelemetry/go/bin` (`GOBIN`), and `.exe` handling. Verify with tests for version parsing (1.24, 1.25, 1.26, garbage), PATH-wins, and the install command line (fake runner via the existing `CmdRunner`).
- [ ] 3.2 Add `golang_modguard.go` (backup, atomic restore with retry, leftover-backup check). Verify with tests: restore after success, after failure, after context cancellation, leftover differing backup refuses, identical leftover cleans, absent `go.sum` removed afterwards, a module with uncommitted `go.mod`/`go.sum` changes is restored byte-identical and still uses tier 1.
- [ ] 3.3 Implement the tier-1 build (`otelc --work-dir <tmp> go build -o .otel/go/bin/<svc> ./<pkgdir>`) inside the guard, with SIGINT/SIGTERM-aware context and error messages that include `runtime.GOOS/GOARCH` and classify network errors. Verify with a unit test using a fake runner and an integration test (build tag) that builds a tiny module with real otelc and checks the binary exists and `go.mod`/`go.sum` are byte-identical.

## 4. Tier 2: client-only fallback and manual steps

- [ ] 4.1 Add `golang_fallback.go` (and the init source file) generating the overlay build with isolated `go.mod` copy under `.otel/go/`, `-modfile`; all builds set `GOWORK=off`. Verify with tests that overlay JSON escapes Windows paths, one overlay entry exists per main package, and project sources/go.mod are untouched.
- [ ] 4.2 Implement per-service tier selection in the plan: otelc unavailable → all tier 2; otelc build fails → that service tier 2; tier 2 fails → manual steps for that service. Verify with table tests over the combinations in `golang_test.go` and that other services continue.
- [ ] 4.3 Rework manual steps into a real plan with method `manual` (decision 1a): preview section stating why and that the collector is still installed; `Execute()` prints packages, env vars with the local collector endpoint, and the walkthrough link, returns success. Verify with tests that the plan is non-nil for workspace / no toolchain / both-methods-failed, the endpoint is `http://127.0.0.1:<port>`, no `Authorization` header appears, and the generic "Select another project?" prompt is not triggered.
- [ ] 4.4 Integration check in `pkg/installer/otel/otel_test.go` with `createRuntimePlanFn` stubbed to a manual Go plan: the collector installation step runs and the manual instructions print after it; with a failing collector install, the error is reported and instructions are not presented as ready. Verify the tests pass.

## 5. Plan, execute and launch

- [ ] 5.1 Rewrite `PrintPlanSteps()` to show project, method per service, tool to install, "sources not modified", and the first-build network note; honor `--dry-run` and `--yes` through `ShouldProceed`. Verify with output tests for tier 1, tier 2 and mixed plans, and that dry-run creates no files.
- [ ] 5.2 Implement `Execute()`: stop previously running project processes (before overwriting executables), build each service, launch from `.otel/go/bin/<svc>` by absolute path with the service environment, print the process summary, return an error only when no service started, then reuse the existing ingest watch. Verify with tests using fake launchers (all started, one failed, none started) and by manually running `dtwiz install otel` on a sample two-service Go project.
- [ ] 5.3 Ensure the launched service environment contains service name, local collector endpoint, HTTP protocol, delta temporality and no Dynatrace token. Verify with an environment assertion test.

## 6. Uninstall

- [ ] 6.1 Add `pkg/installer/otel/uninstall_go.go` with a `goCleaner` registered in `runtimeCleaners` (`uninstall_runtime.go`): scan from CWD for Go instrumentation directories under `.otel/go/` recognized by marker file; list them in the preview. Verify with tests for found, Node-only `.otel/`, unrelated `.otel/`, and several independent modules in one tree. Register `goCleaner` so it is skipped when `GoInstrumentation` is off; add a test that with the flag off and Go artifacts present, no Go section appears and nothing is stopped or removed.
- [ ] 6.2 Detect dtwiz-launched Go services by executable location under `.otel/go/bin/` (Unix via process list; `uninstall_go_windows.go` via executable path), and never match plain `go run` processes. Verify with unit tests on fake process data for both platforms' parsers.
- [ ] 6.3 Delete directories after stopping services (retry for locked executables via `removeWithRetry`), remove `.otel/` if left empty, keep Node files, preserve and warn about a leftover `modguard/` backup, remove only dtwiz's isolated otelc, support `--dry-run` and the "Nothing to remove" path. Verify with tests for each scenario in `go-otel-uninstall` (including polyglot folder and user-owned otelc in `GOBIN`).

## 7. Cross-platform and integration verification

- [ ] 7.1 Add a CI job (or extend the existing one) that runs the Go unit tests on Linux, macOS and Windows, and the otelc integration test on all three. Verify by a green run on each OS.
- [ ] 7.2 Run `make test` and `make lint`; verify both pass with no new `//nolint` lacking a justification.
- [ ] 7.3 End-to-end manual check on macOS (and Linux/Windows where available): `dtwiz install otel` on a two-service Go project shows server spans and runtime metrics in Dynatrace, `dtwiz uninstall otel` removes everything and `git status` of the project is clean. Record the result in the PR description.
- [ ] 7.4 Update `CHANGELOG.md` (Unreleased: Added Go instrumentation, available behind `--go-instrumentation` / `DTWIZ_GO_INSTRUMENTATION`) and any docs listing supported runtimes; verify the entries exist and links resolve.

## Workflow follow-up

- After implementation and review, sync the delta specs into `openspec/specs/` (`/opsx:sync`) and archive the change (`/opsx:archive`).
