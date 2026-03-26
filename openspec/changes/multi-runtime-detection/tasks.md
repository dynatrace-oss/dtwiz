# Tasks

## 1. Java Detection & Plan

- [ ] 1.1 Add `JavaProject` struct and `detectJavaProjects()` scanner (scan for `pom.xml`, `build.gradle`, `build.gradle.kts`) in `pkg/installer/otel_java.go`
- [ ] 1.2 Add `detectJavaProcesses()` and `matchJavaProcessesToProjects()` in `pkg/installer/otel_java.go`
- [ ] 1.3 Define `JavaInstrumentationPlan` struct with project, agent JAR path, env vars, `EnvURL`, `PlatformToken` in `pkg/installer/otel_java.go`
- [ ] 1.4 Implement `DetectJavaPlan(apiURL, token string) *JavaInstrumentationPlan` — project listing, user prompt, plan assembly in `pkg/installer/otel_java.go`
- [ ] 1.5 Implement `PrintPlanSteps()` on `JavaInstrumentationPlan`
- [ ] 1.6 Implement `Execute()` on `JavaInstrumentationPlan` — download agent JAR, print `-javaagent` flag and env vars

## 2. Node.js Detection & Plan

- [ ] 2.1 Create `pkg/installer/otel_nodejs.go` with `NodeProject` struct and `detectNodeProjects()` scanner (scan for `package.json`, exclude `node_modules`)
- [ ] 2.2 Add `detectNodeProcesses()` and `matchNodeProcessesToProjects()` in `pkg/installer/otel_nodejs.go`
- [ ] 2.3 Add `detectNodeEntrypoints()` in `pkg/installer/otel_nodejs.go` — parse `package.json` `main`/`scripts.start`, fall back to `index.js`/`app.js`/`server.js`
- [ ] 2.4 Define `NodeInstrumentationPlan` struct with project, entrypoints, env vars, `EnvURL`, `PlatformToken` in `pkg/installer/otel_nodejs.go`
- [ ] 2.5 Implement `DetectNodePlan(apiURL, token string) *NodeInstrumentationPlan` — project listing, user prompt, entrypoint detection, plan assembly in `pkg/installer/otel_nodejs.go`
- [ ] 2.6 Implement `PrintPlanSteps()` on `NodeInstrumentationPlan`
- [ ] 2.7 Implement `Execute()` on `NodeInstrumentationPlan` — npm install OTel packages, launch with `--require` flag
- [ ] 2.8 Add `generateOtelNodeEnvVars()` function in `pkg/installer/otel_nodejs.go`

## 3. Go Detection & Plan

- [ ] 3.1 Create `pkg/installer/otel_go.go` with `GoProject` struct and `detectGoProjects()` scanner (scan for `go.mod`, extract module name)
- [ ] 3.2 Define `GoInstrumentationPlan` struct with project, module name, env vars, `EnvURL`, `PlatformToken` in `pkg/installer/otel_go.go`
- [ ] 3.3 Implement `DetectGoPlan(apiURL, token string) *GoInstrumentationPlan` — project listing, user prompt, plan assembly in `pkg/installer/otel_go.go`
- [ ] 3.4 Implement `PrintPlanSteps()` on `GoInstrumentationPlan` — label as "SDK integration (manual)"
- [ ] 3.5 Implement `Execute()` on `GoInstrumentationPlan` — print `go get` commands, env vars, and SDK initialization guidance
- [ ] 3.6 Add `generateOtelGoEnvVars()` function in `pkg/installer/otel_go.go`

## 4. Orchestration in `pkg/installer/otel.go`

- [ ] 4.1 Add `detectAvailableRuntimes()` in `pkg/installer/otel.go` that uses `exec.LookPath` for each runtime and returns the list of available ones
- [ ] 4.2 Add runtime selection menu with "Skip — collector only" option and "coming soon" labels in `pkg/installer/otel.go`
- [ ] 4.3 Call the selected runtime's `Detect<Lang>Plan` after menu selection in `InstallOtelCollector()`
- [ ] 4.4 Update confirmation preview to show collector + at most one runtime plan in `InstallOtelCollector()`
- [ ] 4.5 Update intro message to reflect whether a runtime was selected
- [ ] 4.6 Add execution block for the selected plan after collector install, with separator header
- [ ] 4.7 Pass `EnvURL`, `PlatformToken`, and generated env vars to the selected plan before execution

## 5. Coming Soon & Dry-Run

- [ ] 5.1 Add "coming soon" label rendering for unimplemented runtimes in the selection menu
- [ ] 5.2 Ensure unimplemented runtime entries are not selectable (print "not yet available" on selection)
- [ ] 5.3 Ensure `--dry-run` prints the runtime selection menu and combined preview without installing
- [ ] 5.4 Verify `InstallOtelCollectorOnly()` in `pkg/installer/otel.go` is not modified and existing tests pass

## 6. Testing

Tests derived from spec scenarios to ensure coverage of new behavior and edge cases:

- [ ] 6.1 `pkg/installer/otel_java_test.go`: test `detectJavaProjects()` — Maven project detected (dir with `pom.xml`), Gradle project detected (`build.gradle`/`build.gradle.kts`), no projects found returns nil (spec: Java project scanning)
- [ ] 6.2 `pkg/installer/otel_nodejs_test.go`: test `detectNodeProjects()` — project detected (dir with `package.json`), `node_modules` excluded, no projects found returns nil (spec: Node.js project scanning)
- [ ] 6.3 `pkg/installer/otel_nodejs_test.go`: test `detectNodeEntrypoints()` — entrypoint from `main` field, from `scripts.start`, fallback to `index.js`/`app.js`/`server.js`, no entrypoint returns empty (spec: Node.js entrypoint detection)
- [ ] 6.4 `pkg/installer/otel_go_test.go`: test `detectGoProjects()` — project detected with module name extraction from `go.mod`, no projects found returns nil (spec: Go project scanning)
- [ ] 6.5 `pkg/installer/otel_test.go`: test `detectAvailableRuntimes()` — multiple runtimes on PATH, single runtime, no runtimes (spec: Runtime selection menu)
- [ ] 6.6 `pkg/installer/otel_test.go`: test "coming soon" label logic — unimplemented runtime shown with label, all implemented hides label (spec: Unimplemented runtimes shown as "coming soon")
- [ ] 6.7 Run `make test` and `make lint` to verify no regressions (spec: InstallOtelCollectorOnly non-regression)
