## Context

`dtwiz uninstall otel` is implemented in `pkg/installer/otel_uninstall.go`. Currently it:
1. Finds running `dynatrace-otel-collector` processes via `findRunningOtelProcesses()`
2. Identifies candidate install directories via `candidateOtelDirs()`
3. Shows a preview and prompts for confirmation
4. Kills collector processes and removes directories

The process detection infrastructure already exists in `otel_runtime_scan.go`: `detectProcesses(filterTerm, excludeTerms)` returns `[]DetectedProcess{PID, Command, WorkingDirectory}` for any process matching a filter string. `stopProcesses(pids)` handles graceful shutdown (SIGINT on Unix, Kill on Windows).

The Java uninstall branch (`add-java-auto-instrumentation`, Task 13) is adding Java cleanup to the same `UninstallOtelCollector()` function using a parallel, additive approach: a new `findInstrumentedJavaProcesses()` function feeds an extra preview section and kill loop.

## Goals / Non-Goals

**Goals:**
- Detect running Python processes instrumented via `opentelemetry-instrument` and stop them as part of `dtwiz uninstall otel`.
- Follow the same additive pattern as the Java uninstall change to avoid merge conflicts.
- Reuse `detectProcesses()` from `otel_runtime_scan.go` — no new detection primitives.
- Keep the UX consistent: unified preview listing both collector and Python processes, single confirmation, `--dry-run` respected.

**Non-Goals:**
- Removing Python virtualenvs, OTel packages, or any files — stop processes only.
- Detecting Node.js, Go, or other runtimes (handled by future changes).
- A generic `RuntimeCleaner` interface — premature with two runtimes and would require refactoring Java's already-specced code.

## Decisions

### 1. Additive pattern over shared abstraction

**Decision:** Add a standalone `findInstrumentedPythonProcesses()` function and a Python section in `UninstallOtelCollector()`, mirroring exactly what the Java branch does for Java.

**Alternatives considered:**
- *`RuntimeCleaner` interface*: Clean long-term but forces refactoring of the Java branch's approach, creating a merge conflict and requiring coordination with that team. Deferred until a third or fourth runtime warrants it.
- *Single combined loop over `[]runtimeCleaner`*: Same problem — touches lines the Java branch is also modifying.

**Rationale:** Two runtimes don't justify an abstraction. When Node.js and Go are added, a refactor to an interface can be done in a single dedicated commit without blocking any runtime.

### 2. Process identification via command-line marker

**Decision:** A Python process is considered OTel-instrumented if its command line contains `opentelemetry-instrument`. This is the binary that dtwiz uses to launch instrumented Python apps (`opentelemetry-instrument python <entrypoint>`).

**Alternatives considered:**
- *Scan for OTEL_* env vars on the process*: Not reliably accessible cross-platform without elevated privileges.
- *Match by working directory against known projects*: Requires project state that uninstall doesn't have. Too fragile.

**Rationale:** The marker is unambiguous — only dtwiz-managed Python processes use this wrapper. False positives from unrelated tools using the same wrapper are acceptable: the preview lists them and the user confirms.

### 3. Reuse `detectProcesses()` — no new platform-specific code

**Decision:** Call `detectProcesses("opentelemetry-instrument", nil)` from `otel_runtime_scan_unix.go` / `otel_runtime_scan_windows.go`. This already searches command lines cross-platform.

**Rationale:** The infrastructure is already tested and handles Windows PowerShell vs Unix `ps` differences. No reason to duplicate it.

### 4. Placement in `UninstallOtelCollector()` preview and kill sections

**Decision:** Python section is appended *after* the collector section in the preview, and Python processes are killed *after* collector processes. Java (when merged) adds its section independently in the same relative position.

**Rationale:** Collector should always be stopped first — it is the telemetry sink. Instrumented apps can safely be stopped after the collector is gone. This ordering is natural and avoids any ordering conflict with Java's changes.

## Risks / Trade-offs

- **False positives**: Any process with `opentelemetry-instrument` in its command line will appear, including processes not started by dtwiz. Mitigation: the preview lists them explicitly so the user can cancel if something looks wrong.
- **Nothing to show if no Python apps are running**: Python section is silently omitted from preview (same behaviour as the collector section when no collector is found). No noise.
- **Merge conflict risk with Java branch**: Low. The Java branch adds its section to `UninstallOtelCollector()` around the collector kill loop. Python adds its section in a parallel but separate block. The only potential conflict is the final success message line — trivial to resolve.

## Migration Plan

No migration needed. The change is additive and backward-compatible:
- If no instrumented Python processes are running, output is identical to today.
- `--dry-run` and `--yes` flags already propagate through `UninstallOtelCollector()` — no changes to the flag wiring.
- Rollback: revert `otel_uninstall.go` additions. No other files change.
