# Proposal

## Why

`dtwiz install otel-java` currently prints manual instructions — the user must download the agent JAR themselves, set environment variables by hand, and restart their application. This violates the project's "if we detect it, we enable it" principle. Java is gated behind `DTWIZ_ALL_RUNTIMES` and marked "Coming soon". Making it production-ready closes that gap.

## What Changes

- Implement a fully automated `InstallOtelJava()` flow: detect Java projects on disk, resolve launch entrypoints, download the OpenTelemetry Java agent JAR from GitHub releases, stop any matching running processes, and start the application fresh with `-javaagent` and `OTEL_*` environment variables configured for Dynatrace.
- **`dtwiz uninstall otel` (extended)**: the existing `UninstallOtelCollector()` is extended to also stop all Java processes instrumented by dtwiz (identified by the `-javaagent:...opentelemetry-javaagent.jar` flag in their command line) and remove the downloaded agent JAR directory (`~/opentelemetry/java/`). Java cleanup appears as an additional section in the same preview/confirm/execute flow — mirroring how Node.js cleanup is folded into the same command. Only processes with the agent flag are stopped — unrelated Java processes are not touched.
- **Project-first, not process-first.** The installer scans for Java projects (by presence of `pom.xml`, `build.gradle`, etc.), matches any already-running processes to those projects, and prompts the user to select a project. It then resolves the launch command from the project — it does not require the app to already be running.
- **Java entrypoint detection.** Inspect the selected project to find a runnable artifact: the fat JAR produced by Maven/Gradle (`target/*.jar`, `build/libs/*.jar`). When no built JAR is found, the installer first attempts an auto-build (`./mvnw clean package -DskipTests` for Maven, `./gradlew build -x test` for Gradle) and re-scans. If the build fails, a clear error is printed directing the user to fix it and re-run. If no build tool is present at all, a message is printed and the installer exits. A build-tool wrapper is also offered as a direct launch option (without building) when it can be invoked without additional configuration: `./mvnw spring-boot:run` for Spring Boot Maven projects, `./gradlew bootRun` for Spring Boot Gradle projects, `./gradlew run` for non-Spring Boot Gradle projects. `exec:java` is never offered as it requires `mainClass` POM configuration absent in most projects. When exactly one entrypoint is found, it is auto-selected without prompting. Multiple candidates are presented in a menu.
- **OTel Collector config update.** After instrumenting the Java process, update the existing OTel Collector config (if one is present) to include a Java-specific pipeline or confirm the OTLP receiver is already covering it — mirroring the `dtwiz update otel` behavior.
- Add pre-flight validation: Java in PATH, version >= 8.
- Add Java version parsing that handles all common `java -version` output formats (openjdk version "1.8.0_…", java version "17.0.1", etc.).
- After launching the instrumented process, reuse the existing `ManagedProcess` / `PrintProcessSummary` infrastructure from `otel_process.go` and call `waitForServices()` to verify the service appears in Dynatrace via DQL.

- Remove Java from the `DTWIZ_ALL_RUNTIMES` gate so it appears in `dtwiz install otel` project selection by default.
- `--dry-run` supported for the full flow.

## Capabilities

### New Capabilities

- `java-auto-instrumentation`: Scan for Java projects, detect launch entrypoints, download the OTel Java agent JAR, stop any running instance, start the application with `-javaagent` and OTEL env vars, update the OTel Collector config, and verify traces/logs appear in Dynatrace.
- `java-uninstall`: detect Java processes whose command line references the exact dtwiz agent path (`~/opentelemetry/java/opentelemetry-javaagent.jar`), stop them, and remove `~/opentelemetry/java/`. This logic is folded into the existing `UninstallOtelCollector()` as an additional cleanup section — no separate command is added. The preview explicitly asks the user to verify the list before confirming. Supports `--dry-run`.
- `java-entrypoint-detection`: Locate runnable JAR artifacts and build-tool wrappers in a Java project directory. Support Maven (`target/*.jar`, `./mvnw spring-boot:run` for Spring Boot), Gradle (`build/libs/*.jar`, `./gradlew bootRun` for Spring Boot, `./gradlew run` otherwise), and plain `java -jar` invocations. When no built JAR is found, attempt an auto-build before falling back to an error. Non-Spring Boot Maven projects with no build tool receive a "no build tool detected" message.
- `java-process-detection`: Detect running Java processes from `ps ax` output (and `jps` when available) as an enrichment signal — matched to detected projects to show which projects are currently running and to stop any running instance before relaunch.
- `java-version-validation`: Pre-flight check that Java is on PATH and the version is >= 8. Parse all common `java -version` output formats.

### Modified Capabilities

- `otel-collector-update`: After Java instrumentation is applied, update the OTel Collector config to ensure the Java service's OTLP pipeline is covered (same mechanism as `dtwiz update otel`).

## Impact

- **Code**: New files `pkg/installer/otel_java_process.go` (process detection, command parsing, reconstruction, entrypoint detection), modified `pkg/installer/otel_java.go` (full automated flow replacing manual instructions), modified `pkg/installer/otel_uninstall.go` (extend `UninstallOtelCollector` to include Java process and agent-dir cleanup), modified `cmd/install.go`, modified `pkg/installer/otel.go` (enable Java in default runtimes, update `createRuntimePlan` Java case). No new `cmd/uninstall.go` subcommand is added.
- **Dependencies**: No new Go module dependencies. Uses existing `os/exec`, `net/http` for JAR download, `filepath.Glob` for artifact discovery, and existing `ManagedProcess`/`waitForServices`/collector-update infrastructure.
- **UX**: `install otel-java` becomes a fully automated, interactive installer that starts from project detection — no running process required. `install otel` project selection shows Java projects by default (no longer gated).
- **Token scope**: Reuses existing `DT_ACCESS_TOKEN` for OTLP ingest and DQL verification (Bearer auth).
- **Non-regression**: The `setup` flow's multi-runtime detection continues to work with Java now enabled.

## Rollback Plan

All changes are additive. To roll back:

1. **Revert `pkg/installer/otel_java.go`** — restore the manual-instructions-only `InstallOtelJava`.
2. **Delete `pkg/installer/otel_java_process.go`** — all process detection/reconstruction and entrypoint detection logic is contained here.
3. **Revert `cmd/install.go`** — restore original `InstallOtelJava` call.
4. **Revert `pkg/installer/otel.go`** — re-gate Java behind `DTWIZ_ALL_RUNTIMES`.
5. No database, config, or external service changes are involved.
