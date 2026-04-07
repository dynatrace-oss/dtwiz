# Proposal

## Why

The OTel Python installer lacks a platform-validation section at entry — it was validated only on macOS, so install-time checks should verify prerequisites and surface clear errors on unsupported configurations.

## What Changes

- Add a validation section at the start of `InstallOtelPython()` that checks prerequisites: Python 3 in PATH, pip module availability, venv availability, OS compatibility
- Add `isVenvHealthy()`: smoke-tests the venv's Python binary with `--version` before trusting the venv. If it fails (venv from a different machine/environment), `NeedsVenv` is set and `Execute()` asks for confirmation before deleting and recreating the venv using the current machine's Python — making the setup fully environment-agnostic while keeping the destructive step explicit
- `detectProjectPip()` now invokes pip via the virtualenv's Python binary (`python -m pip`) instead of executing the pip script directly — this avoids `fork/exec … pip: no such file or directory` errors that occur when the pip script's shebang points to a Python interpreter path that no longer exists (common on macOS with Homebrew or after Python version upgrades)
- `opentelemetry-instrument` is launched as `venvPython /path/to/opentelemetry-instrument …` rather than exec-ing the script directly, for the same shebang-safety reason
- `opentelemetry-bootstrap -a install` is run after OTel packages are installed, but verified afterwards — if no framework instrumentation packages are detected in the venv, dtwiz falls back to installing them directly based on the project's installed packages (Flask, Django, requests, etc.). This is required because `opentelemetry-bootstrap` version 0.61b0 silently exits 0 while installing nothing in certain environments.
- All pip and opentelemetry-bootstrap error messages now include the exact command that was executed, making failures self-diagnosable without re-running with debug flags
- Launched processes are tracked via a background goroutine that captures their exit status. If a process crashes within the 2-second settle window, the summary line shows `[crashed: exit status 1 — check log for details]` instead of silently omitting the URL. If all processes have crashed, the Dynatrace traffic-waiting prompt is suppressed and the user is told no services are running. The process management logic is extracted into `pkg/installer/otel_process.go` so it can be reused by future runtime installers (Java, Node.js, etc.)

## Capabilities

### New Capabilities

- `python-install-validation`: Pre-flight validation checks at the start of the Python installer

### Modified Capabilities

## Impact

- `pkg/installer/otel_python.go` — add validation section at top of `InstallOtelPython()`
- `pkg/installer/otel_process.go` — new file; reusable process lifecycle tracking for any runtime installer
- `pkg/installer/otel_python_packages.go` — new file; pip/bootstrap package management extracted from `otel_python.go` (`pipCommand`, `otelPythonPackages`, `installPackages`, `runOtelBootstrap`, `bootstrapRequirementsScript`, `normalizePipName`, `listInstalledPipPackages`, `queryBootstrapRequirements`, `ensureFrameworkInstrumentations`)
- `pkg/installer/otel_python.go` — add `ensureFrameworkInstrumentations()` post-bootstrap verification and direct fallback install
