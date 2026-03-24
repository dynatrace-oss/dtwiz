<!-- Parallelism: Groups 1 and 2+3 are independent — validation and uninstall can be developed in parallel. -->

## 1. Pre-flight Validation

- [ ] 1.1 Add a `validatePythonPrerequisites()` function in `otel_python.go` that checks: `exec.LookPath("python3")` (or fall back to `"python"` + version check like existing `detectPython()`), `exec.LookPath("pip3")` or `exec.LookPath("pip")`, and `exec.Command(python, "-m", "venv", "--help").Run()` for venv module
- [ ] 1.2 Return `fmt.Errorf` with clear user-facing messages on failure (no `os.Exit` — errors bubble to cmd layer). For venv: suggest `apt install python3-venv` on Debian/Ubuntu
- [ ] 1.3 Call `validatePythonPrerequisites()` at the start of `InstallOtelPython()` — return error before any detection work
- [ ] 1.4 Add unit tests for the validation function with various missing-prerequisite scenarios

## 2. Uninstall Command Registration

- [ ] 2.1 Add `otel-python` subcommand to `uninstallCmd` in `cmd/uninstall.go` — follow existing pattern: `Use: "otel-python"`, `Args: cobra.NoArgs`, `RunE` calls `installer.UninstallOtelPython(uninstallDryRun)`
- [ ] 2.2 `--dry-run` is already a `PersistentFlag` on `uninstallCmd` — no additional flag setup needed, just pass `uninstallDryRun` to the function

## 3. Uninstall Implementation

- [ ] 3.1 Create `pkg/installer/otel_python_uninstall.go` with `UninstallOtelPython(dryRun bool) error`
- [ ] 3.2 Implement process detection: reuse the `ps ax -o pid=,command=` parsing pattern from `detectPythonProcesses()`, filter for processes containing `opentelemetry-instrument` in the command line
- [ ] 3.3 For each detected process, resolve the virtualenv path from the Python binary path in the command (e.g., `/path/to/project/.venv/bin/python` → `/path/to/project/.venv/bin/pip`)
- [ ] 3.4 Show preview: list processes to stop (PID + command) and packages to uninstall (use the existing `otelPythonPackages` var: `opentelemetry-distro`, `opentelemetry-exporter-otlp`), with the target virtualenv paths
- [ ] 3.5 In dry-run mode: print `[dry-run]` prefix + preview, return nil
- [ ] 3.6 Prompt via `confirmProceed("Proceed with uninstall?")` — on confirm: stop processes via `stopProcesses()` (reuse from `otel_python.go`, sends SIGINT), then run `exec.Command(pipPath, "uninstall", "-y", packages...)` in each resolved virtualenv
- [ ] 3.7 Print summary of stopped processes and removed packages

## 4. Testing

- [ ] 4.1 Add unit tests for instrumented-process detection logic (parsing `ps` output for `opentelemetry-instrument` patterns)
- [ ] 4.2 Add unit tests for virtualenv path resolution from process command
- [ ] 4.3 Manual validation: install otel-python on a sample app, verify uninstall stops the process and removes packages
