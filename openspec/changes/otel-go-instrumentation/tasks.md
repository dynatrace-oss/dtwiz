<!-- Parallelism: Groups 1+2 (scaffold) can be done first. Then groups 3, 4, 5 are independent of each other. Group 6 is independent of 4+5. -->

## 1. Pre-flight Validation

- [ ] 1.1 Implement `validateGoPrerequisites(projectPath string) error` in a new `otel_go.go`: use `exec.LookPath("go")` to check Go in PATH, verify `os.Stat(filepath.Join(projectPath, "go.mod"))` exists
- [ ] 1.2 Return `fmt.Errorf` with clear user-facing messages on failure (no `os.Exit` — errors bubble to cmd layer)
- [ ] 1.3 Add unit tests for the validation function

## 2. CLI Command Registration

- [ ] 2.1 Add `otel-go` subcommand to `installCmd` in `cmd/install.go` — follow existing `otel-python` pattern: `Use: "otel-go"`, `Args: cobra.NoArgs`, `RunE` calls `getDtEnvironment()` + `validateCredentials()` + `installer.InstallOtelGo(envURL, accessTok, platformTok, serviceName, installDryRun)`. Note: `platformToken` is needed for DQL verification. `serviceName` from `--service-name` flag (default empty, defaulted inside function)
- [ ] 2.2 Add `otel-go` subcommand to `uninstallCmd` in `cmd/uninstall.go` — `Args: cobra.NoArgs`, `RunE` calls `installer.UninstallOtelGo(uninstallDryRun)`
- [ ] 2.3 `--dry-run` is already a `PersistentFlag` on both parent commands — no additional flag setup needed

## 3. Go Project Detection (independent of groups 4+5)

- [ ] 3.1 Define `GoProject` struct: `Path string`, `ModuleName string` — simpler than Python/Node since Go has no running-process correlation for auto-instrumentation
- [ ] 3.2 Implement `detectGoProjects() []GoProject`: scan CWD + immediate subdirectories, then `$HOME/Code`, `$HOME/code`, `$HOME/projects`, `$HOME/src`, `$HOME/dev` (two levels deep) for `go.mod` files. Deduplicate via symlink resolution (same pattern as `detectPythonProjects()`)
- [ ] 3.3 Parse `go.mod` for module name: read first line matching `^module\s+(.+)$` using `bufio.Scanner`
- [ ] 3.4 Present detected projects as a numbered selection menu using `bufio.Scanner` (same pattern as Python project selection)
- [ ] 3.5 Add unit tests for project detection and module name parsing

## 4. Dependency Injection (independent of groups 3+5)

- [ ] 4.1 Implement `addOtelGoDependencies(projectPath string) error`: run `exec.Command("go", "get", pkg)` with `cmd.Dir = projectPath` for each core OTel package: `go.opentelemetry.io/otel@latest`, `go.opentelemetry.io/otel/sdk@latest`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest`, `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@latest`, `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp@latest`
- [ ] 4.2 Check if OTel packages are already in `go.mod` by reading the file and searching for `go.opentelemetry.io` — if present, inform user and offer to update to latest via `go get -u`
- [ ] 4.3 Show preview: list `go get` commands, prompt via `confirmProceed("Apply?")`
- [ ] 4.4 In dry-run mode: print `[dry-run]` prefix + commands, return nil early

## 5. Bootstrap Snippet Generation (independent of groups 3+4)

- [ ] 5.1 Create an embedded Go template (`//go:embed otel_go_bootstrap.tmpl` or inline string) that generates an OTel initialization snippet with: OTLP HTTP trace exporter, OTLP HTTP metric exporter (with delta temporality), OTLP HTTP log exporter, TracerProvider/MeterProvider/LoggerProvider setup, and `defer shutdown(ctx)` pattern
- [ ] 5.2 Implement `generateOtelGoEnvVars(apiURL, token, serviceName string) map[string]string` — same env vars as other languages: `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`
- [ ] 5.3 Print the snippet to stdout with clear placement instructions: "Add this to your main() function, before your application logic starts"
- [ ] 5.4 Print required import statements as a separate block the user can merge into their existing imports
- [ ] 5.5 Print OTEL_* env vars using `GenerateEnvExportScript()` (reuse from `otel_python.go`)

## 6. Uninstall Implementation (independent of groups 4+5)

- [ ] 6.1 Create `pkg/installer/otel_go_uninstall.go` with `UninstallOtelGo(dryRun bool) error`
- [ ] 6.2 Detect Go project: reuse `detectGoProjects()` for project selection, then scan `go.mod` for lines containing `go.opentelemetry.io` to find OTel packages
- [ ] 6.3 Remove packages: for each found OTel module, run `exec.Command("go", "mod", "edit", "-droprequire", modulePath)` with `cmd.Dir = projectPath`, then run `exec.Command("go", "mod", "tidy")` with `cmd.Dir = projectPath`
- [ ] 6.4 Show preview using separator/color pattern, prompt via `confirmProceed()`, support dry-run (print `[dry-run]` + preview and return)

## 7. Testing & Validation

- [ ] 7.1 Add unit tests for `go.mod` parsing (module name extraction and OTel dependency detection)
- [ ] 7.2 Add unit tests for snippet generation (verify template renders valid Go code)
- [ ] 7.3 Manual validation: add OTel deps to a sample Go project, verify snippet compiles, verify env vars are correct for the target Dynatrace environment, then uninstall
