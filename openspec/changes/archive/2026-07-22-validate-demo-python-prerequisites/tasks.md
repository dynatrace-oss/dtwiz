# 1. Python Prerequisite Validation

- [x] 1.1 Implement `validatePythonPrerequisites()` in `pkg/installer/otel/python_venv.go` that checks Python 3, pip, venv module, and pip-in-fresh-venv in sequence.
- [x] 1.2 Implement `probeVenvPip(pythonBin)` that creates a temporary virtualenv and verifies pip is present inside it.
- [x] 1.3 Return a clear error when `python -m venv --help` fails, suggesting `apt install python3-venv` on Debian/Ubuntu.
- [x] 1.4 Return `true` (assume satisfied) when temp dir creation fails during the venv probe.

## 2. Shebang-Safe pip and Script Invocation

- [x] 2.1 Implement `detectProjectPip(projectPath)` to resolve pip as `<venvPython> -m pip` instead of the pip shebang script.
- [x] 2.2 Apply the same pattern to `opentelemetry-instrument` invocation so a broken shebang never causes a failure.
- [x] 2.3 Implement `isVenvHealthy(projectPath)` to verify the venv's Python binary is executable.

## 3. Stale Virtualenv Handling

- [x] 3.1 Detect a stale project venv via `isVenvHealthy` before any pip install step.
- [x] 3.2 Prompt the user (`confirmRecreateVirtualenv`) before removing and recreating a stale venv.
- [x] 3.3 Surface the recreate action distinctly in the plan preview (vs. a fresh create).
- [x] 3.4 Implement `removeStaleVirtualenv(venvDir)` that removes the directory only after user confirmation.

## 4. Demo Flow Python Auto-Install

- [x] 4.1 Implement `pythonInstallPlan()` in `pkg/installer/otel/demo.go` that returns the platform-appropriate install command or nil if prerequisites are present.
- [x] 4.2 On macOS: return `brew install python3`; surface an actionable error if brew is not available.
- [x] 4.3 On Linux Debian/Ubuntu: return `sudo apt-get install -y python3 python3-pip python3-venv`.
- [x] 4.4 On Linux RHEL/Fedora/CentOS (and unknown distros): return `sudo dnf install -y python3 python3-pip python3-venv`.
- [x] 4.5 On Windows: implement `installPythonWindows()` using `winget install --id Python.Python.3.14` with agreement flags; refresh PATH from registry after install.
- [x] 4.6 On Linux: also probe pip-in-venv (not just the interpreter) before deciding prerequisites are present.
- [x] 4.7 Include the Python install step in the plan preview inside `InstallDemo()` before the confirmation prompt.
- [x] 4.8 Implement `describeDemoInstallCmd(cmd)` for human-readable plan labels (e.g., `python3, python3-pip, python3-venv via apt-get`).

## 5. Validation

- [x] 5.1 Add unit tests for `probeVenvPip` scenarios: pip present, pip absent, venv creation failure, temp dir failure.
- [x] 5.2 Add unit tests for `pythonInstallPlan` across platforms and distros.
- [x] 5.3 Add unit tests for `describeDemoInstallCmd`.
- [x] 5.4 Add or update tests for stale venv detection and recreation flow.
- [x] 5.5 Write OpenSpec specs covering the new validation and demo auto-install behavior.
- [x] 5.6 Run `make test`, `make lint`, and verify cross-platform build compiles cleanly.
