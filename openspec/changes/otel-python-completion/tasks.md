## 1. Pre-flight Validation

- [ ] 1.1 Add a `validatePythonPrerequisites()` function in `otel_python.go` that checks: `python3` in PATH, `pip3`/`pip` available, `venv` module functional
- [ ] 1.2 Call `validatePythonPrerequisites()` at the start of `InstallOtelPython()` — exit with clear error messages if any check fails
- [ ] 1.3 Add unit tests for the validation function with various missing-prerequisite scenarios

## 2. Uninstall Command Registration

- [ ] 2.1 Add `otel-python` subcommand to `uninstallCmd` in `cmd/uninstall.go`, wiring to a new `UninstallOtelPython()` function
- [ ] 2.2 Ensure the subcommand inherits the `--dry-run` persistent flag from the parent `uninstall` command

## 3. Uninstall Implementation

- [ ] 3.1 Create `pkg/installer/otel_python_uninstall.go` with `UninstallOtelPython(dryRun bool) error`
- [ ] 3.2 Implement process detection: find running Python processes that have `opentelemetry-instrument` in their command line (using `ps ax` parsing, similar to existing `detectPythonProcesses()`)
- [ ] 3.3 For each detected process, resolve the virtualenv path from the Python binary path in the process command
- [ ] 3.4 Show preview: list processes to stop and packages to uninstall, with the target virtualenv paths
- [ ] 3.5 Prompt `Apply? [Y/n]` — on confirm: stop processes (SIGINT), then run `pip uninstall -y opentelemetry-distro opentelemetry-exporter-otlp` in each virtualenv
- [ ] 3.6 In dry-run mode, print the preview without executing

## 4. Testing

- [ ] 4.1 Add unit tests for instrumented-process detection logic
- [ ] 4.2 Add unit tests for virtualenv path resolution from process command
- [ ] 4.3 Manual validation: install otel-python on a sample app, verify uninstall stops the process and removes packages
