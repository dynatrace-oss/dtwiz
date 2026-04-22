## Why

`dtwiz uninstall otel` currently only kills the OTel Collector process and removes its installation directory. It leaves instrumented application processes (e.g., Python apps launched via `opentelemetry-instrument`) running, forcing users to find and kill them manually. This breaks the zero-config promise and leaves orphaned processes tied to a Dynatrace tenant the user may be cleaning up.

## What Changes

- Extend `dtwiz uninstall otel` to detect and stop running OTel-instrumented Python processes in addition to the collector.
- Follow the same per-runtime function pattern that the Java uninstall change (`add-java-auto-instrumentation`, Task 13) is using — no shared abstraction that would conflict with that branch.
- Only processes are stopped — instrumentation artifacts (venvs, OTel packages, config files) are left intact for easy re-enablement.
- The existing preview/confirmation UX is extended with a Python section; `--dry-run` and `--yes` flags apply automatically.
- The approach is intentionally incremental: each runtime adds its own detection function and preview section independently. Adding Node.js, Go, or additional runtimes later requires no refactoring of existing code.

## Capabilities

### New Capabilities

- `otel-python-process-uninstall`: Detect running OTel-instrumented Python processes (those launched via `opentelemetry-instrument`) and stop them as part of `dtwiz uninstall otel`. Processes are identified by scanning running `python` processes for `opentelemetry-instrument` in their command line. Only stopping — no artifact removal.

### Modified Capabilities

_(none — no existing spec-level requirements change)_

## Impact

- **`pkg/installer/otel_uninstall.go`**: New `findInstrumentedPythonProcesses()` function; `UninstallOtelCollector()` extended with Python process section in preview and kill loop.
- **`pkg/installer/otel_runtime_scan.go` / `otel_runtime_scan_unix.go` / `otel_runtime_scan_windows.go`**: Read-only reuse of existing `detectProcesses()` — no changes needed.
- **CLI**: No new commands or flags. `dtwiz uninstall otel` gains additional behavior transparently.
- **Merge safety**: Changes are additive, confined to new lines in `otel_uninstall.go`. The Java uninstall branch (`add-java-auto-instrumentation` Task 13) modifies the same file but adds a Java-specific section. The two sets of changes touch different lines and will not produce meaningful merge conflicts regardless of merge order.
