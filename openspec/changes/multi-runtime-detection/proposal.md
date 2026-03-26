## Why

The `InstallOtelCollector` flow currently detects only Python projects during its preparation phase. Java, Node.js, and Go runtimes are ignored, meaning users must manually instrument those applications after the collector is installed. Extending detection to all supported runtimes makes the guided flow language-agnostic and delivers on the zero-config promise.

## What Changes

- Add `DetectJavaPlan` function and `JavaInstrumentationPlan` struct to `otel_java.go`, following the same pattern as `DetectPythonPlan` / `PythonInstrumentationPlan`.
- Create `otel_nodejs.go` with `DetectNodePlan` / `NodeInstrumentationPlan` — project discovery via `package.json`, entrypoint detection, npm-based OTel SDK installation.
- Create `otel_go.go` with `DetectGoPlan` / `GoInstrumentationPlan` — project discovery via `go.mod`, binary detection, guidance for compile-time instrumentation.
- Update `InstallOtelCollector` in `otel.go` to detect available runtimes, present a single selection menu, and call the corresponding `Detect<Lang>Plan` for the user's choice. Only one runtime is instrumented per invocation.
- Runtimes whose installer is not yet implemented are shown with a "coming soon" label instead of being hidden.
- `--dry-run` covers all new flows (runtime selection menu, combined preview).
- Existing `InstallOtelCollectorOnly()` flow remains unaffected (no regressions).

## Capabilities

### New Capabilities
- `java-runtime-detection`: Detect Java projects/processes, build a `JavaInstrumentationPlan`, and execute auto-instrumentation via the OTel Java agent JAR.
- `nodejs-runtime-detection`: Detect Node.js projects/processes, build a `NodeInstrumentationPlan`, and execute auto-instrumentation via `@opentelemetry/auto-instrumentations-node`.
- `go-runtime-detection`: Detect Go projects/binaries, build a `GoInstrumentationPlan`, and provide compile-time instrumentation guidance (Go lacks a runtime agent).
- `multi-runtime-orchestration`: Update the collector install flow to detect available runtimes, present a selection menu, and execute the chosen runtime's instrumentation plan alongside the collector install.

### Modified Capabilities
<!-- No existing specs to modify -->

## Impact

- **Code**: New files `otel_nodejs.go`, `otel_go.go`; extended `otel_java.go`; modified `otel.go` orchestration.
- **Dependencies**: No new Go module dependencies. Runtime detection uses `exec.LookPath` and filesystem scanning already established by the Python implementation.
- **UX**: A runtime selection menu lists detected runtimes (plus "Skip — collector only"). The user picks one; its plan is shown in the confirmation preview alongside the collector. Unimplemented runtimes appear with a "coming soon" label. The confirmation prompt remains a single `Proceed? [Y/n]`.
- **Non-regression**: `InstallOtelCollectorOnly()` path must remain functional and unchanged.
