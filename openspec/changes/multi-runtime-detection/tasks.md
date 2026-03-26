## 1. Java Detection & Plan

- [ ] 1.1 Add `JavaProject` struct and `detectJavaProjects()` scanner (scan for `pom.xml`, `build.gradle`, `build.gradle.kts`) in `otel_java.go`
- [ ] 1.2 Add `detectJavaProcesses()` and `matchJavaProcessesToProjects()` in `otel_java.go`
- [ ] 1.3 Define `JavaInstrumentationPlan` struct with project, agent JAR path, env vars, `EnvURL`, `PlatformToken`
- [ ] 1.4 Implement `DetectJavaPlan(apiURL, token string) *JavaInstrumentationPlan` — project listing, user prompt, plan assembly
- [ ] 1.5 Implement `PrintPlanSteps()` on `JavaInstrumentationPlan`
- [ ] 1.6 Implement `Execute()` on `JavaInstrumentationPlan` — download agent JAR, print `-javaagent` flag and env vars

## 2. Node.js Detection & Plan

- [ ] 2.1 Create `otel_nodejs.go` with `NodeProject` struct and `detectNodeProjects()` scanner (scan for `package.json`, exclude `node_modules`)
- [ ] 2.2 Add `detectNodeProcesses()` and `matchNodeProcessesToProjects()` in `otel_nodejs.go`
- [ ] 2.3 Add `detectNodeEntrypoints()` — parse `package.json` `main`/`scripts.start`, fall back to `index.js`/`app.js`/`server.js`
- [ ] 2.4 Define `NodeInstrumentationPlan` struct with project, entrypoints, env vars, `EnvURL`, `PlatformToken`
- [ ] 2.5 Implement `DetectNodePlan(apiURL, token string) *NodeInstrumentationPlan` — project listing, user prompt, entrypoint detection, plan assembly
- [ ] 2.6 Implement `PrintPlanSteps()` on `NodeInstrumentationPlan`
- [ ] 2.7 Implement `Execute()` on `NodeInstrumentationPlan` — npm install OTel packages, launch with `--require` flag
- [ ] 2.8 Add `generateOtelNodeEnvVars()` function

## 3. Go Detection & Plan

- [ ] 3.1 Create `otel_go.go` with `GoProject` struct and `detectGoProjects()` scanner (scan for `go.mod`, extract module name)
- [ ] 3.2 Define `GoInstrumentationPlan` struct with project, module name, env vars, `EnvURL`, `PlatformToken`
- [ ] 3.3 Implement `DetectGoPlan(apiURL, token string) *GoInstrumentationPlan` — project listing, user prompt, plan assembly
- [ ] 3.4 Implement `PrintPlanSteps()` on `GoInstrumentationPlan` — label as "SDK integration (manual)"
- [ ] 3.5 Implement `Execute()` on `GoInstrumentationPlan` — print `go get` commands, env vars, and SDK initialization guidance
- [ ] 3.6 Add `generateOtelGoEnvVars()` function

## 4. Orchestration in otel.go

- [ ] 4.1 Add `detectAvailableRuntimes()` that uses `exec.LookPath` for each runtime and returns the list of available ones
- [ ] 4.2 Add runtime selection menu with "Skip — collector only" option and "coming soon" labels
- [ ] 4.3 Call the selected runtime's `Detect<Lang>Plan` after menu selection
- [ ] 4.4 Update confirmation preview to show collector + at most one runtime plan
- [ ] 4.5 Update intro message to reflect whether a runtime was selected
- [ ] 4.6 Add execution block for the selected plan after collector install, with separator header
- [ ] 4.7 Pass `EnvURL`, `PlatformToken`, and generated env vars to the selected plan before execution

## 5. Coming Soon & Dry-Run

- [ ] 5.1 Add "coming soon" label rendering for unimplemented runtimes in the selection menu
- [ ] 5.2 Ensure unimplemented runtime entries are not selectable (print "not yet available" on selection)
- [ ] 5.3 Ensure `--dry-run` prints the runtime selection menu and combined preview without installing
- [ ] 5.4 Verify `InstallOtelCollectorOnly()` is not modified and existing tests pass

## 6. Testing

- [ ] 6.1 Add unit tests for `detectJavaProjects()` with temp directory fixtures
- [ ] 6.2 Add unit tests for `detectNodeProjects()` with temp directory fixtures (including `node_modules` exclusion)
- [ ] 6.3 Add unit tests for `detectGoProjects()` and module name extraction
- [ ] 6.4 Add unit tests for "coming soon" label logic
- [ ] 6.5 Run `make test` and `make lint` to verify no regressions
