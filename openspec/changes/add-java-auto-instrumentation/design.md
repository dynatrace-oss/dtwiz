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

- Full Maven/Gradle build invocation for single-module projects when no build tool is present — if no built artifact exists and no `mvnw`, `mvn`, `gradlew`, or `gradle` is found, inform the user and exit.
- Instrumenting Java processes running inside Docker containers or managed by systemd (warn and skip).
- Persistent configuration management (tracking which processes were instrumented).
- Supporting GraalVM native images (no JVM, no `-javaagent`).
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
| 3 | Build-tool wrapper (Spring Boot only for Maven) | `./mvnw spring-boot:run` or `./gradlew bootRun` / `./gradlew run` |

For fat JARs, the installer checks the `MANIFEST.MF` for a `Main-Class` attribute to confirm the JAR is executable. JARs without `Main-Class` are skipped.

If exactly one candidate is found, it is auto-selected without prompting — the installer prints the selected entrypoint and proceeds. If multiple candidates are found, the user is presented with a numbered menu to select one. If no entrypoint can be resolved, the installer prints manual build instructions and exits.

**Wrapper fallback rules (only when no fat JAR is found):**

- **Maven:** `./mvnw exec:java` is never offered — it requires an explicit `mainClass` configuration in the exec-maven-plugin that most projects omit, causing a cryptic plugin error with no actionable guidance for the user. Instead, `pom.xml` is checked for `spring-boot`; if found, `./mvnw spring-boot:run` is offered. For non-Spring Boot Maven projects with no built JAR, the "build first" message is shown instead (see Risks).
- **Gradle:** `./gradlew bootRun` is offered when `build.gradle` / `build.gradle.kts` references `springframework.boot` or `spring-boot`. `./gradlew run` is offered for non-Spring Boot Gradle projects (the `run` task is reliably configured by the Gradle Application plugin).

Spring Boot detection is done by substring matching (`spring-boot` in pom.xml, `spring-boot` or `springframework.boot` in Gradle build files) — no deep XML/Groovy parsing required.

**Alternative considered:** Use the running process's `ps ax` command line as a fallback entrypoint source. Rejected — `ps` output does not include the working directory, args are truncated on most systems beyond 4096 bytes, and environment variables are not captured. The reconstructed command would silently fail or misbehave for any non-trivial app.

**Alternative considered:** Invoke `mvn package` or `gradle build` automatically. For single-module projects this is now the default behaviour — if no entrypoint is found, the installer attempts the build, then re-scans. If the build fails, a clear error directing the user to fix it is shown. For both single- and multi-module projects, if no build wrapper is available and JARs are missing, the user is told to build manually and the installer exits.

**Alternative considered:** Offer `./mvnw exec:java` as a generic Maven fallback. Rejected — this command requires `mainClass` to be configured in the exec-maven-plugin POM section, which is absent in the vast majority of projects (including all Spring Boot projects). The result is a cryptic Maven plugin error (`PluginParameterException: mainClass is missing`) with no clear path forward for the user.

### 3. Agent JAR download to `~/opentelemetry/java/opentelemetry-javaagent.jar`

The agent JAR is downloaded from the official GitHub releases URL (`https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar`) to `~/opentelemetry/java/opentelemetry-javaagent.jar`. This location is:

- Outside any project directory (the JAR is reusable across projects).
- Under the user's home directory (no root/sudo required).
- Consistent with a potential future convention for other runtime agents.

If the file already exists, it is re-downloaded (the "latest" release URL may point to a newer version). The download uses `net/http` with `os.Create` and `io.Copy` — no external dependencies.

**Alternative considered:** Download to the project directory. Rejected because the same JAR is used across all Java projects and downloading per-project wastes disk and complicates cleanup.

### 4. `DetectedProcess.Description` field for JPS enrichment

`enrichProcessesWithJPS` needs to attach a human-readable name (main class or JAR name from `jps`) to each detected Java process without losing the original `ps` command line. A new `Description string` field is added to the shared `DetectedProcess` struct in `otel_runtime_scan.go`. Display code in the Java installer uses `Description` when non-empty, falling back to `Command`. This field is ignored by all existing non-Java callers (Python, Node.js, Go).

**Alternative considered:** Return a separate `map[int]string` from `enrichProcessesWithJPS`. Rejected because it requires callers to carry two parallel data structures; embedding in the struct is cleaner and consistent with how `WorkingDirectory` is already attached.

**Alternative considered:** Mutate the `Command` field in-place with the JPS description. Rejected because it destroys the original `ps` output, which is useful for process matching (`matchProcessesToProjects` compares against `Command`).

### 5. Silent OTel Collector config probe after Java launch

After the instrumented Java process starts, the installer calls `updateOtelCollectorIfPresent(envURL, token, dryRun bool)` — a new helper that probes the well-known dtwiz collector config path (`<cwd>/opentelemetry/config.yaml`, matching the path used by `dtwiz install otel`) and patches it silently with `PatchConfigFile` if it exists. If the file does not exist, the step is skipped with no output. No interactive prompt, no error.

**Alternative considered:** Call the existing `UpdateOtelConfig(configPath, ...)` with a discovered path. Rejected because `UpdateOtelConfig` is interactive (shows a diff, asks for confirmation, restarts the collector) — too disruptive as an automatic post-launch step.

### 6. Minimum Java version: 8

The OpenTelemetry Java agent requires Java 8 as its minimum runtime (see [opentelemetry-java-instrumentation requirements](https://github.com/open-telemetry/opentelemetry-java-instrumentation#requirements)). Any JVM older than 8 cannot load the agent JAR and will fail at startup with a cryptic JVM error. The pre-flight check surfaces this before any download or process manipulation happens, giving the user a clear actionable message.

**Alternative considered:** Allow the installer to proceed and let the JVM fail at launch. Rejected — the failure mode is a raw JVM error with no guidance, and the user may have already waited through a JAR download.

### 8. Java version parsing from `java -version` stderr

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

### 9. Process detection: enrichment and stop signal

Running process detection via `detectProcesses("java", nil)` serves two purposes:

1. **Enrichment:** Projects with a matched running PID show `← PIDs: 1234` in the selection menu, giving the user a visual cue that the project is live.
2. **Stop before launch:** When executing the plan, any PIDs matched to the selected project are stopped (SIGINT → SIGKILL fallback) before the instrumented process is started.

When `jps` (JDK tool) is available in PATH, it provides richer process descriptions (main class / JAR name) stored in the new `DetectedProcess.Description` field (see Decision 4). `jps` is supplemental, never required.

### 10. OTel Collector config update

After the instrumented Java process is started, the installer calls `updateOtelCollectorIfPresent(envURL, token, dryRun)`. This helper probes the well-known dtwiz collector config path (`<cwd>/opentelemetry/config.yaml`) and patches it silently with `PatchConfigFile` if found. If not found, the step is skipped with no output (see Decision 5).

### 11. Stop-and-restart flow

The instrumented launch requires JVM flags (`-javaagent`) that can only be set at JVM startup. Therefore:

1. Stop any running processes matched to the selected project (SIGINT → SIGKILL fallback).
2. Determine the launch command from entrypoint detection.
3. Start the new process with the launch command, `-javaagent` flag, and `OTEL_*` environment variables.

When a running process is matched, the plan preview explicitly lists the PID(s) and process description(s) that will be stopped, and the `confirmProceed()` prompt names the process(es) being stopped — e.g. `Stop PID 1234 (myapp) and proceed with installation?`. When no process is matched, the prompt is the standard `Proceed with installation?`. The user must confirm before any process is stopped.

### 12. Reuse `ManagedProcess` and `waitForServices` from existing infrastructure

After launching the instrumented process:

- `StartManagedProcess()` handles the lifecycle (PID tracking, log file, exit detection).
- `PrintProcessSummary()` shows status after the settle period (crashed / running / port detected).
- `waitForServices()` polls DQL to verify the service appears in Dynatrace.

### 13. Pass `platformToken` to `InstallOtelJava` for DQL verification

The function signature becomes `InstallOtelJava(envURL, token, platformToken, serviceName string, dryRun bool) error`. The `platformToken` is needed by `waitForServices()` to authenticate against the DQL endpoint. The Cobra command in `cmd/install.go` already resolves `platformTok` via `getDtEnvironment()` — it just needs to be passed through.

### 14. Enable Java by default in `dtwiz install otel`

In `pkg/installer/otel.go`, `detectAvailableRuntimes()` currently gates Java behind `allRuntimesEnabled()`. Once all other tasks are implemented and verified, the `enabled` field for Java is set to `true` unconditionally — removing the gate is the **last code change** before integration testing.

### 15. File layout

| File | Responsibility |
|---|---|
| `otel_java.go` | `InstallOtelJava`, `DetectJavaPlan`, `JavaInstrumentationPlan`, plan/execute flow, JAR download, env var generation, `updateOtelCollectorIfPresent` |
| `otel_java_process.go` (new) | `parseJavaVersion`, `validateJavaPrerequisites`, Java entrypoint detection (`detectJavaEntrypoints`, `isExecutableJar`), process detection/enrichment via `jps` |
| `otel_runtime_scan.go` | `DetectedProcess` struct — add `Description string` field for JPS enrichment |
| `otel_java_test.go` | Tests for project detection, plan detection (existing + new) |
| `otel_java_process_test.go` (new) | Tests for version parsing, entrypoint detection, process detection |

### 16. Debug logging for entrypoint detection

`detectJavaEntrypoints` and `attemptSingleModuleBuild` emit `logger.Debug` lines at every meaningful branch so users running with `--debug` can trace exactly what was scanned and why a candidate was accepted or rejected. No debug output is produced in normal runs.

Key log points:

| Location | Message pattern |
|---|---|
| Directory not found | `"<dir> not found, skipping JAR scan" dir=<path>` |
| JAR accepted | `"executable JAR found" jar=<path>` |
| JAR rejected (no `Main-Class`) | `"skipping JAR — no Main-Class in MANIFEST.MF" jar=<path>` |
| Spring Boot detection result | `"Spring Boot detection" file=<path> result=<true\|false>` |
| Wrapper fallback chosen | `"no fat JAR found, using wrapper fallback" command=<cmd>` |
| No entrypoint found | `"no entrypoint found" project=<path> scanned=<list>` |
| Single candidate auto-selected | `"auto-selected single entrypoint" command=<cmd>` |
| Auto-build triggered | `"attempting auto-build" command=<cmd> project=<path>` |
| Auto-build result | `"auto-build succeeded"` or `"auto-build failed" error=<err>` |

All messages use structured key=value pairs consistent with the rest of the codebase. The launch debug line in task 5.5 (`"launching instrumented java process"`) remains the final entry point before the process starts.

## Risks / Trade-offs

- **[No built artifact]** → If the user has not built their project yet, no fat JAR will be found. The installer attempts an auto-build (`./mvnw clean package -DskipTests` for Maven, `./gradlew build -x test` for Gradle). If the build fails, a clear message is printed directing the user to fix the build error and re-run. For non-Spring Boot Maven projects with no build wrapper and no built JAR, no wrapper fallback is offered and the user is told to build manually. This applies equally to single-module and multi-module projects.
- **[Process restart is disruptive]** → Stopping and restarting a Java process causes downtime. Mitigation: explicit plan preview and confirmation prompt before execution. `--dry-run` shows the plan without executing.
- **[Java 8 version string format]** → Java 8 uses the old versioning scheme (`1.8.0_382`) while Java 9+ uses the new scheme (`17.0.1`). Mitigation: the parser handles both formats explicitly; unit tests cover all common patterns.
- **[JPS availability]** → `jps` is part of the JDK, not the JRE. Users with only a JRE won't benefit from enriched process names. Mitigation: `jps` is optional.
- **[Large JAR download]** → The agent JAR is ~20MB. Mitigation: show a progress indicator during download.
- **[Uninstall is best-effort]** → `dtwiz uninstall otel-java` identifies processes by the dtwiz agent JAR path, which is a heuristic — another process could independently use the same path. Mitigation: the preview explicitly asks the user to verify the list before confirming; `--dry-run` is supported.
- **[Multi-module build output verbosity]** → Running `./mvnw clean package` streams full Maven output to stdout. Mitigation: acceptable — the user needs to see build progress and errors.
- **[Gradle colon notation]** → Gradle sub-project paths use colon separators (`:api`, `:ui:web`) which are converted to OS filesystem separators. Custom `projectDir` overrides in `settings.gradle` are not supported by the regex parser. Mitigation: the regex handles the common 80% case; projects with custom `projectDir` will have missing modules silently skipped.

### 17. Multi-module project detection

When a project is selected and it is the root of a multi-module Maven or Gradle build, the installer
treats each sub-module as an independent service rather than trying to run the root.

**Maven:** The root `pom.xml` is parsed (via `encoding/xml`) for `<modules><module>` entries. A parent
POM with `<packaging>pom</packaging>` (or absent packaging, which defaults to `pom`) and at least one
`<module>` qualifies as multi-module.

**Gradle:** `settings.gradle` and `settings.gradle.kts` are scanned with a regex for `include '...'` /
`include("...")` directives. Each matched path (converting colon notation to directory separators) is
treated as a sub-project.

**Build step:** If any sub-module is missing a fat JAR, the installer includes a build step in the plan:
`./mvnw clean package -DskipTests` (Maven) or `./gradlew build -x test` (Gradle). The same logic applies to single-module projects — if no fat JAR is found, the installer attempts the build automatically before showing the "no entrypoint" error. The build is run after user confirmation (multi-module) or immediately (single-module, as part of entrypoint resolution before the plan is shown). If the build fails, execution aborts with a clear error message. If no build wrapper is available and JARs are missing, the user is told to build manually and the installer exits.

**Launch:** After a successful build (or if JARs already exist), each sub-module's fat JAR is launched
as a separate `ManagedProcess` with `-javaagent` and a distinct `OTEL_SERVICE_NAME` matching the
sub-module's directory name.

**Alternative considered:** Offer a selection menu for sub-modules (pick which ones to instrument).
Rejected for the initial implementation — "zero config, all defaults on" means all detected modules
are instrumented. The user can kill individual processes after the fact.

**Alternative considered:** Invoke `./mvnw spring-boot:run` for multi-module Spring Boot roots.
Rejected — `spring-boot:run` at a parent POM does not start all modules; it either fails or runs the
first module only. Building fat JARs and launching them individually is the reliable path.

### 18. Entrypoint resolution before preview

In both `createRuntimePlan()` (multi-runtime flow) and `InstallOtelJava()` (standalone), entrypoints
are now resolved before the plan is printed. The preview always shows the exact command that will be
executed — never a placeholder like `java -javaagent:... -jar your_app.jar`. If no entrypoint can be
resolved at plan time, the preview shows `(entrypoint will be detected at execution time)` as a fallback,
but this path is only reached in edge cases where the installer cannot auto-detect the entrypoint.

### 19. Uninstall: full agent path filtering (best-effort)

`dtwiz uninstall otel-java` uses a stateless discover-at-uninstall-time approach, consistent with all other dtwiz uninstall commands. It does not rely on PID files or a process registry.

Running Java processes are discovered via `detectJavaProcesses()` + `enrichProcessesWithJPS()`. The result is filtered to those whose command line contains the **exact agent JAR path** dtwiz uses: `~/opentelemetry/java/opentelemetry-javaagent.jar` (resolved via `javaAgentPath()`). This is a best-effort heuristic — matching on the full path rather than just the filename makes false positives unlikely, but another process could independently use the same agent location. The preview explicitly notes this and asks the user to verify the list before confirming.

The agent JAR directory (`~/opentelemetry/java/`) is removed unconditionally if it exists, regardless of whether any processes were found. This cleans up the downloaded artifact even when the user stopped the process manually.

**Flow:** discover → preview (processes to stop + directory to remove, with caveat) → confirm → stop → remove.

**Alternative considered:** Filter by filename only (`opentelemetry-javaagent.jar`). Rejected — too broad; any process using the OTel Java agent from any location would match.

**Alternative considered:** Match processes to detected projects (the same approach used at install time). Rejected — at uninstall time the user may have moved or deleted the project directory.

**Alternative considered:** Track PIDs in a state file at install time. This would be the only truly reliable approach, but adds statefulness and complexity. Deferred — the full-path heuristic is good enough for the common case, and the preview + confirmation prompt mitigates the risk of stopping the wrong process.
