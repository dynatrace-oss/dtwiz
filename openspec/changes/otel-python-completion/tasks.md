# Tasks

## 1. Pre-flight Validation

- [ ] 1.1 Add `validatePythonPrerequisites()` in `pkg/installer/otel_python.go`: check Python 3 in PATH (reuse/extend existing `detectPython()`), check `python3 -m pip --version` (not `exec.LookPath("pip")` — the pip executable in PATH is irrelevant; what matters is whether pip is installed as a Python module), check `python3 -m venv --help`
- [ ] 1.2 Return descriptive `fmt.Errorf` on any failure; for missing venv on Debian/Ubuntu suggest `apt install python3-venv`. No `os.Exit` — errors bubble to `cmd/install.go`
- [ ] 1.3 Call `validatePythonPrerequisites()` at the top of `InstallOtelPython()` in `pkg/installer/otel_python.go`, before any project detection
- [ ] 1.4 Tests in `pkg/installer/installer_test.go`:
  - `TestValidatePythonPrerequisites_PythonNotFound`: mock PATH with no python3/python → expect error containing "Python 3"
  - `TestValidatePythonPrerequisites_PipNotFound`: python3 present, `python3 -m pip` fails → expect error containing "pip"
  - `TestValidatePythonPrerequisites_VenvNotFound`: python3+pip present, `python3 -m venv` fails → expect error containing "venv" and install suggestion
  - `TestValidatePythonPrerequisites_AllPresent`: all checks pass → no error

## 2. Command Logging in Error Messages

- [ ] 2.1 In `installPackages()` (`pkg/installer/otel_python.go`): build `full` string from binary + args, append `command: <full>` to error
- [ ] 2.2 In `runOtelBootstrap()` (`pkg/installer/otel_python.go`): same pattern
- [ ] 2.3 In `installProjectDeps()` (`pkg/installer/otel_python.go`): unify the print and error to use the same `full` variable
- [ ] 2.4 Tests in `pkg/installer/installer_test.go`:
  - `TestInstallPackages_ErrorIncludesCommand`: invoke `installPackages()` with a non-existent binary → error string must contain `"command:"`
  - `TestRunOtelBootstrap_ErrorIncludesCommand`: same for `runOtelBootstrap()`
  - `TestInstallProjectDeps_ErrorIncludesCommand`: create a temp dir with a `requirements.txt`, invoke with a broken pip → error must contain `"command:"`

## 3. Shebang-safe Script Execution

- [ ] 3.1 `detectProjectPip()` in `pkg/installer/otel_python.go`: locate venv Python binary (`bin/python`, `bin/python3`; `Scripts/python.exe` on Windows), return it with `-m pip` args — never reference the pip script
- [ ] 3.2 `Execute()` in `pkg/installer/otel_python.go`: launch `opentelemetry-instrument` as `venvPython <scriptPath> <pythonBin> <entrypoint>` — never exec the script directly
- [ ] 3.3 Tests in `pkg/installer/installer_test.go`:
  - `TestDetectProjectPip_ReturnsPythonMPip`: create a temp dir with a `.venv/bin/python` stub → verify returned `pipCommand.name` is the Python binary and `pipCommand.args` starts with `["-m", "pip"]`
  - `TestDetectProjectPip_NoPipScriptFallback`: venv has only `bin/python3`, not `bin/python` → verify `pip3` script is NOT returned; Python binary with `-m pip` is

## 4. Environment-agnostic Venv Handling

- [ ] 4.1 Add `isVenvHealthy()` in `pkg/installer/otel_python.go`: locate venv Python via `detectProjectPip()`, run a version probe, return false on any error
- [ ] 4.2 `DetectPythonPlan()` in `pkg/installer/otel_python.go`: set `NeedsVenv = !isVenvHealthy(proj.Path)`
- [ ] 4.3 `Execute()` in `pkg/installer/otel_python.go`: when `NeedsVenv` is true and a venv directory exists, remove it before recreating
- [ ] 4.4 `PrintPlanSteps()` in `pkg/installer/otel_python.go`: print "Recreate virtualenv" when stale, "Create virtualenv" when absent
- [ ] 4.5 Tests in `pkg/installer/installer_test.go`:
  - `TestIsVenvHealthy_NoVenv`: no venv directory → returns false
  - `TestIsVenvHealthy_BrokenPython`: venv directory exists but Python binary is absent or non-executable → returns false
  - `TestIsVenvHealthy_WorkingPython`: venv directory with a working Python binary → returns true
- [ ] 4.6 Manual validation: copy a project with a venv from another machine (or delete the venv Python binary) and confirm dtwiz detects, removes, and recreates it cleanly

## 5. Process Lifecycle Tracking

- [ ] 5.1 Create `pkg/installer/otel_process.go` with `ManagedProcess` struct (including `settled bool`, `exited bool`, `exitVal error` cache fields), `StartManagedProcess()`, `WaitResult()`, `PrintSummaryLine()`, `PrintProcessSummary()`, and `detectListeningPort()`
- [ ] 5.2 `WaitResult()` MUST cache the channel result on first read — drain the channel once, store on struct, return cached value on all subsequent calls; a non-cached implementation causes the second call to incorrectly report the process as alive
- [ ] 5.3 `Execute()` in `pkg/installer/otel_python.go`: replace inline launch loop with `StartManagedProcess()` calls; replace summary loop with `PrintProcessSummary()`; remove `detectListeningPort()` from `otel_python.go`
- [ ] 5.4 `Execute()` in `pkg/installer/otel_python.go`: after `PrintProcessSummary`, only call `waitForServices` if the returned alive-names list is non-empty; otherwise print `No services are running — check the logs above for errors.` and return
- [ ] 5.5 Tests in `pkg/installer/otel_process_test.go`:
  - `TestWaitResult_Idempotent`: start a process that exits immediately, call `WaitResult()` twice → both calls must return `(true, <err>)`; second call must NOT return `(false, nil)`
  - `TestWaitResult_StillRunning`: start a long-running process, call `WaitResult()` immediately → returns `(false, nil)`
  - `TestPrintProcessSummary_AllCrashed_NoAliveNames`: all processes crash → `aliveNames` is empty
  - `TestPrintProcessSummary_SomeCrashed_OnlyAliveReturned`: mixed crashed/alive → only alive names returned
  - `TestPrintProcessSummary_CrashedNonZeroExit_SummaryLabel`: crashed process → output contains `[crashed:`
  - `TestPrintProcessSummary_CleanExit_SummaryLabel`: zero-exit process → output contains `[exited cleanly]`
- [ ] 5.6 Do NOT use `syscall` — use `cmd.Wait()` via goroutine for cross-platform exit detection

## 6. End-to-end Validation

- [ ] 6.1 Unit tests for `validatePythonPrerequisites()` covering each missing-prerequisite scenario from the spec (consolidates 1.4)
- [ ] 6.2 Manual end-to-end: run `dtwiz install otel-python` on a machine where the project venv was created on a different machine — confirm clean recreation and successful instrumentation
- [ ] 6.3 Manual end-to-end: run `dtwiz install otel-python` where the app crashes on startup — confirm `No services are running` is shown and `Waiting for traffic` is NOT shown
