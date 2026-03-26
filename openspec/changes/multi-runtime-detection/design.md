# Design

## Context

`InstallOtelCollector` in `otel.go` orchestrates the Dynatrace OTel Collector installation and runtime auto-instrumentation. Today it only detects Python via `DetectPythonPlan`. The codebase already has a partial `otel_java.go` with `detectJava()` and `generateOtelJavaEnvVars()` but no plan struct or detection flow. Node.js and Go have no files at all.

The Python implementation establishes a clear pattern: a `*InstrumentationPlan` struct captures all user choices upfront, detection happens before the confirmation prompt, and execution runs after the collector is installed.

## Goals / Non-Goals

**Goals:**

- Extend the preparation phase of `InstallOtelCollector` to detect Java, Node.js, and Go projects alongside Python.
- Follow the established pattern: `Detect<Lang>Plan()` → `*<Lang>InstrumentationPlan` with `PrintPlanSteps()` and `Execute()`.
- Scan all GA runtimes for projects and present a single unified project list; the user picks one project (or skips). Only one project is instrumented per invocation.
- Keep each runtime's detection and instrumentation logic in its own file (`otel_java.go`, `otel_nodejs.go`, `otel_go.go`).
- Extract shared project scanning, process detection, and env var generation into `otel_common.go` to avoid duplication.
- Support Windows process detection alongside Unix.

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

**Alternative considered:** Supporting multiple project selections in a single invocation. Rejected for now — adds complexity to the confirmation preview, execution ordering, and error handling. Users can re-run `dtwiz install otel` to instrument additional projects.

### 2. Shared utilities in `otel_common.go`

Common logic extracted into `otel_common.go` to eliminate duplication across runtime-specific files:

- `scanProjectDirs(markers, excludeNames)` — scans CWD + home-directory project locations for directories containing marker files.
- `detectProcesses(filterTerm, excludeTerms)` — finds running processes. On Unix uses `ps ax` and `lsof`; on Windows uses PowerShell `Get-CimInstance Win32_Process`.
- `processMatchPIDs(dirPath, procs)` — matches processes to project directories by CWD or command line.
- `generateBaseOtelEnvVars(apiURL, token, serviceName)` — returns the common OTEL_* environment variables shared by all runtimes.
- `getProcessCWD(pid)` — resolves process working directory. On Unix uses `lsof`; on Windows uses `Get-CimInstance`.

### 3. Detection functions per runtime

Each file exports a single detection entry point matching the Python pattern:

| File | Function | Guard |
|------|----------|-------|
| `otel_java.go` | `DetectJavaPlan(apiURL, token string) *JavaInstrumentationPlan` | `exec.LookPath("java")` |
| `otel_nodejs.go` | `DetectNodePlan(apiURL, token string) *NodeInstrumentationPlan` | `exec.LookPath("node")` |
| `otel_go.go` | `DetectGoPlan(apiURL, token string) *GoInstrumentationPlan` | `exec.LookPath("go")` |

Each function: finds projects on the filesystem → detects running processes → matches them → prompts the user to pick a project → infers entrypoints → returns a plan or `nil`.

**Note:** In the unified project list flow, `DetectJavaPlan` / `DetectNodePlan` / `DetectGoPlan` are still available as standalone entry points for direct `dtwiz install otel-java` etc. commands. The unified orchestrator in `otel.go` uses `createRuntimePlan()` which builds plans directly from the selected `detectedProject`, bypassing the per-runtime interactive menus.

### 4. Unified project list replaces runtime menu

Instead of a two-step flow (pick a runtime → pick a project), the orchestrator scans all GA runtimes at once and shows a single list:

```
  Detected projects:
  ──────────────────────────────────────────────────
  [1] Python   /home/user/projects/api  (pyproject.toml)
  [2] Java     /home/user/projects/svc  (pom.xml)
  [3] Node.js  /home/user/projects/web  (package.json)  ← PIDs: 1234
  [4] Skip — collector only
```

The user picks one project directly. This reduces interaction to a single prompt and gives a complete overview of all detected projects regardless of runtime.

### 5. Project detection strategy per runtime

- **Python**: Scan for `pyproject.toml`, `setup.py`, `setup.cfg`, `requirements.txt`, `Pipfile`, `poetry.lock`, `manage.py`. Detect running `python` processes and match by CWD.
- **Java**: Scan for `pom.xml`, `build.gradle`, `build.gradle.kts`. Detect running `java` processes and match by CWD.
- **Node.js**: Scan for `package.json` (excluding `node_modules`). Detect running `node` processes and match by CWD.
- **Go**: Scan for `go.mod`. Note: Go compiles to static binaries, so "auto-instrumentation" means providing OTel SDK integration guidance and env vars rather than attaching an agent.

### 6. Confirmation preview layout

The preview shows `1) OTel Collector` and, if the user selected a project, `2) <Runtime> auto-instrumentation` with the plan's details. If the user chose "Skip", only the collector appears.

### 7. Execution

After collector installation completes, the single selected plan (if any) executes. It prints a separator header (`── <Runtime> auto-instrumentation ──`) before its block.

### 8. Coming-soon runtimes excluded from scanning

Only Python is GA. All other runtimes (Java, Node.js, Go) are "coming soon" by default — their projects are not scanned or shown. The `DTWIZ_ALL_RUNTIMES` environment variable (set to `"true"` or `"1"`) unlocks all runtimes for testing.

**Alternative considered:** Showing coming-soon runtimes in a menu with labels. Rejected — the unified project list shows projects, not runtimes, so there's nothing to label. Coming-soon runtimes are simply excluded from scanning.

### 9. `--dry-run` coverage

All new flows — project scanning, project list, combined preview — SHALL work under `--dry-run`. When `--dry-run` is set, the project list is printed, the combined plan is shown, but no collector or instrumentation is installed.

### 10. Non-regression: `InstallOtelCollectorOnly()`

The existing `InstallOtelCollectorOnly()` code path is not modified by this change. It continues to install the collector without any runtime detection or project list. All new logic lives in `InstallOtelCollector()` only.

## Risks / Trade-offs

- **[Single project per invocation]** → Users who want to instrument multiple projects must run `dtwiz install otel` multiple times. Mitigation: this keeps the UX simple and each run focused; multi-project support can be added later if needed.
- **[Go is fundamentally different]** → Go has no runtime agent. `DetectGoPlan` can detect projects and provide env var configuration + SDK guidance but cannot auto-instrument a running binary. Mitigation: clearly label Go instrumentation as "manual SDK integration" in the preview and execution output.
- **[Partial execution stubs]** → Java/Node/Go execution may initially be incomplete. Mitigation: the proposal scopes execution as non-goal; stubs print clear guidance on what the user needs to do manually.
- **[Non-regression risk]** → Changes to `otel.go` could break `InstallOtelCollectorOnly()`. Mitigation: all new logic is scoped to `InstallOtelCollector()` only; existing tests must continue to pass.
- **[Cross-platform process detection]** → Windows uses PowerShell `Get-CimInstance` which may be slower or unavailable in restricted environments. Mitigation: detection is best-effort; returns nil gracefully on failure.
