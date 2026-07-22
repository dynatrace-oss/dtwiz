# Why

`dtwiz install otel-python` fails with cryptic interpreter or pip errors when Python prerequisites are absent or misconfigured. On Debian/Ubuntu, `python3-venv` is a separate package; without it, venv creation appears to succeed but the resulting venv omits pip — global pip checks do not catch this. Virtualenvs created on another machine or after a Python upgrade can have broken shebangs that cause silent failures deep in the install flow. The demo installer (`dtwiz install demo`) also has no way to install Python automatically, so fresh machines abort before any observability is set up.

## What Changes

- Add structured pre-flight validation (`validatePythonPrerequisites`) before the OTel Python installer runs: detect the interpreter, verify pip and venv module availability, and probe pip inside a fresh temporary virtualenv.
- Detect stale virtualenvs (broken interpreter path) and prompt the user to remove and recreate them before proceeding.
- Invoke pip via `python -m pip` through the venv's Python binary rather than the pip shebang script, to survive broken shebangs from interpreter upgrades.
- Add automatic Python prerequisite installation to the demo flow: select the right package manager command per platform (brew, apt-get, dnf, winget) and include it in the plan preview.

## Capabilities

### New Capabilities

- `python-prerequisite-validation`: Pre-flight check that verifies Python 3, pip, venv module, and pip-in-fresh-venv are all functional before OTel Python installation begins.
- `demo-python-prerequisites`: Demo installer detects missing Python prerequisites and installs them automatically via the platform package manager.

### Modified Capabilities

- None.

## Impact

- **Code**: `pkg/installer/otel/python_venv.go`, `pkg/installer/otel/demo.go`.
- **UX**: Users on Debian/Ubuntu without `python3-venv`, or with a stale virtualenv, receive a clear actionable error or a prompt to recreate instead of a mid-install failure. The demo plan preview lists any Python install step before asking for confirmation.
- **Dependencies**: No new dependencies.
