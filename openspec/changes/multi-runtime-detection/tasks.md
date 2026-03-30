# Tasks

## 1. Shared utilities and Python refactor

Extract duplicated scanning, process detection, and env var generation from `otel_python.go` into a new shared module. Refactor Python to use the shared code. This is the foundation all subsequent tasks depend on.

**Files:** `pkg/installer/otel_common.go` (create), `pkg/installer/otel_python.go` (modify)

- [x] 1.1 Create `otel_common.go` with `scanProjectDirs(markers, excludeNames)` — scans CWD + home-directory project locations for directories containing marker files
- [x] 1.2 Add `detectProcesses(filterTerm, excludeTerms)` — Unix: `ps ax`/`lsof`; Windows: PowerShell `Get-CimInstance Win32_Process`
- [x] 1.3 Add `getProcessCWD(pid)` — Unix: `lsof`; Windows: `Get-CimInstance` executable path fallback
- [x] 1.4 Add `processMatchPIDs(dirPath, procs)` — match processes to project directories by CWD or command line
- [x] 1.5 Add `generateBaseOtelEnvVars(apiURL, token, serviceName)` — common OTEL_* env vars shared by all runtimes
- [x] 1.6 Refactor `otel_python.go` to call shared `scanProjectDirs`, `detectProcesses`, `processMatchPIDs`, and `generateBaseOtelEnvVars` instead of its own duplicated implementations

## 2. Java runtime detection and plan

Add project scanning, process detection, and instrumentation plan for Java. Depends on task 1 (shared utilities).

**Files:** `pkg/installer/otel_java.go` (modify), `pkg/installer/otel_java_test.go` (create)

- [x] 2.1 Add `detectJavaProjects()` using `scanProjectDirs` for `pom.xml`, `build.gradle`, `build.gradle.kts`, `gradlew`, `.mvn`
- [x] 2.2 Add `detectJavaProcesses()` and `matchJavaProcessesToProjects()` using shared `detectProcesses`/`processMatchPIDs`
- [x] 2.3 Define `JavaInstrumentationPlan` struct with project and env vars; implement `Runtime()` satisfying `InstrumentationPlan` interface
- [x] 2.4 Implement `DetectJavaPlan(apiURL, token)` — project listing, user prompt, plan assembly
- [x] 2.5 Implement `PrintPlanSteps()` and `Execute()` on `JavaInstrumentationPlan`
- [x] 2.6 Add tests: Maven project detected, Gradle project detected, no projects returns nil

## 3. Node.js runtime detection and plan

Add project scanning, process detection, entrypoint detection (JS + TypeScript), and instrumentation plan for Node.js. Depends on task 1 (shared utilities).

**Files:** `pkg/installer/otel_nodejs.go` (create), `pkg/installer/otel_nodejs_test.go` (create)

- [x] 3.1 Add `NodeProject` struct and `detectNodeProjects()` using `scanProjectDirs` for `package.json`, excluding `node_modules`
- [x] 3.2 Add `detectNodeProcesses()` and `matchNodeProcessesToProjects()` using shared utilities
- [x] 3.3 Add `detectNodeEntrypoints()` — parse `package.json` `main`/`scripts.start`, fall back to `index.js`/`app.js`/`server.js` (including TypeScript variants `.ts`/`.mts`/`.cts`)
- [x] 3.4 Define `NodeInstrumentationPlan` struct with project, single entrypoint, and env vars; implement `Runtime()` satisfying `InstrumentationPlan` interface (uses `generateBaseOtelEnvVars()` — no separate `generateOtelNodeEnvVars()`)
- [x] 3.5 Implement `DetectNodePlan(apiURL, token)` — project listing, user prompt, entrypoint detection, plan assembly
- [x] 3.6 Implement `PrintPlanSteps()` and `Execute()` — print `npm install` commands for OTel packages, env vars, and instrumented run command with `--require` flag
- [x] 3.7 Add tests: project detected, `node_modules` excluded, entrypoint from `main`/`scripts.start`/fallback, TypeScript variants, no entrypoint returns empty

## 4. Go runtime detection and plan

Add project scanning, module name extraction, and SDK integration guidance for Go. Depends on task 1 (shared utilities).

**Files:** `pkg/installer/otel_go.go` (create), `pkg/installer/otel_go_test.go` (create)

- [x] 4.1 Add `GoProject` struct (with `ModuleName`) and `detectGoProjects()` using `scanProjectDirs` for `go.mod`, extracting module name
- [x] 4.2 Define `GoInstrumentationPlan` struct with `GoProject` (containing module name) and env vars; implement `Runtime()` satisfying `InstrumentationPlan` interface (uses `generateBaseOtelEnvVars()` — no separate `generateOtelGoEnvVars()`)
- [x] 4.3 Implement `DetectGoPlan(apiURL, token)` — project listing, user prompt, plan assembly
- [x] 4.4 Implement `PrintPlanSteps()` (label as "SDK integration (manual)") and `Execute()` — print `go get` commands, env vars, SDK initialization guidance
- [x] 4.5 Add tests: project detected with module name extraction, no projects returns nil

## 5. Orchestration, coming-soon filtering, and dry-run

Replace the runtime selection menu with a unified project list across all GA runtimes. Only Python is GA by default; `DTWIZ_ALL_RUNTIMES=true` unlocks all. Depends on tasks 1–4 (all runtime detectors must exist).

**Files:** `pkg/installer/otel.go` (modify), `pkg/installer/otel_test.go` (modify)

- [x] 5.1 Add `InstrumentationPlan` interface (`Runtime()`, `PrintPlanSteps()`, `Execute()`) and `allRuntimesEnabled()` checking `DTWIZ_ALL_RUNTIMES` env var (`"true"` or `"1"`)
- [x] 5.2 Update `detectAvailableRuntimes()` — Python `enabled: true`, Java/Node.js/Go `enabled: allEnabled`
- [x] 5.3 Add `detectedProject` struct and `detectAllProjects(runtimes)` — scan all enabled runtimes, skip disabled, return unified list
- [x] 5.4 Add `printProjectList()` and `selectProject()` for unified project selection UX
- [x] 5.5 Add `createRuntimePlan()` to dispatch plan creation based on selected project's runtime
- [x] 5.6 Rewrite `InstallOtelCollector()` — scan projects, show unified list, create plan, show combined preview, execute after collector install
- [x] 5.7 `--dry-run` exits after printing collector-only plan, before project scanning begins (no project list shown in dry-run)
- [x] 5.8 Verify `InstallOtelCollectorOnly()` is not modified
- [x] 5.9 Add tests: `detectAvailableRuntimes` enabled defaults (Python enabled), `DTWIZ_ALL_RUNTIMES=true` enables all, `printProjectList` formatting, `detectAllProjects` skips disabled / includes all when unlocked
- [x] 5.10 Run `make test` and `make lint` to verify no regressions
