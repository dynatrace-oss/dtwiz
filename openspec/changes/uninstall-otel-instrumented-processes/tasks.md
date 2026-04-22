## 1. Python Process Detection

- [ ] 1.1 Add `findInstrumentedPythonProcesses()` to `pkg/installer/otel_uninstall.go` — calls `detectProcesses("opentelemetry-instrument", nil)` and returns `[]DetectedProcess`
- [ ] 1.2 Add debug logging inside `findInstrumentedPythonProcesses()` for each process found, logging PID and command with `logger.Debug`

## 2. Extend Preview in `UninstallOtelCollector()`

- [ ] 2.1 Call `findInstrumentedPythonProcesses()` at the top of `UninstallOtelCollector()` alongside the existing collector process detection
- [ ] 2.2 Update the "nothing to remove" early-exit guard to also check `len(pythonProcs) == 0`
- [ ] 2.3 Add a Python section to the preview: print "Instrumented Python processes that will be stopped:" with PID and command for each process, or "No instrumented Python processes found." when empty — matching the style of the existing collector section

## 3. Stop Python Processes After Confirmation

- [ ] 3.1 After the confirmation prompt and the existing `killCollectorProcesses()` call, add a block that calls `stopProcesses(pids)` for all detected Python process PIDs
- [ ] 3.2 Ensure the block is skipped entirely (not reached) when `dryRun == true` — the existing dry-run early-return already covers this; verify no extra guard is needed

## 4. Unit Tests

- [ ] 4.1 Add `TestFindInstrumentedPythonProcesses_NoMatch` in `pkg/installer/otel_uninstall_test.go` (or a new `otel_uninstall_python_test.go`) — mock `detectProcesses` returning processes without `opentelemetry-instrument`; assert empty result
- [ ] 4.2 Add `TestFindInstrumentedPythonProcesses_WithMatch` — mock processes where one contains `opentelemetry-instrument`; assert it is returned and others are not
- [ ] 4.3 Add `TestUninstallOtelCollector_PythonProcessesInPreview` — integration-style test that verifies Python PIDs appear in the dry-run output when matching processes exist

## 5. Manual Verification

- [ ] 5.1 Start a Python process instrumented via `opentelemetry-instrument python app.py`; run `dtwiz uninstall otel --dry-run` and confirm the PID appears in the preview
- [ ] 5.2 Run `dtwiz uninstall otel` (without dry-run), confirm, and verify the Python process is no longer running
- [ ] 5.3 Run `dtwiz uninstall otel` with no instrumented Python processes active and verify output is unchanged from current behaviour
- [ ] 5.4 Run `make test` and `make lint` — all checks pass
