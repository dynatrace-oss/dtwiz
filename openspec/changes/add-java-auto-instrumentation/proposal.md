# Proposal

## Why

`dtwiz install otel-java` currently prints manual instructions — the user must download the agent JAR themselves, set environment variables by hand, and restart their application. This violates the project's "if we detect it, we enable it" principle. Java is gated behind `DTWIZ_ALL_RUNTIMES` and marked "Coming soon". Making it production-ready closes that gap.

## What Changes

- Implement a fully automated `InstallOtelJava()` flow: detect Java projects on disk, resolve launch entrypoints, download the OpenTelemetry Java agent JAR from GitHub releases, stop any matching running processes, and start the application fresh with `-javaagent` and `OTEL_*` environment variables configured for Dynatrace.
- **Project-first, not process-first.** The installer scans for Java projects (by presence of `pom.xml`, `build.gradle`, etc.), matches any already-running processes to those projects, and prompts the user to select a project. It then resolves the launch command from the project — it does not require the app to already be running.
- **Java entrypoint detection.** Inspect the selected project to find a runnable artifact: the fat JAR produced by Maven/Gradle (`target/*.jar`, `build/libs/*.jar`), a `mvnw`/`gradlew` wrapper, or a build-tool `run` goal. Present the candidates to the user if more than one is found.
- **OTel Collector config update.** After instrumenting the Java process, update the existing OTel Collector config (if one is present) to include a Java-specific pipeline or confirm the OTLP receiver is already covering it — mirroring the `dtwiz update otel` behavior.
- Add pre-flight validation: Java in PATH, version >= 8.
- Add Java version parsing that handles all common `java -version` output formats (openjdk version "1.8.0_…", java version "17.0.1", etc.).
- After launching the instrumented process, reuse the existing `ManagedProcess` / `PrintProcessSummary` infrastructure from `otel_process.go` and call `waitForServices()` to verify the service appears in Dynatrace via DQL.
- Extend `InstallOtelJava` signature to accept `platformToken` for DQL verification.
- Remove Java from the `DTWIZ_ALL_RUNTIMES` gate so it appears in `dtwiz install otel` project selection by default.
- `--dry-run` supported for the full flow.

## Capabilities

### New Capabilities

- `java-auto-instrumentation`: Scan for Java projects, detect launch entrypoints, download the OTel Java agent JAR, stop any running instance, start the application with `-javaagent` and OTEL env vars, update the OTel Collector config, and verify traces/logs appear in Dynatrace.
- `java-entrypoint-detection`: Locate runnable JAR artifacts and build-tool wrappers in a Java project directory. Support Maven (`target/*.jar`, `mvnw package && mvnw exec:java`), Gradle (`build/libs/*.jar`, `gradlew run`), and plain `java -jar` invocations.
- `java-process-detection`: Detect running Java processes from `ps ax` output (and `jps` when available) as an enrichment signal — matched to detected projects to show which projects are currently running and to stop any running instance before relaunch.
- `java-version-validation`: Pre-flight check that Java is on PATH and the version is >= 8. Parse all common `java -version` output formats.

### Modified Capabilities

- `otel-collector-update`: After Java instrumentation is applied, update the OTel Collector config to ensure the Java service's OTLP pipeline is covered (same mechanism as `dtwiz update otel`).

## Impact

- **Code**: New files `pkg/installer/otel_java_process.go` (process detection, command parsing, reconstruction, entrypoint detection), modified `pkg/installer/otel_java.go` (full automated flow replacing manual instructions), modified `cmd/install.go` (pass platformToken to `InstallOtelJava`), modified `pkg/installer/otel.go` (enable Java in default runtimes, update `createRuntimePlan` Java case).
- **Dependencies**: No new Go module dependencies. Uses existing `os/exec`, `net/http` for JAR download, `filepath.Glob` for artifact discovery, and existing `ManagedProcess`/`waitForServices`/collector-update infrastructure.
- **UX**: `install otel-java` becomes a fully automated, interactive installer that starts from project detection — no running process required. `install otel` project selection shows Java projects by default (no longer gated).
- **Token scope**: Reuses existing `DT_ACCESS_TOKEN` for OTLP ingest and `DT_PLATFORM_TOKEN` for DQL verification.
- **Non-regression**: The `setup` flow's multi-runtime detection continues to work with Java now enabled.

## Rollback Plan

All changes are additive. To roll back:

1. **Revert `pkg/installer/otel_java.go`** — restore the manual-instructions-only `InstallOtelJava`.
2. **Delete `pkg/installer/otel_java_process.go`** — all process detection/reconstruction and entrypoint detection logic is contained here.
3. **Revert `cmd/install.go`** — remove `platformToken` from `InstallOtelJava` call.
4. **Revert `pkg/installer/otel.go`** — re-gate Java behind `DTWIZ_ALL_RUNTIMES`.
5. No database, config, or external service changes are involved.
