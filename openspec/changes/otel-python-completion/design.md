# Design

## Context

OTel Python instrumentation (`dtwiz install otel-python`) is fully implemented with project detection, entrypoint detection, virtualenv management, package installation, process launch, and Dynatrace verification. However, there is no `dtwiz uninstall otel-python` command, and the installer lacks pre-flight validation checks.

Current Python install flow: detect projects → user selects one → detect entrypoints → stop running processes → create venv → install packages → launch with `opentelemetry-instrument` → verify in Dynatrace.

## Goals / Non-Goals

**Goals:**

- Add pre-flight validation at the start of `InstallOtelPython()`

**Non-Goals:**

- Changing the existing install flow logic
- Supporting multi-project instrumentation
- Persistent configuration management (tracking what was installed where)

## Decisions

**1. Pre-flight validation checks**
At the start of `InstallOtelPython()`, validate:

- A Python 3 interpreter in PATH — `python3` is tried first; `python` is accepted if it reports Python 3.x
- pip available as a Python module (`<detected-python> -m pip --version`) — PATH lookup of `pip`/`pip3` is irrelevant and can produce false negatives
- `venv` module available (`<detected-python> -m venv --help`)
- Fail with clear error message if any check fails

**2. Never exec virtualenv console-scripts directly; always invoke via the venv Python binary**
Virtualenv console-scripts (pip, opentelemetry-instrument, etc.) embed the absolute path of the Python that created them in a shebang line. This works fine when the venv is used on the same machine with the same Python installation. However, when a venv is created on one machine and used on another, or when Python is upgraded (e.g. via Homebrew), that path no longer exists and the kernel returns `ENOENT` when trying to exec the script, even though the script file itself is present on disk.

The fix is to never exec these scripts directly. Instead, pass them as arguments to the venv Python binary, which treats the shebang as a comment. This applies uniformly to all console-scripts in the venv.

**3. Use a venv health check rather than always recreating**
A stale venv must be detected and replaced before any installation work. The simplest signal is whether the venv's own Python binary actually executes. Running a trivial version probe is cheap, cross-platform, and catches all failure modes: broken symlink, removed interpreter, relocated venv directory.

If the probe fails, the stale venv is scheduled for recreation using the current machine's Python. The plan preview distinguishes "create" from "recreate" so the user understands what happened, and `Execute()` asks for confirmation before deleting the existing venv directory. The confirmation explains that a working virtualenv is required so Python auto-instrumentation can install packages and export data reliably.

**4. Include the executed command in error messages**
When a subprocess fails, the OS error alone is not enough to diagnose the problem — the binary path matters. All pip and bootstrap errors include the full command that was run so the user can reproduce the failure directly without re-running dtwiz.

**5. Track process lifecycle to surface crashes at startup**
After launching an instrumented process, the installer previously had no way to distinguish a crash from a slow-starting service — both result in no port being detected after the 2-second wait. The fix is to call `cmd.Wait()` in a background goroutine, buffering the exit error on a channel. After the settle period, a non-blocking channel read determines whether the process is still running, exited cleanly, or crashed with an error.

The exit result must be cached on first read. A Go channel value is consumed on receive; if `WaitResult()` is called twice (once in `PrintSummaryLine`, once in `PrintProcessSummary` to build the alive list), the second call always finds the channel empty and incorrectly reports the process as still running. The fix is to store the drained value on the struct so all subsequent calls return the same cached answer.

The Dynatrace traffic-waiting step (polling for service entities) is only meaningful if at least one process is alive. If all processes have exited or crashed, it is skipped and the user is shown a clear error prompt instead. Polling against an empty service list would produce no output and leave the user confused.

This logic is extracted into `pkg/installer/otel_process.go` (`ManagedProcess`, `StartManagedProcess`, `PrintProcessSummary`) rather than living inline in the Python installer, so future runtime installers (Java, Node.js) can reuse it without duplication. The `syscall` package is intentionally avoided — `cmd.Wait()` returns a platform-neutral `*exec.ExitError` that works on all OS targets.

## Alternatives Considered

**Always recreate the venv instead of health-checking it**
Simpler code path — no `isVenvHealthy()` needed. Rejected because it discards the user's installed packages and adds significant install time on every run, even when the venv is perfectly healthy.

**Automatically delete a stale venv without a second confirmation**
Simpler execution flow once the plan is confirmed. Rejected because removing a virtualenv is still destructive local state change, and directories such as `env` or `.env` are user-visible enough that the installer should make the delete step explicit at the moment it happens.

**`python -m venv --upgrade` to repair a stale venv**
The `--upgrade` flag re-links the venv to a new Python. Rejected because it requires the *original* Python version to still be present (or a compatible one), which is exactly the scenario we are recovering from. A clean recreate is more reliable.

**Check `pyvenv.cfg` to validate the venv**
The venv config file records the Python home path; we could stat that path to detect staleness. Rejected because `--version` is a single cross-platform call that validates the actual binary, not just its recorded source path.

**Use the pip script directly with an explicit interpreter prefix (`/usr/bin/env python pip ...`)**
Not portable — depends on `env` behaviour and the pip script accepting that calling convention. The `python -m pip` approach is the officially supported way to invoke pip and is unambiguous.

**Use `syscall.Signal(0)` to probe process liveness**
Sending signal 0 to a PID checks existence without delivering a real signal. Rejected: `syscall` is frozen by the Go team in favour of `golang.org/x/sys`, and `Signal(0)` is Unix-only — it would break on Windows. `cmd.Wait()` is cross-platform and provides the exit code, which is more useful.

**Poll `/proc/<pid>/status` on Linux**
Cheap on Linux but unavailable on macOS and Windows. The goroutine approach works identically on all platforms.

### 6. Always verify framework instrumentation packages after running bootstrap

`opentelemetry-bootstrap -a install` is unreliable: in some environments (observed with `opentelemetry-distro==0.61b0` on Python 3.14) it exits 0 but installs zero packages — no error, no output, no indication anything went wrong. The root cause is inside the third-party tool's CLI entry point; the internal detection API (`_find_installed_libraries()`) works correctly when called from Python.

After bootstrap runs, dtwiz shall:

1. Run `pip list --format=json` and check whether any framework instrumentation package was installed.
2. If none were installed, call bootstrap's internal detection API directly via a Python snippet (`bootstrapRequirementsScript`). This bypasses the broken CLI entry point and uses bootstrap's own version-matching logic — no hardcoded map needed, automatically picks up new packages as the OTel ecosystem evolves.
3. If the internal API call fails (e.g. API changed across versions), print a warning with the manual `opentelemetry-bootstrap -a install` command and continue non-fatally — services will start but may lack framework trace spans.
4. pip-install the missing packages directly.
5. After installation, verify again. If any packages are still missing, print a clear warning listing the packages and the exact `pip install` command so the user can install them manually.

Package names are PEP 503-normalized (lowercase, underscores and dots replaced with hyphens) before comparison.

This two-tier approach (bootstrap API → hardcoded fallback) is environment-agnostic and self-healing: it works regardless of Python version, OS, or venv tool, and it always tells the user what happened and how to fix it.

## Alternatives Considered (Decision 6)

**Use a hardcoded fallback map when the bootstrap API is unavailable**
Would allow silent automatic recovery. Rejected because the map requires manual maintenance as new OTel instrumentation packages are released and cannot match version constraints the way bootstrap can. A non-fatal warning with the manual command is clearer and less likely to install wrong versions.

**Skip bootstrap entirely and always install from the API**
Would work, but bootstrap also installs non-framework instrumentations (asyncio, threading, logging, sqlite3) that improve observability. Worth giving it a chance to run.

## File Layout

`otel_python.go` grew to over 1,100 lines across four distinct concerns. It has since been split into focused files following the same pattern used when `otel_process.go` was extracted.

| File | Responsibility |
|---|---|
| `otel_python.go` | Env var generation, plan prompting, `DetectPythonPlan`, `Execute`, `InstallOtelPython`, DQL service polling |
| `otel_python_venv.go` | `detectPython`, `validatePythonPrerequisites`, `resolveVenvBinary`, `detectProjectVenvDir`, `detectProjectPip`, `isVenvHealthy`, `removeStaleVirtualenv` |
| `otel_python_project.go` | `PythonProcess`, `PythonProject`, `detectPythonProjects`, `detectPythonProcesses`, `matchProcessesToProjects`, `stopProcesses`, `detectPythonEntrypoints`, `serviceNameFromEntrypoint`, `parseEntrypointFromPyproject`, `getProcessCWD` |
| `otel_python_packages.go` | `pipCommand`, `otelPythonPackages`, `installPackages`, `runOtelBootstrap`, `bootstrapRequirementsScript`, `normalizePipName`, `listInstalledPipPackages`, `queryBootstrapRequirements`, `ensureFrameworkInstrumentations`, `installProjectDeps`, `projectDepsDescription` |
| `otel_process.go` | `ManagedProcess`, `StartManagedProcess`, `PrintProcessSummary`, port detection |

All five files share the same `installer` package — no interface changes, no new public API.

## Risks / Trade-offs

- When bootstrap's internal API is unavailable (e.g. upstream API change), dtwiz surfaces a non-fatal warning and prints the manual command — no framework instrumentations are auto-installed in that scenario.
- None identified for the validation approach.
