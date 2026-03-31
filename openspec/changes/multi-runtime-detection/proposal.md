# Proposal

## Why

The `InstallOtelCollector` flow currently detects only Python projects during its preparation phase. Java, Node.js, and Go runtimes are ignored, meaning users must manually instrument those applications after the collector is installed. Extending detection to all supported runtimes makes the guided flow language-agnostic and delivers on the zero-config promise.

## What Changes

- Add `DetectJavaPlan` function and `JavaInstrumentationPlan` struct to `otel_java.go`, following the same pattern as `DetectPythonPlan` / `PythonInstrumentationPlan`.
- Create `otel_nodejs.go` with `DetectNodePlan` / `NodeInstrumentationPlan` — project discovery via `package.json`, entrypoint detection (JS and TypeScript), npm-based OTel SDK installation.
- Create `otel_go.go` with `DetectGoPlan` / `GoInstrumentationPlan` — project discovery via `go.mod`, module name extraction, guidance for compile-time instrumentation.
- Create `otel_common.go` with shared utilities — `scanProjectDirs()`, `detectProcesses()`, `processMatchPIDs()`, `generateBaseOtelEnvVars()`, `getProcessCWD()` — eliminating duplication across runtime files.
- Update `InstallOtelCollector` in `otel.go` to detect available runtimes, scan all GA runtimes for projects, and present a single unified project list. The user picks one project (or skips). Only one project is instrumented per invocation.
- Only Python is GA by default. Java, Node.js, and Go are "coming soon" — their projects are excluded from scanning unless `DTWIZ_ALL_RUNTIMES=true` is set.
- Process detection works cross-platform: Unix (`ps ax`, `lsof`) and Windows (PowerShell `Get-CimInstance`, `WMIC`).
- `--dry-run` covers all new flows (project list, combined preview).
- Existing `InstallOtelCollectorOnly()` flow remains unaffected (no regressions).

## Capabilities

### New Capabilities

- `java-runtime-detection`: Detect Java projects/processes, build a `JavaInstrumentationPlan`, and print instrumentation guidance (agent JAR download, `-javaagent` flag, env vars).
- `nodejs-runtime-detection`: Detect Node.js projects/processes (including TypeScript entrypoints), build a `NodeInstrumentationPlan`, and print instrumentation guidance (`npm install` commands, env vars, `--require` flag).
- `go-runtime-detection`: Detect Go projects, extract module names, build a `GoInstrumentationPlan`, and print compile-time instrumentation guidance (`go get` commands, env vars, SDK initialization).
- `multi-runtime-orchestration`: Update the collector install flow to detect available runtimes, scan all GA runtimes for projects, present a unified project list, and execute the chosen project's instrumentation plan alongside the collector install.

### Modified Capabilities
<!-- No existing specs to modify -->

## Impact

- **Code**: New files `otel_nodejs.go`, `otel_go.go`, `otel_common.go`; extended `otel_java.go`; rewritten `otel.go` orchestration.
- **Dependencies**: No new Go module dependencies. Runtime detection uses `exec.LookPath` and filesystem scanning already established by the Python implementation.
- **UX**: A unified project list shows all detected projects across GA runtimes (plus "Skip — collector only"). The user picks one project; its plan is shown in the confirmation preview alongside the collector. Coming-soon runtimes are excluded from scanning entirely. The confirmation prompt remains a single `Proceed? [Y/n]`.
- **Non-regression**: `InstallOtelCollectorOnly()` path must remain functional and unchanged.

## Rollback Plan

All new code lives in isolated files (`pkg/installer/otel_nodejs.go`, `pkg/installer/otel_go.go`, `pkg/installer/otel_common.go`) or additive changes to existing files (`pkg/installer/otel_java.go`, `pkg/installer/otel.go`). To roll back:

1. **Revert `pkg/installer/otel.go`** — remove the unified project list and selection logic from `InstallOtelCollector()`, restoring the previous Python-only path.
2. **Delete new files** — remove `pkg/installer/otel_nodejs.go`, `pkg/installer/otel_go.go`, and `pkg/installer/otel_common.go`.
3. **Revert `pkg/installer/otel_java.go`** — remove the `JavaInstrumentationPlan`, `DetectJavaPlan`, and related additions; keep existing `detectJava()` and `generateOtelJavaEnvVars()` unchanged.
4. **Revert `pkg/installer/otel_python.go`** — restore duplicated scanning/process detection code that was refactored into `otel_common.go`.
5. No database, config, or external service changes are involved — rollback is purely code deletion and revert.
