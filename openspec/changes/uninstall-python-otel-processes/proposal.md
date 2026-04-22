# Proposal: Uninstall Python OTel Processes

## Why

`dtwiz uninstall otel` only killed the OTel Collector process and removed its installation directory. Python apps launched via `opentelemetry-instrument` were left running, forcing users to kill them manually — breaking the zero-config promise and leaving orphaned processes tied to a tenant the user is cleaning up.

## What Changes

- `dtwiz uninstall otel` now detects and stops running OTel-instrumented Python processes in addition to the collector.
- Detection uses the same `detectProcesses("python", excludeTerms)` filter as install-time — necessary because `opentelemetry-instrument` calls `os.execl` on Unix, replacing itself with the `python` process image, so the surviving process is a plain `python` command with no wrapper visible in `ps`.
- Only processes are stopped — venvs, packages, and config files are left intact for easy re-enablement.
- Implementation is additive: a standalone `findInstrumentedPythonProcesses()` function and a Python section in the existing preview/confirm/kill flow, following the same per-runtime pattern the `add-java-auto-instrumentation` change uses for Java. No shared abstraction is introduced; future runtimes add their own independent section.

## Capabilities

### New Capabilities

_(none)_

### Modified Capabilities

- `python-install-validation`: Extend with uninstall-side requirements — detecting running Python processes and stopping them as part of `dtwiz uninstall otel`, including preview, dry-run, and debug logging behaviour.

## Impact

- **`pkg/installer/otel_uninstall.go`**: New `findInstrumentedPythonProcesses()`; `UninstallOtelCollector()` extended with Python preview section and stop block.
- **`pkg/installer/otel_uninstall_python_test.go`**: New test file covering detection, self-exclusion, and preview section presence.
- **CLI**: No new commands or flags; `dtwiz uninstall otel` gains additional behaviour transparently.
- **Merge safety**: Additive changes confined to new lines in `otel_uninstall.go`; compatible with the Java uninstall branch regardless of merge order.
