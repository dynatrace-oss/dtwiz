# Tasks

## 1. File Layout Extraction

- [x] 1.1 Create `pkg/installer/otel_python_packages.go` and move `pipCommand`, `otelPythonPackages`, `installPackages`, `runOtelBootstrap`, `bootstrapRequirementsScript`, `normalizePipName`, `listInstalledPipPackages`, `queryBootstrapRequirements`, `ensureFrameworkInstrumentations` out of `otel_python.go` into it — same package, no API changes
- [x] 1.2 Remove the moved declarations from `otel_python.go` and adjust imports (keep `encoding/json` — still used by `querySmartscapeServices`)

## 2. Pre-flight Validation

- [x] 2.1 Add `validatePythonPrerequisites()` in `pkg/installer/otel_python.go`: check Python 3 in PATH (reuse/extend existing `detectPython()`), check `python3 -m pip --version`, check `python3 -m venv --help`
- [x] 2.2 Return descriptive `fmt.Errorf` on any failure; for missing venv on Debian/Ubuntu suggest `apt install python3-venv`. No `os.Exit` — errors bubble to `cmd/install.go`
- [x] 2.3 Call `validatePythonPrerequisites()` at the top of `InstallOtelPython()` in `pkg/installer/otel_python.go`, before any project detection
- [x] 2.4 Tests in `pkg/installer/otel_python_test.go`:
  - `TestValidatePythonPrerequisites_PythonNotFound`
  - `TestValidatePythonPrerequisites_PipNotFound`
  - `TestValidatePythonPrerequisites_VenvNotFound`
  - `TestValidatePythonPrerequisites_AllPresent`

## 3. Command Logging in Error Messages

- [x] 3.1 In `installPackages()` (`pkg/installer/otel_python_packages.go`): build `full` string from binary + args, append `command: <full>` to error
- [x] 3.2 In `runOtelBootstrap()` (`pkg/installer/otel_python_packages.go`): same pattern
- [x] 3.3 In `installProjectDeps()` (`pkg/installer/otel_python.go`): unify the print and error to use the same `full` variable
- [x] 3.4 Tests in `pkg/installer/otel_python_test.go`:
  - `TestInstallPackages_ErrorIncludesCommand`
  - `TestRunOtelBootstrap_ErrorIncludesCommand`
  - `TestInstallProjectDeps_ErrorIncludesCommand`

## 4. Shebang-safe Script Execution

- [x] 4.1 `detectProjectPip()` in `pkg/installer/otel_python.go`: locate venv Python binary (`bin/python`, `bin/python3`; `Scripts/python.exe` on Windows), return it with `-m pip` args — never reference the pip script
- [x] 4.2 `Execute()` in `pkg/installer/otel_python.go`: launch `opentelemetry-instrument` as `venvPython <scriptPath> <pythonBin> <entrypoint>` — never exec the script directly
- [x] 4.3 Tests in `pkg/installer/otel_python_test.go`:
  - `TestDetectProjectPip_ReturnsPythonMPip`
  - `TestDetectProjectPip_NoPipScriptFallback`

## 5. Environment-agnostic Venv Handling

- [x] 5.1 Add `isVenvHealthy()` in `pkg/installer/otel_python.go`: locate venv Python via `detectProjectPip()`, run a version probe, return false on any error
- [x] 5.2 `DetectPythonPlan()` in `pkg/installer/otel_python.go`: set `NeedsVenv = !isVenvHealthy(proj.Path)`
- [ ] 5.3 `Execute()` in `pkg/installer/otel_python.go`: when `NeedsVenv` is true and a venv directory exists, ask the user before removing it and explain that a working virtualenv is required for reliable Python package installation and OTLP ingest before recreating it
- [x] 5.4 `PrintPlanSteps()` in `pkg/installer/otel_python.go`: print `Recreate virtualenv` when stale, `Create virtualenv` when absent
- [x] 5.5 Tests in `pkg/installer/otel_python_test.go`:
  - `TestIsVenvHealthy_NoVenv`
  - `TestIsVenvHealthy_BrokenPython`
  - `TestIsVenvHealthy_WorkingPython`
- [ ] 5.6 Add automated or manual validation that stale virtualenv recreation now requires user confirmation before deletion; full external end-to-end recreation remains tracked in 8.2

## 6. Process Lifecycle Tracking

- [x] 6.1 Create `pkg/installer/otel_process.go` with `ManagedProcess` struct (including `settled bool`, `exited bool`, `exitVal error` cache fields), `StartManagedProcess()`, `WaitResult()`, `PrintSummaryLine()`, `PrintProcessSummary()`, and `detectListeningPort()`
- [x] 6.2 `WaitResult()` caches the channel result on first read so subsequent calls return the same state
- [x] 6.3 `Execute()` in `pkg/installer/otel_python.go`: replace inline launch loop with `StartManagedProcess()` calls; replace summary loop with `PrintProcessSummary()`; remove `detectListeningPort()` from `otel_python.go`
- [x] 6.4 `Execute()` in `pkg/installer/otel_python.go`: after `PrintProcessSummary`, only call `waitForServices` if the returned alive-names list is non-empty; otherwise print `No services are running — check the logs above for errors.` and return
- [x] 6.5 Tests in `pkg/installer/otel_process_test.go`:
  - `TestWaitResult_Idempotent`
  - `TestWaitResult_StillRunning`
  - `TestPrintProcessSummary_AllCrashed_NoAliveNames`
  - `TestPrintProcessSummary_SomeCrashed_OnlyAliveReturned`
  - `TestPrintProcessSummary_CrashedNonZeroExit_SummaryLabel`
  - `TestPrintProcessSummary_CleanExit_SummaryLabel`
- [x] 6.6 Do NOT use `syscall` — use `cmd.Wait()` via goroutine for cross-platform exit detection

## 7. Bootstrap Verification and Fallback

- [x] 7.1 Add `bootstrapRequirementsScript` Python snippet in `pkg/installer/otel_python_packages.go`: calls bootstrap's internal `_find_installed_libraries()` API directly, bypassing the broken CLI entry point, and prints one needed package per line. Exits non-zero if the API is unavailable.
- [x] 7.2 Add `normalizePipName(name string) string`: PEP 503 normalization — lowercase, replace `_` and `.` with `-`.
- [x] 7.3 Add `listInstalledPipPackages(pythonBin string) (map[string]bool, error)`: runs `pip list --format=json`, returns normalized set.
- [x] 7.4 Add `queryBootstrapRequirements(pythonBin string, installed map[string]bool) ([]string, error)`: calls bootstrap API via Python snippet; returns an error if the API is unavailable — no hardcoded fallback.
- [x] 7.5 Add `ensureFrameworkInstrumentations(pythonBin string, pip *pipCommand) error`:
  - Returns nil immediately if any framework instrumentation is already installed (bootstrap worked).
  - Otherwise calls `queryBootstrapRequirements()` and pip-installs the results.
  - If the bootstrap API is unavailable, prints a non-fatal warning with the manual command and returns nil.
  - After installation, verifies again and prints a warning listing any remaining missing packages with the exact `pip install` command for manual resolution.
  - On install failure, prints the manual pip command so the user can fix it themselves.
- [x] 7.6 Call `ensureFrameworkInstrumentations(venvPython, venvPip)` in `Execute()` immediately after `runOtelBootstrap()` returns successfully.

## 8. End-to-end Validation

- [x] 8.1 Unit tests for `validatePythonPrerequisites()` covering each missing-prerequisite scenario from the spec
- [x] 8.2 External manual end-to-end: run `dtwiz install otel-python` on a machine where the project venv was created on a different machine — confirm clean recreation and successful instrumentation
- [x] 8.3 External manual end-to-end: run `dtwiz install otel-python` where the app crashes on startup — confirm `No services are running` is shown and `Waiting for traffic` is NOT shown
- [x] 8.4 External manual end-to-end: run `dtwiz install otel-python` against a Flask project — confirm `opentelemetry-instrumentation-flask` is installed even when `opentelemetry-bootstrap` exits 0 without installing it, and that spans appear in Dynatrace after sending requests
