<!-- Parallelism: Groups 1+2 (scaffold) can be done first as one unit. Then groups 3+4 (detection) are independent of each other. Group 5 depends on 3+4. Group 6 is independent of 5. -->

## 1. Pre-flight Validation

- [ ] 1.1 Implement `validateNodePrerequisites()` in a new `otel_node.go`: use `exec.LookPath("node")` to check Node.js in PATH, then check at least one of `exec.LookPath("npm")`, `exec.LookPath("yarn")`, `exec.LookPath("pnpm")` succeeds
- [ ] 1.2 Return `fmt.Errorf` with clear user-facing messages on failure (no `os.Exit` — errors bubble to cmd layer)
- [ ] 1.3 Call validation at the start of `InstallOtelNode()` — return error before any detection work
- [ ] 1.4 Add unit tests for the validation function

## 2. CLI Command Registration

- [ ] 2.1 Add `otel-node` subcommand to `installCmd` in `cmd/install.go` — follow existing `otel-python` pattern: `Use: "otel-node"`, `Args: cobra.NoArgs`, `RunE` calls `getDtEnvironment()` + `validateCredentials()` + `installer.InstallOtelNode(envURL, accessTok, platformTok, serviceName, installDryRun)`. The `serviceName` is obtained from `--service-name` flag (default empty, defaulted inside the function like Python does)
- [ ] 2.2 Add `otel-node` subcommand to `uninstallCmd` in `cmd/uninstall.go` — `Args: cobra.NoArgs`, `RunE` calls `installer.UninstallOtelNode(uninstallDryRun)`
- [ ] 2.3 `--dry-run` is already a `PersistentFlag` on both parent commands — no additional flag setup needed

## 3. Node Project Detection (independent of group 4)

- [ ] 3.1 Define `NodeProject` struct: `Path string`, `Name string`, `Markers []string`, `RunningPIDs []int` — mirror `PythonProject` pattern
- [ ] 3.2 Implement `detectNodeProjects() []NodeProject`: scan CWD + immediate subdirectories, then `$HOME/Code`, `$HOME/code`, `$HOME/projects`, `$HOME/src`, `$HOME/dev` (two levels deep) for `package.json` files. Deduplicate via symlink resolution (same pattern as `detectPythonProjects()`)
- [ ] 3.3 Parse `package.json` (use `encoding/json` — `json.Unmarshal` into a struct with `Name`, `Main`, `Scripts.Start` fields) to extract project name and entrypoint candidates
- [ ] 3.4 Implement entrypoint detection: check `main` field, parse `scripts.start` field (extract the script file if it's a simple `node <file>` pattern), then check common filenames (`index.js`, `server.js`, `app.js`, `main.js`) — return first found
- [ ] 3.5 Present detected projects as a numbered selection menu using `bufio.Scanner` (same pattern as Python project selection)
- [ ] 3.6 Add unit tests for project detection and entrypoint resolution

## 4. Node Process Detection (independent of group 3)

- [ ] 4.1 Implement `detectNodeProcesses() []NodeProcess` (define `NodeProcess` struct: `PID int`, `Command string`, `CWD string`): use `exec.Command("ps", "ax", "-o", "pid=,command=")`, filter for lines containing `node` (exclude `npm`, `yarn`, `pnpm` wrapper processes), resolve CWD via `lsof -a -d cwd -p {pid} -Fn` (same pattern as `detectPythonProcesses()`)
- [ ] 4.2 Implement `matchNodeProcessesToProjects()`: add matching process PIDs to `NodeProject.RunningPIDs` (same pattern as `matchProcessesToProjects()`)
- [ ] 4.3 Add unit tests for process detection and correlation

## 5. Package Installation & Instrumented Launch (depends on 3+4)

- [ ] 5.1 Implement `detectPackageManager(projectPath string) string`: check for `package-lock.json` → `"npm"`, `yarn.lock` → `"yarn"`, `pnpm-lock.yaml` → `"pnpm"`, fallback → `"npm"`
- [ ] 5.2 Implement `installNodeOtelPackages(projectPath, pkgManager string) error`: run `{pkgManager} install @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @opentelemetry/exporter-trace-otlp-http @opentelemetry/exporter-metrics-otlp-http @opentelemetry/exporter-logs-otlp-http` via `exec.Command` with `cmd.Dir = projectPath` — capture combined output, print on failure only (same pattern as `installPackages()` in Python)
- [ ] 5.3 Implement `generateOtelNodeEnvVars(apiURL, token, serviceName string) map[string]string` — set `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT` (`apiURL + "/api/v2/otlp"`), `OTEL_EXPORTER_OTLP_HEADERS` (`"Authorization=Api-Token%20" + token`), `OTEL_EXPORTER_OTLP_PROTOCOL` (`"http/protobuf"`), `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE` (`"delta"`), `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`, `OTEL_LOGS_EXPORTER` (all `"otlp"`), `OTEL_NODE_RESOURCE_DETECTORS` (`"all"`)
- [ ] 5.4 Stop existing running processes for the selected project via `stopProcesses()` (reuse from `otel_python.go`)
- [ ] 5.5 Launch: `exec.Command("node", "--require", "@opentelemetry/auto-instrumentations-node/register", entrypoint)` with `cmd.Env = append(os.Environ(), envVars...)`, `cmd.Dir = projectPath`, `cmd.Stdout/Stderr = logFile`, `cmd.Start()` + `cmd.Process.Release()` for detachment (same pattern as Python entrypoint launch)
- [ ] 5.6 Show preview before execution: print install command, launch command, env vars using separator/color pattern, prompt via `confirmProceed("Apply?")`
- [ ] 5.7 After launch: call `detectListeningPort(pid)` (reuse from `otel_python.go`, skip 4317/4318), then `waitForServices(envURL, platformToken, []string{serviceName})` for DQL verification
- [ ] 5.8 In dry-run mode: print all planned commands and env vars, return nil early

## 6. Uninstall Implementation (independent of group 5)

- [ ] 6.1 Create `pkg/installer/otel_node_uninstall.go` with `UninstallOtelNode(dryRun bool) error`
- [ ] 6.2 Find running Node processes with `@opentelemetry/auto-instrumentations-node` in their command via `ps ax` parsing
- [ ] 6.3 Resolve project directory from process CWD, detect package manager via `detectPackageManager()`, run `{pkgManager} uninstall @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node ...` via `exec.Command`
- [ ] 6.4 Show preview using separator/color pattern, prompt via `confirmProceed()`, support dry-run (print `[dry-run]` + preview and return)

## 7. Testing & Validation

- [ ] 7.1 Add unit tests for `detectPackageManager()` (test with each lockfile present)
- [ ] 7.2 Add unit tests for `package.json` parsing and entrypoint resolution
- [ ] 7.3 Manual validation: run a sample Node.js app (Express), run `dtwiz install otel-node`, verify traces in Dynatrace, then `dtwiz uninstall otel-node`
