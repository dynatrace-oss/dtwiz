# Spec: python install validation

## MODIFIED Requirements

### Requirement: Pre-flight validation for Python installer

The `InstallOtelPython()` function SHALL validate prerequisites before proceeding with installation. If `python3` is not found on PATH, dtwiz SHALL install Python using the platform package manager rather than exiting with an error.

#### Scenario: Python 3 not in PATH — macOS with Homebrew

- **WHEN** neither `python3` nor `python` is found in PATH on macOS
- **AND** `brew` is available
- **THEN** the installer SHALL include `brew install python3` in the plan preview
- **AND** execute it after user confirmation (or immediately if `--yes` is set)

#### Scenario: Python 3 not in PATH — macOS without Homebrew

- **WHEN** neither `python3` nor `python` is found in PATH on macOS
- **AND** `brew` is not available
- **THEN** the installer SHALL exit with: `Python 3 is required but not found. Install Homebrew first: https://brew.sh`

#### Scenario: Python 3 not in PATH — Debian/Ubuntu Linux

- **WHEN** neither `python3` nor `python` is found in PATH
- **AND** `/etc/os-release` indicates a Debian/Ubuntu-based distro
- **THEN** the installer SHALL run `sudo apt-get install -y python3`

#### Scenario: Python 3 not in PATH — RHEL/Fedora/CentOS Linux

- **WHEN** neither `python3` nor `python` is found in PATH
- **AND** `/etc/os-release` indicates a RHEL/Fedora/CentOS-based distro
- **THEN** the installer SHALL run `sudo dnf install -y python3`

#### Scenario: Python 3 not in PATH — Windows

- **WHEN** neither `python3` nor `python` is found in PATH on Windows
- **THEN** the installer SHALL run `winget install Python.Python.3`

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
