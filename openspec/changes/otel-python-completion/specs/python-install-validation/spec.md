## ADDED Requirements

### Requirement: Pre-flight validation for Python installer
The `InstallOtelPython()` function SHALL validate prerequisites before proceeding with installation.

#### Scenario: Python 3 not in PATH
- **WHEN** `python3` is not found in PATH
- **THEN** the installer SHALL exit with a clear error message indicating Python 3 is required

#### Scenario: pip not available
- **WHEN** `python3` is found but `pip3` and `pip` are both unavailable
- **THEN** the installer SHALL exit with a clear error message indicating pip is required

#### Scenario: venv module not available
- **WHEN** `python3 -m venv` is not functional (e.g., missing `python3-venv` package on Debian/Ubuntu)
- **THEN** the installer SHALL exit with a clear error message and suggest installing the `python3-venv` package

#### Scenario: All prerequisites met
- **WHEN** `python3`, `pip`, and `venv` are all available
- **THEN** the installer SHALL proceed with the normal installation flow
