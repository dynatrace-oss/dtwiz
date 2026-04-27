# Design: Uninstall Python OTel Processes

## Context

`dtwiz uninstall otel` lives in `pkg/installer/otel_uninstall.go`. Before this change it found running `dynatrace-otel-collector` processes, identified candidate install directories, showed a preview, and killed/removed everything after confirmation.

The process detection infrastructure in `otel_runtime_scan.go` provides `detectProcesses(filterTerm, excludeTerms)` which scans full command lines via `ps ax` (Unix) or `Get-CimInstance` (Windows) and returns `[]DetectedProcess`. `stopProcesses(pids)` handles graceful shutdown cross-platform. The install-time Python flow (`otel_python_project.go`) already uses `detectProcesses("python", []string{"pip ", "setup.py", "/bin/dtwiz"})` to find running Python apps.

## Goals / Non-Goals

### Goals

- Detect and stop running Python processes instrumented by dtwiz as part of `dtwiz uninstall otel`.
- Reuse existing detection and process-stop infrastructure with no new platform-specific code.
- Introduce a `RuntimeCleaner` interface so future runtimes register a single implementation with no changes to the uninstall flow.

### Non-Goals

- Removing Python venvs, OTel packages, or config files.
- Detecting other runtimes (each registers its own `RuntimeCleaner` when ready).

## Decisions

### Filter on `"python"`, then confirm with project directory cross-reference

`opentelemetry-instrument` calls `os.execl` on Unix, replacing its own process image with the Python interpreter. On Windows it spawns a Python child and exits. In both cases the surviving process appears as a plain `python` command in `ps` — filtering on `"opentelemetry-instrument"` returns nothing. The broad `"python"` filter is the correct first pass.

However, the broad filter alone produces false positives: every Python process on the system is listed, not just OTel-instrumented ones. A second pass cross-references each candidate's working directory against Python project directories discovered on disk (`scanProjectDirs` with markers `pyproject.toml`, `setup.py`, `setup.cfg`, `requirements.txt`, `Pipfile`, `poetry.lock`, `manage.py`). A process is included only if its working directory starts with a known project path, or its command line contains the project path. This is the same `matchProcessesToProjects` mechanism used at install time, so the two flows stay consistent.

### `RuntimeCleaner` interface over per-runtime code blocks

A two-method interface (`Label() string`, `DetectProcesses() []DetectedProcess`) replaces the hardcoded Python section in `UninstallOtelCollector()`. The function iterates a package-level `runtimeCleaners` registry for preview and stop — adding a new runtime is a single registration line with no changes to the uninstall flow itself.

`Stop()` is deliberately excluded from the interface: `stopProcesses(pids)` already handles graceful shutdown cross-platform and is the same for every runtime.

### Collector stopped before runtime processes

The collector is the telemetry sink; instrumented apps lose their export target the moment it stops. Stopping the collector first, then the apps, is the natural ordering.

## Risks / Trade-offs

- **False positives reduced but not eliminated**: The project directory cross-reference eliminates unrelated Python processes, but a process running from inside a Python project directory that is not dtwiz-instrumented would still appear. This is an acceptable edge case.
- **Misses out-of-tree processes**: A process launched from outside the scanned project directories (e.g. an absolute path launch from a different CWD) will not be detected. Acceptable given dtwiz always starts instrumented apps from within the project directory.

## Migration Plan

Additive and backward-compatible. If no Python processes are running the output is identical to the previous version. Rollback: revert `otel_uninstall.go` and delete `otel_uninstall_python_test.go`.
