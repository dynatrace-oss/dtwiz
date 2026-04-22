# Tasks

## 1. Enhanced Project Detection

Expand Node.js project detection to support monorepos, Next.js, Nuxt, and package manager identification.

**Files:** `pkg/installer/otel_nodejs.go` (modify), `pkg/installer/otel_nodejs_test.go` (modify)

- [ ] 1.1a Add `isNextJSProject(projectPath string) bool` — check for `next.config.js`, `next.config.ts`, `next.config.mjs`, or `next` in `package.json` dependencies/devDependencies
- [ ] 1.1b Extend `detectNodeFramework()` to also detect Nuxt — check for `nuxt.config.js`, `nuxt.config.ts`, `nuxt.config.mjs`, or `nuxt` in `package.json` dependencies/devDependencies → return `"nuxt"`. Next.js check takes precedence (checked first).
- [ ] 1.2 Add `detectNodePackageManager(projectPath string) string` — check lockfiles: `package-lock.json` → `"npm"`, `yarn.lock` → `"yarn"`, `pnpm-lock.yaml` → `"pnpm"`, fallback → `"npm"`
- [ ] 1.3 Add monorepo detection — parse `package.json` `"workspaces"` field (array of globs or `{"packages": [...]}` object), resolve workspace directories containing `package.json`, include them as individual `ScannedProject` entries alongside the monorepo root
- [ ] 1.4 Update `detectNodeEntrypoints()` to handle Next.js — when `isNextJSProject()` is true, return a marker entrypoint (e.g., `"next:start"`) instead of scanning for `.js` files
- [ ] 1.5 Update `detectNodeEntrypoints()` to handle Nuxt — when `detectNodeFramework()` returns `"nuxt"`, return a marker entrypoint (e.g., `"nuxt:start"`)
- [ ] 1.6 Tests:
  - `TestIsNextJSProject_ConfigFile` (next.config.js detected)
  - `TestIsNextJSProject_PackageDep` (next in dependencies)
  - `TestIsNextJSProject_NotNextJS` (regular project)
  - `TestDetectNodeFramework_NuxtConfigFile` (nuxt.config.ts detected)
  - `TestDetectNodeFramework_NuxtDep` (nuxt in dependencies)
  - `TestDetectNodeFramework_NextTakesPrecedence` (both next and nuxt → returns "next")
  - `TestDetectNodePackageManager_NPM` (package-lock.json)
  - `TestDetectNodePackageManager_Yarn` (yarn.lock)
  - `TestDetectNodePackageManager_PNPM` (pnpm-lock.yaml)
  - `TestDetectNodePackageManager_Default` (no lockfile → npm)
  - `TestDetectNodeProjects_Monorepo` (workspaces field → child packages listed)
  - `TestDetectNodeEntrypoints_NextJS` (returns next marker)
  - `TestDetectNodeEntrypoints_Nuxt` (returns nuxt marker)

## 2. Env Vars and Package List

Update the OTel Node.js env var generation and package list.

**Files:** `pkg/installer/otel_nodejs.go` (modify), `pkg/installer/otel_nodejs_test.go` (modify)

- [ ] 2.1 Update `otelNodePackages` to include: `@opentelemetry/auto-instrumentations-node`, `@opentelemetry/sdk-node`, `@opentelemetry/exporter-trace-otlp-http`, `@opentelemetry/exporter-metrics-otlp-http`, `@opentelemetry/exporter-logs-otlp-http`
- [ ] 2.2 Add `generateOtelNodeEnvVars(apiURL, token, serviceName string) map[string]string` — call `generateBaseOtelEnvVars()` and add `OTEL_NODE_RESOURCE_DETECTORS=all`
- [ ] 2.3 Tests:
  - `TestGenerateOtelNodeEnvVars_IncludesResourceDetectors`
  - `TestGenerateOtelNodeEnvVars_IncludesBaseVars`

## 3. NodeInstrumentationPlan and .otel/ Directory

Update the plan struct and implement `.otel/` directory creation with package installation.

**Files:** `pkg/installer/otel_nodejs.go` (modify), `pkg/installer/otel_nodejs_test.go` (modify)

- [ ] 3.1 Update `NodeInstrumentationPlan` struct: add `PackageManager string`, `OtelDir string`, `Framework string` (holds `"next"`, `"nuxt"`, or `""`), `PlatformToken string`, `EnvURL string`
- [ ] 3.2 Update `buildNodeInstrumentationPlan()` signature to accept `envURL, platformToken string`; populate new struct fields including `OtelDir = filepath.Join(proj.Path, ".otel")`, detect package manager via `detectNodePackageManager()`, detect framework via `detectNodeFramework()`
- [x] 3.3 Implement `createOtelDir(plan *NodeInstrumentationPlan) error` — create `.otel/` directory, write `.otel/package.json` with OTel deps as dependencies
- [x] 3.4 Implement `generateWrapperJS(framework string, envVars map[string]string) string` — generate CJS wrapper script content that sets `process.env` vars, requires `@opentelemetry/auto-instrumentations-node/register`, and delegates to the Next.js CLI (`next/dist/bin/next`). Called only for Next.js. Implement `generateNuxtBootstrapMJS(otelDir string) string` — generate an ESM bootstrap script (`.mjs`) that uses `module.register()` to install `import-in-the-middle` hooks and loads the OTel CJS register via `createRequire`. Called only for Nuxt (Nuxt bypasses the CLI; the Nitro server is launched directly).
- [x] 3.5 Implement `installOtelNodeDeps(otelDir string) error` — run `npm install` inside `.otel/` directory using `exec.Command`
- [x] 3.6 Tests:
  - `TestCreateOtelDir_CreatesPackageJSON`
  - `TestCreateOtelDir_PackageJSONContainsOtelDeps`
  - `TestGenerateWrapperJS_Next_SetsEnvVars`
  - `TestGenerateWrapperJS_Next_DelegatesToNextCLI`
  - `TestGenerateWrapperJS_Nuxt_NoWrapper` (Nuxt passes through `generateWrapperJS` unchanged — it uses `generateNuxtBootstrapMJS` instead)
  - `TestGenerateNuxtBootstrapMJS_ContainsModuleRegister`
  - `TestGenerateNuxtBootstrapMJS_UsesOtelDir`
  - `TestBuildNodeInstrumentationPlan_DetectsNextJS`
  - `TestBuildNodeInstrumentationPlan_DetectsNuxt`
  - `TestBuildNodeInstrumentationPlan_DetectsPackageManager`
  - `TestBuildNodeInstrumentationPlan_SetsOtelDir`

## 4. Full Execute Flow

Rewrite `Execute()` to perform actual installation, process launch, and Dynatrace verification.

**Files:** `pkg/installer/otel_nodejs.go` (modify), `pkg/installer/otel_nodejs_test.go` (modify)

- [x] 4.1 Rewrite `Execute()`: stop running processes (reuse `stopProcesses()`), call `createOtelDir()`, for Next.js/Nuxt write framework wrapper script, call `installOtelNodeDeps()`, build the run command (regular vs Next.js vs Nuxt), set OTEL\_\* env vars on the process, use `StartManagedProcess()` to launch, use `PrintProcessSummary()` for port detection, use `waitForServices()` for Smartscape polling
- [x] 4.2 For regular apps: the run command is `node --require @opentelemetry/auto-instrumentations-node/register <entrypoint>` with CWD set to `.otel/` and entrypoint path adjusted to be relative from `.otel/` (e.g., `../server.js`)
- [x] 4.3 For Next.js apps: the run command is `node otel/next-otel-bootstrap.js start` with CWD set to project root
- [x] 4.4 For Nuxt apps: the run command is `node --import .otel/nuxt-otel-bootstrap.mjs .output/server/index.mjs` with CWD set to project root (launches the Nitro server directly; Nuxt CLI is not used because it spawns child processes that lose OTel registration)
- [x] 4.5 Update `PrintPlanSteps()` to show: project path, package manager, framework status (Next.js/Nuxt if applicable), `.otel/` directory creation, `npm install` in `.otel/`, run command
- [x] 4.6 Update `DetectNodePlan()` to call updated `buildNodeInstrumentationPlan()` with `apiURL, token` (not `envURL, platformToken` — those are set later like Python)
- [x] 4.7 Tests:
  - `TestNodeInstrumentationPlan_PrintPlanSteps_Regular`
  - `TestNodeInstrumentationPlan_PrintPlanSteps_NextJS`
  - `TestNodeInstrumentationPlan_PrintPlanSteps_Nuxt`
  - `TestNodeInstrumentationPlan_PrintPlanSteps_ShowsPackageManager`

## 5. Standalone CLI Command

Register `dtwiz install otel-node` following existing patterns.

**Files:** `cmd/install.go` (modify), `pkg/installer/otel_nodejs.go` (modify)

- [x] 5.1 Add `InstallOtelNode(envURL, token, platformToken, serviceName string, dryRun bool) error` in `otel_nodejs.go` — validate prerequisites (node + npm on PATH), generate env vars via `generateOtelNodeEnvVars()`, dry-run path (print preview and return), detect projects via `DetectNodePlan()`, print plan steps, confirm, set `EnvURL` and `PlatformToken` on plan, execute
- [x] 5.2 Add `var otelNodeServiceName string` and `installOtelNodeCmd` in `cmd/install.go` — `Use: "otel-node"`, `Short: "Set up OpenTelemetry Node.js auto-instrumentation"`, `Args: cobra.NoArgs`. RunE: resolve creds, validate, call `installer.InstallOtelNode()`
- [x] 5.3 Register `installOtelNodeCmd.Flags().StringVar(&otelNodeServiceName, "service-name", ...)` and add command to `installCmd` in `init()`
- [x] 5.4 Tests:
  - Verify `installOtelNodeCmd` is registered (check `installCmd.Commands()` contains `otel-node`)

## 6. Extend Uninstall OTel for Node.js

Add Node.js `.otel/` cleanup to the existing `dtwiz uninstall otel` command.

**Files:** `pkg/installer/otel_uninstall.go` (modify)

- [ ] 6.1 Add `findNodeOtelDirs() []string` — scan CWD and parent directories for `.otel/` directories that contain a `package.json` with `@opentelemetry` in its content
- [ ] 6.2 Add `findInstrumentedNodeProcesses() []otelProcessInfo` — detect running `node` processes whose command line contains `@opentelemetry/auto-instrumentations-node/register`, `.otel/next-otel-bootstrap.js`, or `.otel/nuxt-otel-bootstrap.mjs`
- [ ] 6.3 Extend `UninstallOtelCollector()` — after the existing collector preview section, add a "Node.js instrumentation" subsection showing `.otel/` dirs to remove and instrumented node processes to kill. On confirmation, handle Node.js cleanup alongside collector cleanup.
- [ ] 6.4 Handle the case where only Node.js artifacts exist (no collector) — the "nothing to remove" check must account for Node.js dirs/processes too
- [ ] 6.5 Tests:
  - `TestFindNodeOtelDirs_Found`
  - `TestFindNodeOtelDirs_IgnoresNonOtelDirs`
  - `TestUninstallOtelCollector_IncludesNodeDirs`

## 7. Integration with Combined OTel Flow

Update the `dtwiz install otel` combined flow to pass required parameters to the Node.js branch.

**Files:** `pkg/installer/otel.go` (modify)

- [ ] 7.1 Update `createRuntimePlan()` Node.js branch — pass `envURL` and `platformToken` to `buildNodeInstrumentationPlan()` (currently only passes `apiURL` and `token`)
- [ ] 7.2 Verify Node.js remains behind `DTWIZ_ALL_RUNTIMES` gate — no change needed, just confirm

## 8. End-to-end Validation

- [ ] 8.1 `make test` — all existing + new tests pass
- [ ] 8.2 `make lint` — no new lint issues
- [ ] 8.3 Manual: `dtwiz install otel-node --dry-run` shows correct preview for a regular Node.js project
- [ ] 8.4 Manual: `dtwiz install otel-node` in a real Node.js project — creates `.otel/`, runs `npm install`, starts instrumented app, service appears in Dynatrace
- [ ] 8.5 Manual: `dtwiz install otel-node` in a Next.js project — generates wrapper script, uses Next.js launch command
- [ ] 8.6 Manual: `dtwiz install otel-node` in a Nuxt project — generates wrapper script, uses Nuxt launch command
- [ ] 8.7 Manual: `dtwiz install otel-node` in a monorepo — child packages listed individually
- [ ] 8.8 Manual: `dtwiz uninstall otel` removes `.otel/` dirs and kills instrumented node processes
- [ ] 8.9 Manual: `dtwiz install otel` with `DTWIZ_ALL_RUNTIMES=true` — Node.js projects appear in list
