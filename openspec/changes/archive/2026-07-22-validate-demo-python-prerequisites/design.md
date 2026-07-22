# Context

OTel Python installation requires a working Python 3 interpreter, pip, and the venv module. On Debian/Ubuntu these are separate packages; a minimal Python install can pass a `python --version` check yet produce virtualenvs that are missing pip entirely. Virtualenvs can also become stale when the creating interpreter is upgraded or the venv is moved between machines, leaving broken shebang lines in pip and opentelemetry-instrument scripts. The demo installer targets fresh developer or CI machines where Python may not be installed at all.

## Goals / Non-Goals

**Goals:**

- Fail fast with an actionable message when any Python prerequisite is missing before the installer touches any project files.
- Probe pip availability inside a fresh temporary venv (not just globally) to catch the Debian/Ubuntu `python3-venv`-without-pip case.
- Invoke pip and opentelemetry-instrument via `python -m pip` through the venv's Python binary to be shebang-agnostic.
- Detect a stale project virtualenv and prompt the user before deleting and recreating it.
- Add automatic Python installation to the demo flow using the platform package manager, surfaced in the plan preview before confirmation.

**Non-Goals:**

- Automatically install Python for `dtwiz install otel-python` (only for the demo flow).
- Support package managers other than brew, apt-get, dnf, and winget.
- Detect or repair all possible venv corruption states beyond a broken interpreter path.

## Decisions

- `validatePythonPrerequisites()` is the single entry point for all pre-flight checks; it runs before any installation step in the OTel Python flow.
- Interpreter detection (`DetectPython`) scans all matching executables on PATH (not just `exec.LookPath`) so Windows Store Python stubs can be skipped without stopping the search.
- The venv pip probe creates a real temporary virtualenv and runs `python -m pip --version` inside it. Temp dir creation failure returns `true` (assume satisfied) to avoid blocking on infrastructure noise; venv creation failure returns `false`.
- Pip is always invoked as `<venvPython> -m pip` (via `detectProjectPip`) rather than by running the `bin/pip` shebang script, so interpreter path changes after venv creation never break the install flow. The same approach applies to `opentelemetry-instrument`.
- Stale venv detection (`isVenvHealthy`) runs the venv's Python binary to confirm it is executable. A missing or non-runnable binary is treated as stale.
- The demo flow calls `pythonInstallPlan()` before building the plan preview so the install step (if any) is shown to the user before they confirm. On Windows, `installPythonWindows` refreshes the process PATH from the registry after winget completes so dtwiz can immediately detect the newly installed interpreter.
- Linux distro detection reads `/etc/os-release`; unrecognised distros fall back to dnf (RHEL family).

## Risks / Trade-offs

- The venv pip probe creates and removes a temporary virtualenv on every `dtwiz install otel-python` or `dtwiz setup` run. Creation takes roughly one second; this is acceptable for a one-time setup command.
- `Python.Python.3.14` (used on Windows via winget) will need updating when a newer Python minor version replaces it as the recommended winget package.
- PATH refresh on Windows reads only the user PATH from the registry; machine-wide PATH updates still require a new terminal session.
- Stale venv removal requires user confirmation to avoid silent data loss. If the user declines, the install is aborted.
