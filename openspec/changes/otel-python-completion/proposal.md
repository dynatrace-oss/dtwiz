## Why

The OTel Python installer lacks a platform-validation section at entry — it was validated only on macOS, so install-time checks should verify prerequisites and surface clear errors on unsupported configurations.

## What Changes

- Add a validation section at the start of `InstallOtelPython()` that checks prerequisites: Python 3 in PATH, pip/venv availability, OS compatibility

## Capabilities

### New Capabilities
- `python-install-validation`: Pre-flight validation checks at the start of the Python installer

### Modified Capabilities

## Impact

- `pkg/installer/otel_python.go` — add validation section at top of `InstallOtelPython()`
