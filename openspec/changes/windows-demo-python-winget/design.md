## Context

Windows package managers can install tools and update the user PATH, but the running dtwiz process keeps the PATH it inherited at startup. Windows can also expose Python Store launch aliases that are not usable interpreters for instrumentation.

## Goals / Non-Goals

**Goals:**

- Use a real winget package ID for Windows Python installation.
- Refresh the current process PATH after Windows package installs.
- Share PATH refresh logic across installers.
- Skip Windows Store Python stubs when detecting Python.
- Keep the install command simple and predictable.
- Avoid extra winget prompts during demo setup.

**Non-Goals:**

- Resolve the latest Python package dynamically.
- Change macOS or Linux Python installation behavior.
- Add a new package manager fallback.

## Decisions

- Use `Python.Python.3.14` as the Windows winget package ID because it exists as a real winget package.
- Keep `--id` in the command so winget searches by package identifier.
- Keep agreement flags in the execution path so setup can continue without extra prompts.
- Move Windows PATH refresh into `installer.RefreshWindowsPath()` so Kubernetes and demo installers use the same behavior.
- Keep shared PATH merge helpers in `pkg/installer/path.go`, with build-tagged refresh implementations in `path_windows.go` and `path_other.go`.
- Read the Windows user PATH directly through `golang.org/x/sys/windows/registry` to avoid spawning PowerShell for every refresh.
- Refresh PATH before checking for Python in the demo flow, and again after winget runs.
- Replace single-result `exec.LookPath` Python detection with PATH scanning so Store stubs can be skipped and later real interpreters can still be used.

## Risks / Trade-offs

- `Python.Python.3.14` may become stale over time. Update the package ID when the branch moves to a newer Python minor version.
- A machine without winget still cannot auto-install Python. The installer should continue to surface a manual install path in that case.
- PATH refresh only reads the user PATH. If a tool updates only the machine PATH, a new terminal may still be needed.
- Direct registry access is Windows-only, so the refresh implementation is split by build tags.
- PATH scanning is slightly more work than `exec.LookPath`, but it lets dtwiz skip bad candidates and keep searching.
