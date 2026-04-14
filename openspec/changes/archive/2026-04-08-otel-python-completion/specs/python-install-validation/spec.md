# Spec: Python Install Validation

## ADDED Requirements

### Requirement: Pre-flight validation for Python installer

The `InstallOtelPython()` function SHALL validate prerequisites before proceeding with installation.

#### Scenario: Python 3 not in PATH

- **WHEN** neither `python3` nor `python` is found in PATH (or both resolve to Python 2)
- **THEN** the installer SHALL exit with a clear error message indicating Python 3 is required
- **NOTE** `python3` is tried first; if absent, `python` is accepted provided it reports a Python 3.x version

#### Scenario: pip not available

- **WHEN** a Python 3 interpreter is found but the `pip` module is unavailable (`<python> -m pip` fails)
- **THEN** the installer SHALL exit with a clear error message indicating pip is required

#### Scenario: pip script has a broken shebang

- **WHEN** a virtualenv exists and `bin/pip` is present but its shebang points to a Python interpreter that no longer exists at that path (common after Python upgrades or on macOS with Homebrew)
- **THEN** the installer SHALL invoke pip via the virtualenv's Python binary (`python -m pip`) rather than executing the pip script directly, so a broken shebang never causes a failure

#### Scenario: virtualenv was created on a different machine or environment

- **WHEN** a virtualenv exists in the project but its Python binary fails to execute (e.g. the interpreter path from the original machine no longer exists on the current machine)
- **THEN** the installer SHALL detect the stale venv and prompt the user before deleting and recreating it using the current machine's Python 3 interpreter
- **AND** the plan preview SHALL distinguish a recreate from a fresh create so the user understands what happened
- **SO THAT** the setup flow works regardless of where the venv was originally created

#### Scenario: venv module not available

- **WHEN** `<detected-python> -m venv` is not functional (e.g., missing `python3-venv` package on Debian/Ubuntu)
- **THEN** the installer SHALL exit with a clear error message and suggest installing the `python3-venv` package

#### Scenario: opentelemetry-instrument script has a broken shebang

- **WHEN** `opentelemetry-instrument` is present in the virtualenv's `bin/` directory but its shebang points to a Python interpreter that no longer exists at that path
- **THEN** the installer SHALL invoke it via the virtualenv's Python binary rather than executing the script directly, so a broken shebang never causes a failure

#### Scenario: pip/bootstrap command fails

- **WHEN** any pip or opentelemetry-bootstrap invocation fails
- **THEN** the error message SHALL include:
  - The human-readable failure description (e.g. `pip install -r requirements.txt failed`)
  - The exact command that was executed (e.g. `command: /project/.venv/bin/python -m pip install -r requirements.txt`)
  - The combined stdout+stderr output from the failed command
- **SO THAT** the user can reproduce or diagnose the failure without re-running dtwiz with debug flags

#### Scenario: All prerequisites met

- **WHEN** a Python 3 interpreter (`python3` or `python`), `pip`, and `venv` are all available
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

---

### Requirement: Framework instrumentation verification after bootstrap

`opentelemetry-bootstrap -a install` SHALL be verified after execution. If it exits 0 but installs no framework instrumentation packages, dtwiz SHALL install them directly.

#### Scenario: Bootstrap exits 0 but installs nothing

- **GIVEN** `opentelemetry-bootstrap -a install` exits with code 0
- **WHEN** no `opentelemetry-instrumentation-<framework>` package is present in the venv
- **THEN** dtwiz SHALL:
  - Call bootstrap's internal detection API (`_find_installed_libraries`) via a Python snippet to get the list of needed packages, bypassing the broken CLI entry point
  - If the API call fails (import error, API change), print a non-fatal warning with the manual `opentelemetry-bootstrap -a install` command and continue
  - Install the missing packages directly via pip
  - Print a message listing each package being installed
- **AND** after installation, dtwiz SHALL verify again and report any packages that are still missing with the exact `pip install` command for manual resolution
- **SO THAT** the user always knows what happened and can take action if automatic installation fails

#### Scenario: Bootstrap's internal API is unavailable

- **GIVEN** the `_find_installed_libraries` function cannot be imported (e.g. API changed in a future opentelemetry-instrumentation release)
- **WHEN** boostrap verification runs the Python snippet
- **THEN** the snippet SHALL exit with a non-zero code
- **AND** dtwiz SHALL print a non-fatal warning with the manual `opentelemetry-bootstrap -a install` command so the user can install framework instrumentations themselves

#### Scenario: Bootstrap installs framework instrumentations correctly

- **GIVEN** `opentelemetry-bootstrap -a install` exits with code 0
- **WHEN** at least one `opentelemetry-instrumentation-<framework>` package is present in the venv
- **THEN** the verification step SHALL return without installing additional packages

#### Scenario: Project uses Flask

- **GIVEN** the project's venv has `flask` installed
- **WHEN** bootstrap fails to install `opentelemetry-instrumentation-flask`
- **THEN** the fallback SHALL install `opentelemetry-instrumentation-flask` directly
- **AND** after restarting with `opentelemetry-instrument`, HTTP spans SHALL be generated for Flask routes
- **AND** Dynatrace SHALL create a SERVICE entity for the application

#### Scenario: Fallback installation partially fails

- **GIVEN** dtwiz detected missing instrumentation packages and attempted to install them
- **WHEN** some packages fail to install (network error, version conflict, etc.)
- **THEN** dtwiz SHALL print a warning listing each package that is still missing
- **AND** SHALL print the exact `pip install` command so the user can resolve it manually
- **SO THAT** the user is never left without a clear path to a working setup

#### Scenario: Package name normalization

- **WHEN** comparing installed package names against bootstrap's detected requirements
- **THEN** names SHALL be PEP 503-normalized: lowercased, with underscores and dots replaced by hyphens
- **SO THAT** `psycopg2-binary`, `psycopg2_binary`, and `Psycopg2-Binary` are all treated as the same package
