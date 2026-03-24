## 1. Pre-flight Validation

- [ ] 1.1 Implement `validateNodePrerequisites()` in a new `otel_node.go`: check `node` in PATH, check `npm`/`yarn`/`pnpm` availability
- [ ] 1.2 Call validation at the start of `InstallOtelNode()` — exit with clear error if checks fail
- [ ] 1.3 Add unit tests for the validation function

## 2. CLI Command Registration

- [ ] 2.1 Add `otel-node` subcommand to `installCmd` in `cmd/install.go`, wiring to `InstallOtelNode()`
- [ ] 2.2 Add `otel-node` subcommand to `uninstallCmd` in `cmd/uninstall.go`, wiring to `UninstallOtelNode()`
- [ ] 2.3 Ensure both subcommands set `Args: cobra.NoArgs` and inherit `--dry-run`

## 3. Node Project Detection

- [ ] 3.1 Implement `detectNodeProjects()`: scan CWD, immediate subdirectories, and common dev directories (`$HOME/Code`, `$HOME/projects`, `$HOME/src`, `$HOME/dev`) for `package.json` files
- [ ] 3.2 Parse `package.json` for project name, `main` field, and `scripts.start` field
- [ ] 3.3 Implement entrypoint detection: check `main`, `scripts.start`, then common filenames (`index.js`, `server.js`, `app.js`, `main.js`)
- [ ] 3.4 Present detected projects as a selection menu
- [ ] 3.5 Add unit tests for project detection and entrypoint resolution

## 4. Node Process Detection

- [ ] 4.1 Implement `detectNodeProcesses()`: use `ps ax` to find running `node` processes, parse PID, script path, and working directory
- [ ] 4.2 Implement `matchNodeProcessesToProjects()`: correlate running processes to detected project directories
- [ ] 4.3 Add unit tests for process detection and correlation

## 5. Package Installation & Instrumented Launch

- [ ] 5.1 Implement package manager detection: `package-lock.json` → npm, `yarn.lock` → yarn, `pnpm-lock.yaml` → pnpm, fallback → npm
- [ ] 5.2 Install OTel packages: `@opentelemetry/sdk-node`, `@opentelemetry/auto-instrumentations-node`, `@opentelemetry/exporter-trace-otlp-http`, `@opentelemetry/exporter-metrics-otlp-http`, `@opentelemetry/exporter-logs-otlp-http`
- [ ] 5.3 Stop existing running process for the selected project (if any)
- [ ] 5.4 Launch with `node --require @opentelemetry/auto-instrumentations-node/register <entrypoint>` and OTEL_* env vars as background detached process
- [ ] 5.5 Show preview before execution: install command, launch command, env vars → prompt `Apply? [Y/n]`
- [ ] 5.6 After launch: detect listening ports (skip 4317/4318), verify service appears in Dynatrace via DQL query
- [ ] 5.7 In dry-run mode: print planned commands without executing

## 6. Uninstall Implementation

- [ ] 6.1 Create `pkg/installer/otel_node_uninstall.go` with `UninstallOtelNode(dryRun bool) error`
- [ ] 6.2 Find running Node processes with `@opentelemetry/auto-instrumentations-node` in their command
- [ ] 6.3 Resolve project directory from the process, detect package manager, run uninstall for OTel packages
- [ ] 6.4 Show preview and prompt, support dry-run

## 7. Testing & Validation

- [ ] 7.1 Add unit tests for package manager detection
- [ ] 7.2 Manual validation: run a sample Node.js app (Express), install otel-node, verify traces in Dynatrace, then uninstall
