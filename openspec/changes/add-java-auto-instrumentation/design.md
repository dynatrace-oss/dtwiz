# Design

## Context

`dtwiz install otel-java` currently exists as a stub that prints manual instructions — download the agent JAR, set env vars, and run with `-javaagent`.

Java auto-instrumentation works by attaching a single agent JAR at JVM startup via the `-javaagent` flag. There are no packages to install — the agent is a single JAR added to the JVM command line. The key challenge is determining *how* to launch the application: from a built artifact (fat JAR) or a build-tool wrapper.

The installer follows a project-first flow:
1. Scan for Java projects on disk.
2. Match any already-running processes to those projects (informational — shows which are live).
3. User selects a project.
4. Resolve the launch entrypoint for that project.
5. Download the agent JAR.
6. Stop any running instance of the project.
7. Launch the application fresh with `-javaagent` and `OTEL_*` env vars.
8. Update the OTel Collector config if one exists.
9. Verify the service appears in Dynatrace via DQL.

Existing infrastructure to reuse:

- `detectProcesses("java", nil)` in `otel_runtime_scan_unix.go` already detects Java processes.
- `ManagedProcess`, `StartManagedProcess`, `PrintProcessSummary` in `otel_process.go` handle process lifecycle.
- `waitForServices()` in `otel_env.go` polls DQL for service entities.
- `generateBaseOtelEnvVars()` in `otel_env.go` generates the OTEL_* env vars (already includes `OTEL_EXPORTER_OTLP_PROTOCOL` and `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`).
- `scanProjectDirs()` with `javaProjectMarkers` already detects Java projects.
- `confirmProceed()` for the UX confirmation pattern.
- `updateOtelCollectorConfig()` / `UpdateOtelCollector()` in `otel_update.go` for patching the collector config.

## Goals / Non-Goals

**Goals:**

- Implement a fully automated `InstallOtelJava()`: project-first, no running process required.
- Detect Java projects on disk via `scanProjectDirs` and resolve a launch entrypoint (built JAR artifact or build-tool wrapper).
- Pre-flight validation: Java in PATH, version >= 8.
- Parse `java -version` output reliably across JDK vendors and versions (Oracle, OpenJDK, GraalVM, etc.).
- Match already-running Java processes to detected projects so the UI can show which are live.
- Stop any running instance of the selected project before launching the instrumented one.
- Pass `platformToken` through `InstallOtelJava` for DQL verification via `waitForServices()`.
- Update the OTel Collector config (if present) after instrumentation is applied.
- Remove the `DTWIZ_ALL_RUNTIMES` gate so Java appears in `dtwiz install otel` by default — **done last, after all other tasks are implemented and verified**.
- Support `--dry-run`.

**Non-Goals:**

- Full Maven/Gradle build invocation — if no built artifact exists yet, inform the user and print manual build instructions; do not invoke `mvn package` or `gradle build`.
- Instrumenting Java processes running inside Docker containers or managed by systemd (warn and skip).
- Persistent configuration management (tracking which processes were instrumented).
- Supporting GraalVM native images (no JVM, no `-javaagent`).
- Uninstall command for Java (the user stops the process manually).
- Reconstructing a launch command from a running process's `ps` output — this approach is unreliable due to missing working directory, truncated args, and absent environment variables.

## Decisions

### 1. Project-first flow

`detectJavaProjects()` scans for `pom.xml`, `build.gradle`, `build.gradle.kts`, `gradlew`, `.mvn`.
- `detectJavaProcesses()` detects running Java processes; `matchProcessesToProjects()` associates PIDs with projects.
- User selects a project from the list (which shows live PIDs if any).
- `detectJavaEntrypoints()` inspects the project directory for runnable artifacts.

A running process is **not required** — it is only used as supplemental information (which project is currently live).

**Alternative considered:** Require a running process as the primary input. Rejected because it breaks the zero-friction goal when the app is not running.

### 2. Java entrypoint detection

The installer inspects the project directory for runnable artifacts in priority order:

| Priority | Source | Example |
|---|---|---|
| 1 | Fat JAR in `target/` (Maven) | `target/myapp-1.0-SNAPSHOT.jar` (with `Main-Class` in manifest) |
| 2 | Fat JAR in `build/libs/` (Gradle) | `build/libs/myapp-all.jar` |
| 3 | Build-tool wrapper + run goal | `./mvnw exec:java -Dexec.mainClass=…` or `./gradlew run` |

For fat JARs, the installer checks the `MANIFEST.MF` for a `Main-Class` attribute to confirm the JAR is executable. JARs without `Main-Class` are skipped.

If multiple candidates are found, the user is presented with a numbered menu to select one. If no entrypoint can be resolved, the installer prints manual build instructions and exits.

**Alternative considered:** Use the running process's `ps ax` command line as a fallback entrypoint source. Rejected — `ps` output does not include the working directory, args are truncated on most systems beyond 4096 bytes, and environment variables are not captured. The reconstructed command would silently fail or misbehave for any non-trivial app.

**Alternative considered:** Invoke `mvn package` or `gradle build` automatically. Rejected for the first iteration — building can have side effects (running tests, fetching dependencies) that the user should control. A clear message pointing to `mvn package` or `./gradlew build` is sufficient.

### 3. Agent JAR download to `~/opentelemetry/java/opentelemetry-javaagent.jar`

The agent JAR is downloaded from the official GitHub releases URL (`https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar`) to `~/opentelemetry/java/opentelemetry-javaagent.jar`. This location is:

- Outside any project directory (the JAR is reusable across projects).
- Under the user's home directory (no root/sudo required).
- Consistent with a potential future convention for other runtime agents.

If the file already exists, it is re-downloaded (the "latest" release URL may point to a newer version). The download uses `net/http` with `os.Create` and `io.Copy` — no external dependencies.

**Alternative considered:** Download to the project directory. Rejected because the same JAR is used across all Java projects and downloading per-project wastes disk and complicates cleanup.

### 4. Java version parsing from `java -version` stderr

`java -version` outputs to **stderr** (not stdout) and the format varies by vendor:

```text
openjdk version "1.8.0_382"        → Java 8
java version "17.0.1" 2021-10-19   → Java 17
openjdk version "21" 2023-09-19    → Java 21
```

The parser extracts the quoted version string from any line matching the pattern `version "X.Y.Z…"`. The major version is determined by:

- If the version starts with `1.`, the major version is the second component (e.g., `1.8.0` → 8).
- Otherwise, the first component is the major version (e.g., `17.0.1` → 17, `21` → 21).

This handles all common JDK distributions (Oracle, OpenJDK, Adoptium, Amazon Corretto, GraalVM JDK).

**Alternative considered:** Using `java --version` (double dash). Rejected because `--version` was introduced in Java 9 and fails on Java 8, which we need to support.

### 5. Process detection: enrichment and stop signal

Running process detection via `detectProcesses("java", nil)` serves two purposes:

1. **Enrichment:** Projects with a matched running PID show `← PIDs: 1234` in the selection menu, giving the user a visual cue that the project is live.
2. **Stop before launch:** When executing the plan, any PIDs matched to the selected project are stopped (SIGINT → SIGKILL fallback) before the instrumented process is started.

When `jps` (JDK tool) is available in PATH, it provides richer process descriptions (main class / JAR name) for the stop-step summary line. `jps` is supplemental, never required.

### 6. OTel Collector config update

After the instrumented Java process is started, the installer checks whether an OTel Collector is already configured (same detection as `dtwiz update otel`). If found, it updates the config to ensure the Java service's OTLP pipeline is covered.

If no collector config is found, this step is skipped silently (the Java agent exports directly to Dynatrace via OTLP — no local collector is strictly required).

### 7. Stop-and-restart flow

The instrumented launch requires JVM flags (`-javaagent`) that can only be set at JVM startup. Therefore:

1. Stop any running processes matched to the selected project (SIGINT → SIGKILL fallback).
2. Determine the launch command from entrypoint detection.
3. Start the new process with the launch command, `-javaagent` flag, and `OTEL_*` environment variables.

When a running process is matched, the plan preview explicitly lists the PID(s) and process description(s) that will be stopped, and the `confirmProceed()` prompt names the process(es) being stopped — e.g. `Stop PID 1234 (myapp) and proceed with installation?`. When no process is matched, the prompt is the standard `Proceed with installation?`. The user must confirm before any process is stopped.

### 8. Reuse `ManagedProcess` and `waitForServices` from existing infrastructure

After launching the instrumented process:

- `StartManagedProcess()` handles the lifecycle (PID tracking, log file, exit detection).
- `PrintProcessSummary()` shows status after the settle period (crashed / running / port detected).
- `waitForServices()` polls DQL to verify the service appears in Dynatrace.

### 9. Pass `platformToken` to `InstallOtelJava` for DQL verification

The function signature becomes `InstallOtelJava(envURL, token, platformToken, serviceName string, dryRun bool) error`. The `platformToken` is needed by `waitForServices()` to authenticate against the DQL endpoint. The Cobra command in `cmd/install.go` already resolves `platformTok` via `getDtEnvironment()` — it just needs to be passed through.

### 10. Enable Java by default in `dtwiz install otel`

In `pkg/installer/otel.go`, `detectAvailableRuntimes()` currently gates Java behind `allRuntimesEnabled()`. Once all other tasks are implemented and verified, the `enabled` field for Java is set to `true` unconditionally — removing the gate is the **last code change** before integration testing.

### 11. File layout

| File | Responsibility |
|---|---|
| `otel_java.go` | `InstallOtelJava`, `DetectJavaPlan`, `JavaInstrumentationPlan`, plan/execute flow, JAR download, env var generation |
| `otel_java_process.go` (new) | `parseJavaVersion`, `validateJavaPrerequisites`, Java entrypoint detection (`detectJavaEntrypoints`, `isExecutableJar`), process detection/enrichment via `jps` |
| `otel_java_test.go` | Tests for project detection, plan detection (existing + new) |
| `otel_java_process_test.go` (new) | Tests for version parsing, entrypoint detection, process detection |

## Risks / Trade-offs

- **[No built artifact]** → If the user has not built their project yet (`mvn package` / `gradle build` not run), no fat JAR will be found. Mitigation: detect this case, print a clear message with the build command to run, then exit — don't silently fall through to a broken state.
- **[Process restart is disruptive]** → Stopping and restarting a Java process causes downtime. Mitigation: explicit plan preview and confirmation prompt before execution. `--dry-run` shows the plan without executing.
- **[Java 8 version string format]** → Java 8 uses the old versioning scheme (`1.8.0_382`) while Java 9+ uses the new scheme (`17.0.1`). Mitigation: the parser handles both formats explicitly; unit tests cover all common patterns.
- **[JPS availability]** → `jps` is part of the JDK, not the JRE. Users with only a JRE won't benefit from enriched process names. Mitigation: `jps` is optional.
- **[Large JAR download]** → The agent JAR is ~20MB. Mitigation: show a progress indicator during download.
- **[No uninstall]** → First iteration has no `dtwiz uninstall otel-java`. Mitigation: acceptable for MVP.
