## Context

`dtwiz uninstall otel` lives in `pkg/installer/otel_uninstall.go`. Before this change it found running `dynatrace-otel-collector` processes, identified candidate install directories, showed a preview, and killed/removed everything after confirmation.

The process detection infrastructure in `otel_runtime_scan.go` provides `detectProcesses(filterTerm, excludeTerms)` which scans full command lines via `ps ax` (Unix) or `Get-CimInstance` (Windows) and returns `[]DetectedProcess`. `stopProcesses(pids)` handles graceful shutdown cross-platform. The install-time Python flow (`otel_python_project.go`) already uses `detectProcesses("python", []string{"pip ", "setup.py", "/bin/dtwiz"})` to find running Python apps.

The `add-java-auto-instrumentation` branch (Task 13) is adding Java cleanup to the same `UninstallOtelCollector()` function using an additive, per-runtime approach.

## Goals / Non-Goals

**Goals:**
- Detect and stop running Python processes instrumented by dtwiz as part of `dtwiz uninstall otel`.
- Reuse existing detection and process-stop infrastructure with no new platform-specific code.
- Stay additive and conflict-free with the Java uninstall branch.

**Non-Goals:**
- Removing Python venvs, OTel packages, or config files.
- Detecting Node.js, Go, or other runtimes.
- Introducing a shared `RuntimeCleaner` abstraction (premature with two runtimes).

## Decisions

### Filter on `"python"`, not `"opentelemetry-instrument"`

`opentelemetry-instrument` calls `os.execl` on Unix, replacing its own process image with the Python interpreter. On Windows it spawns a Python child and exits. In both cases the surviving process appears as a plain `python` command in `ps` — filtering on `"opentelemetry-instrument"` returns nothing. The install-time code already solved this with the `"python"` filter and the same exclude list; uninstall reuses that exact call.

### Additive per-runtime pattern, no shared abstraction

Each runtime adds a standalone detection function (`findInstrumentedPythonProcesses`) and an independent preview/kill block inside `UninstallOtelCollector()`. This mirrors what the Java branch does and avoids touching lines that branch is also modifying. An interface-based abstraction can be introduced later as a pure refactor once three or more runtimes exist.

### Collector stopped before Python processes

The collector is the telemetry sink; instrumented apps lose their export target the moment it stops. Stopping the collector first, then the apps, is the natural ordering and avoids any contention.

## Risks / Trade-offs

- **False positives**: Any `python` process not started by dtwiz will appear in the preview. Mitigation: the preview shows the full command so the user can cancel if something looks unexpected.
- **Merge conflict risk with Java branch**: Low. The Java section is a separate block; the only shared line is the final success message, which is a trivial conflict to resolve.

## Migration Plan

Additive and backward-compatible. If no Python processes are running the output is identical to the previous version. Rollback: revert the additions to `otel_uninstall.go` and delete `otel_uninstall_python_test.go`.
