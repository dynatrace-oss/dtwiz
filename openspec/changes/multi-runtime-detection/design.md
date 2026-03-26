# Design

## Context

`InstallOtelCollector` in `otel.go` orchestrates the Dynatrace OTel Collector installation and runtime auto-instrumentation. Today it only detects Python via `DetectPythonPlan`. The codebase already has a partial `otel_java.go` with `detectJava()` and `generateOtelJavaEnvVars()` but no plan struct or detection flow. Node.js and Go have no files at all.

The Python implementation establishes a clear pattern: a `*InstrumentationPlan` struct captures all user choices upfront, detection happens before the confirmation prompt, and execution runs after the collector is installed.

## Goals / Non-Goals

**Goals:**

- Extend the preparation phase of `InstallOtelCollector` to detect Java, Node.js, and Go runtimes alongside Python.
- Follow the established pattern: `Detect<Lang>Plan()` → `*<Lang>InstrumentationPlan` with `PrintPlanSteps()` and `Execute()`.
- Present a single runtime selection menu; the user picks one runtime (or skips). Only one runtime is instrumented per invocation.
- Keep each runtime's detection and instrumentation logic in its own file (`otel_java.go`, `otel_nodejs.go`, `otel_go.go`).

**Non-Goals:**

- Full end-to-end execution of Java/Node/Go instrumentation is not required in this change — execution stubs that print TODO/guidance are acceptable. The priority is detection and plan creation.
- Modifying the existing Python flow.
- Support for additional runtimes beyond Java, Node.js, Go.
- Changes to `dtwiz install otel-python` or `dtwiz install otel-java` standalone commands.

## Decisions

### 1. Common InstrumentationPlan interface

Each runtime defines its own struct (`JavaInstrumentationPlan`, `NodeInstrumentationPlan`, `GoInstrumentationPlan`) with the same two methods:

```go
PrintPlanSteps()           // render preview lines for the combined plan
Execute()                  // run the actual instrumentation after collector install
```

No formal Go interface is introduced — the orchestrator in `otel.go` calls the selected plan's methods directly via a nil-check, exactly as it does today for Python. Only one plan is active per invocation. This avoids unnecessary abstraction while keeping the pattern uniform.

**Alternative considered:** A shared `InstrumentationPlan` interface. Rejected because it would require refactoring the existing Python code and the plans have different fields (virtualenv flags, JAR paths, etc.).

**Alternative considered:** Supporting multiple runtime selections in a single invocation. Rejected for now — adds complexity to the confirmation preview, execution ordering, and error handling. Users can re-run `dtwiz install otel` to instrument additional runtimes.

### 2. Detection functions per runtime

Each file exports a single detection entry point matching the Python pattern:

| File | Function | Guard |
|------|----------|-------|
| `otel_java.go` | `DetectJavaPlan(apiURL, token string) *JavaInstrumentationPlan` | `exec.LookPath("java")` |
| `otel_nodejs.go` | `DetectNodePlan(apiURL, token string) *NodeInstrumentationPlan` | `exec.LookPath("node")` |
| `otel_go.go` | `DetectGoPlan(apiURL, token string) *GoInstrumentationPlan` | `exec.LookPath("go")` |

Each function: finds projects on the filesystem → detects running processes → matches them → prompts the user to pick a project → infers entrypoints → returns a plan or `nil`.

### 3. Project detection strategy per runtime

- **Java**: Scan for `pom.xml`, `build.gradle`, `build.gradle.kts`. Detect running `java` processes and match by CWD.
- **Node.js**: Scan for `package.json` (excluding `node_modules`). Detect running `node` processes and match by CWD.
- **Go**: Scan for `go.mod`. Detect running Go binaries by inspecting processes. Note: Go compiles to static binaries, so "auto-instrumentation" means providing OTel SDK integration guidance and env vars rather than attaching an agent.

### 4. Confirmation preview layout

The preview shows `1) OTel Collector` and, if the user selected a runtime, `2) <Runtime> auto-instrumentation` with the plan's details. If the user chose "Skip", only the collector appears.

### 5. Execution

After collector installation completes, the single selected plan (if any) executes. It prints a separator header (`── <Runtime> auto-instrumentation ──`) before its block.

### 6. "Coming soon" for unimplemented runtimes

Runtimes detected on PATH but whose installer is not yet implemented are still listed in the selection menu with a "coming soon" suffix (e.g., `[3] Go (coming soon)`). They are not selectable. This gives users visibility into planned support without hiding information.

**Alternative considered:** Hiding unimplemented runtimes entirely. Rejected because showing them communicates roadmap intent and avoids user confusion when a runtime is installed but not listed.

### 7. `--dry-run` coverage

All new flows — runtime detection, selection menu, combined preview — SHALL work under `--dry-run`. When `--dry-run` is set, the menu is printed, the combined plan is shown, but no collector or instrumentation is installed.

### 8. Non-regression: `InstallOtelCollectorOnly()`

The existing `InstallOtelCollectorOnly()` code path is not modified by this change. It continues to install the collector without any runtime detection or selection menu. All new logic lives in `InstallOtelCollector()` only.

## Risks / Trade-offs

- **[Single runtime per invocation]** → Users who want to instrument multiple runtimes must run `dtwiz install otel` multiple times. Mitigation: this keeps the UX simple and each run focused; multi-runtime support can be added later if needed.
- **[Go is fundamentally different]** → Go has no runtime agent. `DetectGoPlan` can detect projects and provide env var configuration + SDK guidance but cannot auto-instrument a running binary. Mitigation: clearly label Go instrumentation as "manual SDK integration" in the preview and execution output.
- **[Partial execution stubs]** → Java/Node/Go execution may initially be incomplete. Mitigation: the proposal scopes execution as non-goal; stubs print clear guidance on what the user needs to do manually.
- **[Non-regression risk]** → Changes to `otel.go` could break `InstallOtelCollectorOnly()`. Mitigation: all new logic is scoped to `InstallOtelCollector()` only; existing tests must continue to pass.
