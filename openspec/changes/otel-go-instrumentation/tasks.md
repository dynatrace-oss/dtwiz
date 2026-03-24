## 1. Pre-flight Validation

- [ ] 1.1 Implement `validateGoPrerequisites()` in a new `otel_go.go`: check `go` in PATH, verify `go.mod` exists in selected project
- [ ] 1.2 Call validation at the start of `InstallOtelGo()` — exit with clear error if checks fail
- [ ] 1.3 Add unit tests for the validation function

## 2. CLI Command Registration

- [ ] 2.1 Add `otel-go` subcommand to `installCmd` in `cmd/install.go`, wiring to `InstallOtelGo()`
- [ ] 2.2 Add `otel-go` subcommand to `uninstallCmd` in `cmd/uninstall.go`, wiring to `UninstallOtelGo()`
- [ ] 2.3 Ensure both subcommands set `Args: cobra.NoArgs` and inherit `--dry-run`

## 3. Go Project Detection

- [ ] 3.1 Implement `detectGoProjects()`: scan CWD, immediate subdirectories, and common dev directories for `go.mod` files
- [ ] 3.2 Parse `go.mod` for module name
- [ ] 3.3 Present detected projects as a selection menu
- [ ] 3.4 Add unit tests for project detection and module name parsing

## 4. Dependency Injection

- [ ] 4.1 Implement `addOtelGoDependencies()`: run `go get` for core OTel packages (`go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`, `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`, `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp`)
- [ ] 4.2 Check if OTel packages are already in `go.mod` — if so, inform user and offer to update to latest
- [ ] 4.3 Show preview: list `go get` commands, prompt `Apply? [Y/n]`
- [ ] 4.4 In dry-run mode: print commands without executing

## 5. Bootstrap Snippet Generation

- [ ] 5.1 Create a Go template (embedded or inline) that generates an OTel initialization snippet with TracerProvider, MeterProvider, LoggerProvider, and OTLP HTTP exporters
- [ ] 5.2 Print the snippet to stdout with clear placement instructions: "Add this to your main() function, before your application logic"
- [ ] 5.3 Print the required OTEL_* environment variables with values populated for the user's Dynatrace environment
- [ ] 5.4 Include import statements needed for the snippet

## 6. Uninstall Implementation

- [ ] 6.1 Create `pkg/installer/otel_go_uninstall.go` with `UninstallOtelGo(dryRun bool) error`
- [ ] 6.2 Detect Go project (prompt user to select if multiple), scan `go.mod` for `go.opentelemetry.io` packages
- [ ] 6.3 Remove packages via `go mod edit -droprequire` for each OTel package, then run `go mod tidy`
- [ ] 6.4 Show preview and prompt, support dry-run

## 7. Testing & Validation

- [ ] 7.1 Add unit tests for `go.mod` parsing and OTel dependency detection
- [ ] 7.2 Add unit tests for snippet generation
- [ ] 7.3 Manual validation: add OTel deps to a sample Go project, verify snippet compiles, then uninstall
