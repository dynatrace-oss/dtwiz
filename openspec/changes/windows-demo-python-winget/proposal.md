## Why

Windows installs can update the user PATH without updating the current dtwiz process. The demo installer also needs to avoid the Windows Store Python stub and use a real winget Python package ID.

## What Changes

- Add a shared Windows PATH refresh helper for installers.
- Read the Windows user PATH directly from the registry instead of spawning PowerShell.
- Use the shared helper from Kubernetes and demo setup flows.
- Skip Windows Store Python stubs when detecting Python.
- Use `winget install --id Python.Python.3.14` when Python is missing on Windows.
- Keep the install non-interactive by passing winget agreement flags during execution.

## Capabilities

### New Capabilities

- `windows-demo-python-winget`: Defines the Windows winget package ID used by the demo installer when Python is missing.
- `windows-path-refresh`: Defines how installers refresh the current process PATH after Windows package installs.
- `windows-python-detection`: Defines how Python detection avoids Windows Store stubs and finds a real Python 3 interpreter.

### Modified Capabilities

- None.

## Impact

- **Code**: `pkg/installer/path.go`, `pkg/installer/path_windows.go`, `pkg/installer/path_other.go`, `pkg/installer/kubernetes/*.go`, `pkg/installer/otel/demo.go`, `pkg/installer/otel/python_venv.go`.
- **UX**: Windows users can continue setup after package installs without opening a new terminal, and Python detection ignores Store launch aliases.
- **Dependencies**: No new dependency; uses existing `golang.org/x/sys/windows/registry` support and winget when available on Windows.
