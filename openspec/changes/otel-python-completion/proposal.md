## Why

OTel Python auto-instrumentation (`dtwiz install otel-python`) is fully implemented, but there is no corresponding `dtwiz uninstall otel-python` command yet. This leaves users without a clean way to remove Python instrumentation. Additionally, the installer lacks a platform-validation section at entry — it was validated only on macOS, so install-time checks should verify prerequisites and surface clear errors on unsupported configurations.

## What Changes

- Implement `dtwiz uninstall otel-python` — stop instrumented processes, remove installed OTel Python packages, restore original launch configuration
- Add a validation section at the start of `InstallOtelPython()` that checks prerequisites: Python 3 in PATH, pip/venv availability, OS compatibility
- Add validation guidance for verifying instrumentation works (requires sending traffic to the instrumented app)

## Capabilities

### New Capabilities
- `python-uninstall`: Uninstall OTel Python auto-instrumentation — stop instrumented processes and remove OTel Python packages
- `python-install-validation`: Pre-flight validation checks at the start of the Python installer

### Modified Capabilities

## Impact

- `pkg/installer/otel_python.go` — add validation section at top of `InstallOtelPython()`
- New file: `pkg/installer/otel_python_uninstall.go` — uninstall logic
- `cmd/uninstall.go` — register `otel-python` subcommand
