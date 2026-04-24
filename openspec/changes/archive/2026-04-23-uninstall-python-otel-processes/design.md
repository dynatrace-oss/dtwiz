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

### Filter on `"python"`, then confirm with OTel env vars

`opentelemetry-instrument` calls `os.execl` on Unix, replacing its own process image with the Python interpreter. On Windows it spawns a Python child and exits. In both cases the surviving process appears as a plain `python` command in `ps` — filtering on `"opentelemetry-instrument"` returns nothing. The broad `"python"` filter is the correct first pass.

However, the broad filter alone produces false positives: every Python process on the system is listed, not just OTel-instrumented ones. A second pass checks each candidate process for OTel env vars (`OTEL_SERVICE_NAME` or `OTEL_EXPORTER_OTLP_ENDPOINT`). Processes with these vars set were launched under `opentelemetry-instrument` (which injects them) and are the only ones that belong in the uninstall preview.

**Platform implementation:**

- **macOS**: `ps eww -p <pid> -o command=` emits the full env block alongside the command; scan the output for the marker var names.
- **Linux**: Read `/proc/<pid>/environ` (null-delimited key=value pairs); check for marker keys.
- **Windows**: `Win32_Process` does not expose env vars. dtwiz always launches instrumented Python apps via the virtualenv Python binary (e.g. `.venv\Scripts\python.exe`). The command line is checked for any known venv name followed by `\Scripts\` (using the same `venvNames` slice as the installer). A plain `python script.py` launched by the user will never have this path.

### `RuntimeCleaner` interface over per-runtime code blocks

A two-method interface (`Label() string`, `DetectProcesses() []DetectedProcess`) replaces the hardcoded Python section in `UninstallOtelCollector()`. The function iterates a package-level `runtimeCleaners` registry for preview and stop — adding a new runtime is a single registration line with no changes to the uninstall flow itself.

`Stop()` is deliberately excluded from the interface: `stopProcesses(pids)` already handles graceful shutdown cross-platform and is the same for every runtime.

### Collector stopped before runtime processes

The collector is the telemetry sink; instrumented apps lose their export target the moment it stops. Stopping the collector first, then the apps, is the natural ordering.

## Risks / Trade-offs

- **False positives reduced but not eliminated**: The env var check eliminates plain Python processes, but a process that happens to have `OTEL_SERVICE_NAME` set for reasons unrelated to dtwiz would still appear. This is an acceptable edge case.
- **Windows accuracy**: The Windows fallback (command-line check) may miss processes on some configurations. Acceptable given Windows is not the primary target platform.

## Migration Plan

Additive and backward-compatible. If no Python processes are running the output is identical to the previous version. Rollback: revert `otel_uninstall.go` and delete `otel_uninstall_python_test.go`.
