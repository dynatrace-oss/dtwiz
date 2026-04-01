# Proposal

## Why

The OTel Python installer lacks a platform-validation section at entry — it was validated only on macOS, so install-time checks should verify prerequisites and surface clear errors on unsupported configurations.

## What Changes

- Add a validation section at the start of `InstallOtelPython()` that checks prerequisites: Python 3 in PATH, pip module availability, venv availability, OS compatibility
- Add `isVenvHealthy()`: smoke-tests the venv's Python binary with `--version` before trusting the venv. If it fails (venv from a different machine/environment), `NeedsVenv` is set and `Execute()` deletes and recreates the venv using the current machine's Python — making the setup fully environment-agnostic
- `detectProjectPip()` now invokes pip via the virtualenv's Python binary (`python -m pip`) instead of executing the pip script directly — this avoids `fork/exec … pip: no such file or directory` errors that occur when the pip script's shebang points to a Python interpreter path that no longer exists (common on macOS with Homebrew or after Python version upgrades)
- `opentelemetry-instrument` is launched as `venvPython /path/to/opentelemetry-instrument …` rather than exec-ing the script directly, for the same shebang-safety reason
- All pip and opentelemetry-bootstrap error messages now include the exact command that was executed, making failures self-diagnosable without re-running with debug flags
- Launched processes are tracked via a background goroutine that captures their exit status. If a process crashes within the 2-second settle window, the summary line shows `[crashed: exit status 1 — check log for details]` instead of silently omitting the URL. If all processes have crashed, the Dynatrace traffic-waiting prompt is suppressed and the user is told no services are running. The process management logic is extracted into `pkg/installer/otel_process.go` so it can be reused by future runtime installers (Java, Node.js, etc.)

## Capabilities

### New Capabilities

- `python-install-validation`: Pre-flight validation checks at the start of the Python installer

### Modified Capabilities

## Impact

- `pkg/installer/otel_python.go` — add validation section at top of `InstallOtelPython()`
- `pkg/installer/otel_process.go` — new file; reusable process lifecycle tracking for any runtime installer
