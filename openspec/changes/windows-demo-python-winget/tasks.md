# 1. Windows Python Package

- [x] 1.1 Confirm the winget package ID exists for Python 3.14.
- [x] 1.2 Use `Python.Python.3.14` for Windows demo Python installation.
- [x] 1.3 Keep winget agreement flags in the Windows install command.
- [x] 1.4 Surface missing `winget` as a clear error.
- [x] 1.5 Preserve winget failure details when Python is still unavailable.

## 2. Windows PATH Refresh

- [x] 2.1 Move Windows PATH refresh into shared installer utilities.
- [x] 2.2 Use the shared PATH refresh from Kubernetes install, uninstall, and Helm install flows.
- [x] 2.3 Refresh PATH before and after demo Python installation on Windows.
- [x] 2.4 Move PATH merge helpers into a path-specific helper file.
- [x] 2.5 Read the Windows user PATH directly from the registry instead of spawning PowerShell.

## 3. Python Detection

- [x] 3.1 Scan all matching Python executables on PATH.
- [x] 3.2 Skip Windows Store Python stubs.
- [x] 3.3 Keep accepting the first real Python 3 interpreter.

## 4. Validation

- [x] 4.1 Add branch-specific OpenSpec specs for the significant Windows behavior.
- [x] 4.2 Add or update tests for shared PATH merge behavior and Python detection.
- [x] 4.3 Run focused tests, lint, Windows compile check, and OpenSpec validation.
