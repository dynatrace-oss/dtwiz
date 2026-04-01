# Spec: Python Install Validation

## ADDED Requirements

### Requirement: Pre-flight validation for Python installer

The `InstallOtelPython()` function SHALL validate prerequisites before proceeding with installation.

#### Scenario: Python 3 not in PATH

- **WHEN** `python3` is not found in PATH
- **THEN** the installer SHALL exit with a clear error message indicating Python 3 is required

#### Scenario: pip not available

- **WHEN** `python3` is found but the `pip` module is unavailable (`python3 -m pip` fails)
- **THEN** the installer SHALL exit with a clear error message indicating pip is required

#### Scenario: pip script has a broken shebang

- **WHEN** a virtualenv exists and `bin/pip` is present but its shebang points to a Python interpreter that no longer exists at that path (common after Python upgrades or on macOS with Homebrew)
- **THEN** executing `bin/pip` fails at the OS level with `fork/exec … pip: no such file or directory`
- **THEREFORE** `detectProjectPip()` SHALL invoke pip via the virtualenv's Python binary (`python -m pip`) rather than executing the pip script directly, making the shebang irrelevant
- **AND** `detectProjectPip()` SHALL locate the virtualenv's Python binary (`bin/python` or `bin/python3` on Unix; `Scripts/python.exe` or `Scripts/python3.exe` on Windows) and return a `pipCommand{name: pythonPath, args: ["-m", "pip"]}`

#### Scenario: virtualenv was created on a different machine or environment

- **WHEN** a `.venv` (or `venv`, `env`, `.env`) directory exists in the project but its Python binary fails to execute (e.g. the symlink target or the concrete interpreter path from the original machine no longer exists on the current machine)
- **THEN** `isVenvHealthy()` SHALL return false by running `venvPython --version` as a smoke-test
- **AND** the plan SHALL set `NeedsVenv = true`, causing `Execute()` to delete the stale venv directory and recreate it using the current machine's `python3`
- **AND** the plan preview SHALL inform the user: *"Recreate virtualenv (.venv) — existing venv is from a different environment"*
- **SO THAT** the setup flow is fully environment-agnostic and works regardless of where the venv was originally created

#### Scenario: venv module not available

- **WHEN** `python3 -m venv` is not functional (e.g., missing `python3-venv` package on Debian/Ubuntu)
- **THEN** the installer SHALL exit with a clear error message and suggest installing the `python3-venv` package

#### Scenario: opentelemetry-instrument script has a broken shebang

- **WHEN** `opentelemetry-instrument` is present in the virtualenv's `bin/` directory but its shebang points to a Python interpreter that no longer exists at that path
- **THEN** launching it directly fails with `fork/exec … opentelemetry-instrument: no such file or directory`
- **THEREFORE** the installer SHALL invoke it as `venvPython /path/to/opentelemetry-instrument pythonBin entrypoint` — passing the script as a positional argument to Python — so Python interprets it directly and the shebang is treated as a comment

#### Scenario: pip/bootstrap command fails

- **WHEN** any pip or opentelemetry-bootstrap invocation fails
- **THEN** the error message SHALL include:
  - The human-readable failure description (e.g. `pip install -r requirements.txt failed`)
  - The exact command that was executed (e.g. `command: /project/.venv/bin/python -m pip install -r requirements.txt`)
  - The combined stdout+stderr output from the failed command
- **SO THAT** the user can reproduce or diagnose the failure without re-running dtwiz with debug flags

#### Scenario: All prerequisites met

- **WHEN** `python3`, `pip`, and `venv` are all available
- **THEN** the installer SHALL proceed with the normal installation flow

---

### Requirement: Process crash visibility

When the installer launches instrumented processes, the user SHALL receive explicit feedback if a process exits early rather than silently seeing a missing URL.

#### Scenario: Process crash status is queried more than once

- **GIVEN** a process has crashed and its exit status has been read once (e.g. to print the summary line)
- **WHEN** the exit status is queried a second time (e.g. to build the alive-process list)
- **THEN** the second query SHALL return the same `(exited=true, err)` result as the first
- **AND** SHALL NOT incorrectly report the process as still running
- **NOTE** a Go channel value is consumed on receive; the implementation MUST cache the drained value on the struct to make `WaitResult()` idempotent

#### Scenario: Process crashes within the startup settle window

- **GIVEN** one or more processes have been started with `opentelemetry-instrument`
- **WHEN** a process exits with a non-zero exit code before the settle period ends
- **THEN** the summary line SHALL show `[crashed: <exit error> — check log for details]` and the log filename
- **AND** the URL SHALL NOT be shown for that process
- **AND** if ALL processes have crashed or exited, the installer SHALL print `No services are running — check the logs above for errors.` and SHALL NOT print the traffic-waiting prompt

#### Scenario: Process exits cleanly within the startup settle window

- **GIVEN** a process was started successfully
- **WHEN** it exits with exit code 0 before the settle period ends (e.g. a one-shot script)
- **THEN** the summary line SHALL show `[exited cleanly]` and the log filename

#### Scenario: Process is running but has not bound a port

- **GIVEN** a process is still alive after the settle period
- **WHEN** no listening TCP port is detected for its PID
- **THEN** the summary line SHALL show `[running, port not detected]` and the log filename

#### Scenario: Process is running and has bound a port

- **GIVEN** a process is still alive after the settle period
- **WHEN** a listening TCP port is detected for its PID
- **THEN** the summary line SHALL show `→ http://localhost:<port>` and the log filename
