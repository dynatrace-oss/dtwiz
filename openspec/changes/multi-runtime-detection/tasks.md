# Tasks

## 1. Shared utilities and Python refactor

Extract duplicated scanning, process detection, and env var generation from `otel_python.go` into a new shared module. Refactor Python to use the shared code. This is the foundation all subsequent tasks depend on.

**Files:** `pkg/installer/otel_common.go` (create), `pkg/installer/otel_python.go` (modify)

- [ ] 1.1 Create `otel_common.go` with `scanProjectDirs(markers, excludeNames)` — scans CWD + home-directory project locations for directories containing marker files
- [ ] 1.2 Add `detectProcesses(filterTerm, excludeTerms)` — Unix: `ps ax`/`lsof`; Windows: PowerShell `Get-CimInstance Win32_Process`
- [ ] 1.3 Add `getProcessCWD(pid)` — Unix: `lsof`; Windows: `Get-CimInstance` executable path fallback
- [ ] 1.4 Add `processMatchPIDs(dirPath, procs)` — match processes to project directories by CWD or command line
- [ ] 1.5 Add `generateBaseOtelEnvVars(apiURL, token, serviceName)` — common OTEL_* env vars shared by all runtimes
- [ ] 1.6 Refactor `otel_python.go` to call shared `scanProjectDirs`, `detectProcesses`, `processMatchPIDs`, and `generateBaseOtelEnvVars` instead of its own duplicated implementations

## 2. Java runtime detection and plan

Add project scanning, process detection, and instrumentation plan for Java. Depends on task 1 (shared utilities).

**Files:** `pkg/installer/otel_java.go` (modify), `pkg/installer/otel_java_test.go` (create)

- [ ] 2.1 Add `JavaProject` struct and `detectJavaProjects()` using `scanProjectDirs` for `pom.xml`, `build.gradle`, `build.gradle.kts`
- [ ] 2.2 Add `detectJavaProcesses()` and `matchJavaProcessesToProjects()` using shared `detectProcesses`/`processMatchPIDs`
- [ ] 2.3 Define `JavaInstrumentationPlan` struct with project, env vars, `EnvURL`, `PlatformToken`
- [ ] 2.4 Implement `DetectJavaPlan(apiURL, token)` — project listing, user prompt, plan assembly
- [ ] 2.5 Implement `PrintPlanSteps()` and `Execute()` on `JavaInstrumentationPlan`
- [ ] 2.6 Add tests: Maven project detected, Gradle project detected, no projects returns nil

## 3. Node.js runtime detection and plan

Add project scanning, process detection, entrypoint detection (JS + TypeScript), and instrumentation plan for Node.js. Depends on task 1 (shared utilities).

**Files:** `pkg/installer/otel_nodejs.go` (create), `pkg/installer/otel_nodejs_test.go` (create)

- [ ] 3.1 Add `NodeProject` struct and `detectNodeProjects()` using `scanProjectDirs` for `package.json`, excluding `node_modules`
- [ ] 3.2 Add `detectNodeProcesses()` and `matchNodeProcessesToProjects()` using shared utilities
- [ ] 3.3 Add `detectNodeEntrypoints()` — parse `package.json` `main`/`scripts.start`, fall back to `index.js`/`app.js`/`server.js` (including TypeScript variants `.ts`/`.mts`/`.cts`)
- [ ] 3.4 Define `NodeInstrumentationPlan` struct and `generateOtelNodeEnvVars()`
- [ ] 3.5 Implement `DetectNodePlan(apiURL, token)` — project listing, user prompt, entrypoint detection, plan assembly
- [ ] 3.6 Implement `PrintPlanSteps()` and `Execute()` — npm install OTel packages, launch with `--require` flag
- [ ] 3.7 Add tests: project detected, `node_modules` excluded, entrypoint from `main`/`scripts.start`/fallback, TypeScript variants, no entrypoint returns empty

## 4. Go runtime detection and plan

Add project scanning, module name extraction, and SDK integration guidance for Go. Depends on task 1 (shared utilities).

**Files:** `pkg/installer/otel_go.go` (create), `pkg/installer/otel_go_test.go` (create)

- [ ] 4.1 Add `GoProject` struct (with `ModuleName`) and `detectGoProjects()` using `scanProjectDirs` for `go.mod`, extracting module name
- [ ] 4.2 Define `GoInstrumentationPlan` struct and `generateOtelGoEnvVars()`
- [ ] 4.3 Implement `DetectGoPlan(apiURL, token)` — project listing, user prompt, plan assembly
- [ ] 4.4 Implement `PrintPlanSteps()` (label as "SDK integration (manual)") and `Execute()` — print `go get` commands, env vars, SDK initialization guidance
- [ ] 4.5 Add tests: project detected with module name extraction, no projects returns nil

## 5. Orchestration, coming-soon filtering, and dry-run

Replace the runtime selection menu with a unified project list across all GA runtimes. Only Python is GA by default; `DTWIZ_ALL_RUNTIMES=true` unlocks all. Depends on tasks 1–4 (all runtime detectors must exist).

**Files:** `pkg/installer/otel.go` (modify), `pkg/installer/otel_test.go` (modify)

- [ ] 5.1 Add `allRuntimesEnabled()` checking `DTWIZ_ALL_RUNTIMES` env var (`"true"` or `"1"`)
- [ ] 5.2 Update `detectAvailableRuntimes()` — Python `comingSoon: false`, Java/Node.js/Go `comingSoon: !unlockAll`
- [ ] 5.3 Add `detectedProject` struct and `detectAllProjects(runtimes)` — scan all GA runtimes, skip coming-soon, return unified list
- [ ] 5.4 Add `printProjectList()` and `selectProject()` for unified project selection UX
- [ ] 5.5 Add `createRuntimePlan()` to dispatch plan creation based on selected project's runtime
- [ ] 5.6 Rewrite `InstallOtelCollector()` — scan projects, show unified list, create plan, show combined preview, execute after collector install
- [ ] 5.7 Ensure `--dry-run` prints the project list and combined preview without installing
- [ ] 5.8 Verify `InstallOtelCollectorOnly()` is not modified
- [ ] 5.9 Add tests: `detectAvailableRuntimes` coming-soon defaults (Python GA), `DTWIZ_ALL_RUNTIMES=true` unlocks all, `printProjectList` formatting, `detectAllProjects` skips coming-soon / includes all when unlocked
- [ ] 5.10 Run `make test` and `make lint` to verify no regressions
