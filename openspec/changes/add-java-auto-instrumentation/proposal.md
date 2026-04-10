# Proposal

## Why

`dtwiz install otel-java` currently prints manual instructions — the user must download the agent JAR themselves, set environment variables by hand, and restart their application. This is inconsistent with the fully automated Python installer and violates the project's "if we detect it, we enable it" principle. Java is gated behind `DTWIZ_ALL_RUNTIMES` and marked "Coming soon". Making it production-ready brings feature parity across the two most common server-side runtimes.

## What Changes

- Implement a fully automated `InstallOtelJava()` flow that downloads the OpenTelemetry Java agent JAR from GitHub releases, detects running Java processes, reconstructs their launch commands, and restarts them with `-javaagent` and `OTEL_*` environment variables configured for Dynatrace.
- Add pre-flight validation: Java in PATH, version >= 8.
- Add Java version parsing that handles all common `java -version` output formats (openjdk version "1.8.0_…", java version "17.0.1", etc.).
- Detect running Java processes via `ps ax` on Unix/macOS (supplemented by `jps` when available) and present an interactive selection menu. On Windows, use `wmic` / `tasklist` for process discovery.
- Reconstruct the selected process's launch command from its `ps` output, handling common patterns: `java -jar app.jar`, classpath-based (`java -cp …`), module-based (`java -m …`). Print a warning for complex launch methods (systemd units, Docker containers, shell wrappers) that cannot be fully reconstructed.
- After launching the instrumented process, reuse the existing `ManagedProcess` / `PrintProcessSummary` infrastructure from `otel_process.go` and call `waitForServices()` to verify the service appears in Dynatrace via DQL.
- Extend `InstallOtelJava` signature to accept `platformToken` for DQL verification (same pattern as Python).
- Add `OTEL_EXPORTER_OTLP_PROTOCOL` and `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE` to the Java env vars for consistency with the Python installer (these are already in `generateBaseOtelEnvVars`).
- Remove Java from the `DTWIZ_ALL_RUNTIMES` gate so it appears in `dtwiz install otel` project selection by default.
- `--dry-run` supported for the full flow.

## Capabilities

### New Capabilities

- `java-auto-instrumentation`: Download the OTel Java agent JAR, detect and select a running Java process, reconstruct its launch command, restart it with `-javaagent` and OTEL env vars, and verify traces/logs appear in Dynatrace.
- `java-process-detection`: Detect running Java processes from `ps ax` output (and `jps` when available), parse their command lines to extract the application being run, and present an interactive selection menu. Handle Windows via `wmic`/`tasklist`.
- `java-version-validation`: Pre-flight check that Java is on PATH and the version is >= 8. Parse all common `java -version` output formats.

### Modified Capabilities

- `python-install-validation`: No requirement changes — reuse `ManagedProcess` and `waitForServices` patterns as-is.

## Impact

- **Code**: New files `pkg/installer/otel_java_process.go` (process detection, command parsing, reconstruction), modified `pkg/installer/otel_java.go` (full automated flow replacing manual instructions), modified `cmd/install.go` (pass platformToken to `InstallOtelJava`), modified `pkg/installer/otel.go` (enable Java in default runtimes).
- **Dependencies**: No new Go module dependencies. Uses existing `os/exec`, `net/http` for JAR download, and existing `ManagedProcess`/`waitForServices` infrastructure.
- **UX**: `install otel-java` becomes a fully automated, interactive installer. `install otel` project selection shows Java projects by default (no longer gated).
- **Token scope**: Reuses existing `DT_ACCESS_TOKEN` for OTLP ingest and `DT_PLATFORM_TOKEN` for DQL verification — same scopes as Python.
- **Non-regression**: Existing `dtwiz install otel` flow for Python must remain functional. The `setup` flow's multi-runtime detection continues to work with Java now enabled.

## Rollback Plan

All changes are additive. To roll back:

1. **Revert `pkg/installer/otel_java.go`** — restore the manual-instructions-only `InstallOtelJava`.
2. **Delete `pkg/installer/otel_java_process.go`** — all process detection/reconstruction logic is contained here.
3. **Revert `cmd/install.go`** — remove `platformToken` from `InstallOtelJava` call.
4. **Revert `pkg/installer/otel.go`** — re-gate Java behind `DTWIZ_ALL_RUNTIMES`.
5. No database, config, or external service changes are involved.
