# Spec: python-prerequisite-validation

## Requirements

### Requirement: Probe pip availability inside a fresh virtualenv

`validatePythonPrerequisites()` SHALL verify that pip is functional inside a newly created virtualenv, not just available globally. On Debian/Ubuntu, `python3-venv` is a separate package; without it, `python -m venv --help` succeeds and a venv directory is created, but the venv omits pip entirely. A global pip check does not catch this.

#### Scenario: pip missing from fresh venv

- **WHEN** `python -m pip` succeeds globally
- **AND** `python -m venv --help` succeeds
- **AND** a probe virtualenv created by the detected interpreter does not contain a working pip binary
- **THEN** `validatePythonPrerequisites()` SHALL return an error: `pip is not available in new virtualenvs for <interpreter> — on Debian/Ubuntu run: apt install python3-venv`

#### Scenario: probe venv creation fails

- **WHEN** `python -m venv <tmpdir>` itself fails during the probe
- **THEN** the probe SHALL return false (prerequisites not satisfied)

#### Scenario: probe temp dir cannot be created

- **WHEN** a temporary directory cannot be created for the probe
- **THEN** the probe SHALL return true (assume satisfied) and let downstream commands surface any real failure

#### Scenario: all prerequisites satisfied

- **WHEN** the interpreter is found, pip works globally, venv module is functional, and a probe virtualenv contains pip
- **THEN** `validatePythonPrerequisites()` SHALL return the resolved interpreter path with no error
