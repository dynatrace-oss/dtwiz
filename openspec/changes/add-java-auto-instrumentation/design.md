# Design

## Context

`dtwiz install otel-java` currently exists as a stub that prints manual instructions — download the agent JAR, set env vars, and run with `-javaagent`.

Java auto-instrumentation works by attaching a single agent JAR at JVM startup via the `-javaagent` flag. Applications are typically long-running JVM processes started via `java -jar`, classpath-based commands, or build tool wrappers (Maven/Gradle). Because the flag can only be set at startup, instrumenting a running process requires a stop-and-restart cycle. Dependency isolation is handled by the build system, not the installer — there are no virtualenvs to manage.

Existing infrastructure to reuse:
- `detectProcesses("java", nil)` in `otel_runtime_scan_unix.go` already detects Java processes.
- `ManagedProcess`, `StartManagedProcess`, `PrintProcessSummary` in `otel_process.go` handle process lifecycle.
- `waitForServices()` in `otel_env.go` polls DQL for service entities.
- `generateBaseOtelEnvVars()` in `otel_env.go` generates the OTEL_* env vars (already includes `OTEL_EXPORTER_OTLP_PROTOCOL` and `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`).
- `scanProjectDirs()` with `javaProjectMarkers` already detects Java projects.
- `confirmProceed()` for the UX confirmation pattern.

## Goals / Non-Goals

**Goals:**

- Implement a fully automated `InstallOtelJava()` that downloads the agent JAR, detects running Java processes, and restarts the selected process with instrumentation.
- Pre-flight validation: Java in PATH, version >= 8.
- Parse `java -version` output reliably across JDK vendors and versions (Oracle, OpenJDK, GraalVM, etc.).
- Detect running Java processes via `ps ax`, present an interactive selection menu, and reconstruct the selected process's launch command.
- Pass `platformToken` through `InstallOtelJava` for DQL verification via `waitForServices()`.
- Remove the `DTWIZ_ALL_RUNTIMES` gate so Java appears in `dtwiz install otel` by default.
- Support `--dry-run`.

**Non-Goals:**

- Building the application — the user is expected to have their app already built and running. We detect the live JVM process and re-instrument it; we do not invoke `mvn`, `gradle`, or any build tool.
- Instrumenting Java processes running inside Docker containers or managed by systemd (warn and skip).
- Persistent configuration management (tracking which processes were instrumented).
- Supporting GraalVM native images (no JVM, no `-javaagent`).
- Uninstall command for Java (the user stops the process manually).

## Decisions

### 1. Agent JAR download to `~/opentelemetry/java/opentelemetry-javaagent.jar`

The agent JAR is downloaded from the official GitHub releases URL (`https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar`) to `~/opentelemetry/java/opentelemetry-javaagent.jar`. This location is:
- Outside any project directory (the JAR is reusable across projects).
- Under the user's home directory (no root/sudo required).
- Consistent with a potential future convention for other runtime agents.

If the file already exists, it is re-downloaded (the "latest" release URL may point to a newer version). The download uses `net/http` with `os.Create` and `io.Copy` — no external dependencies.

**Alternative considered:** Download to the project directory. Rejected because the same JAR is used across all Java projects and downloading per-project wastes disk and complicates cleanup.

**Alternative considered:** Check the existing JAR version and skip download if up-to-date. Rejected for the first iteration — the latest-redirect URL doesn't expose the version without a HEAD request and parsing, and re-downloading a 20MB JAR is fast enough on modern connections.

### 2. Java version parsing from `java -version` stderr

`java -version` outputs to **stderr** (not stdout) and the format varies by vendor:

```
openjdk version "1.8.0_382"        → Java 8
java version "17.0.1" 2021-10-19   → Java 17
openjdk version "21" 2023-09-19    → Java 21
```

The parser extracts the quoted version string from any line matching the pattern `version "X.Y.Z…"`. The major version is determined by:
- If the version starts with `1.`, the major version is the second component (e.g., `1.8.0` → 8).
- Otherwise, the first component is the major version (e.g., `17.0.1` → 17, `21` → 21).

This handles all common JDK distributions (Oracle, OpenJDK, Adoptium, Amazon Corretto, GraalVM JDK).

**Alternative considered:** Using `java --version` (double dash). Rejected because `--version` was introduced in Java 9 and fails on Java 8, which we need to support.

### 3. Process detection: `ps ax` filtering for `java`, supplemented by `jps`

On Unix/macOS, the existing `detectProcesses("java", nil)` function parses `ps ax -o pid=,command=` and filters for lines containing `java`. This catches all Java processes regardless of how they were started.

When `jps` (JDK tool) is available in PATH, it provides additional metadata: the main class or JAR name, which makes process selection menus more readable. `jps` output is used to **enrich** the `ps`-based results, not replace them, because `jps` only shows JVMs running under the same user with the same Java installation.

On Windows, the existing `detectProcesses` in `otel_runtime_scan_windows.go` uses `wmic`/`tasklist`. The same approach applies — filter for processes with `java` in the command line.

**Alternative considered:** Using `jps` as the primary detection method. Rejected because `jps` only sees JVMs from the same JDK installation and requires the JDK (not JRE) to be installed.

### 4. Command reconstruction from `ps` output

The `ps` command line for a Java process reveals how it was launched. We parse it to reconstruct a restartable command. The supported patterns:

| Pattern | Example | Reconstruction |
|---|---|---|
| `java -jar app.jar` | `java -jar /path/to/app.jar` | Reuse full command, insert `-javaagent:…` before `-jar` |
| Classpath-based | `java -cp lib/*:. com.example.Main` | Insert `-javaagent:…` before `-cp` |
| Module-based | `java -m com.example/com.example.Main` | Insert `-javaagent:…` before `-m` |
| Direct class | `java com.example.Main` | Insert `-javaagent:…` before the class name |

For each pattern, the `-javaagent:/path/to/opentelemetry-javaagent.jar` flag is inserted immediately after `java` (and any existing JVM flags like `-Xmx`).

Complex launch patterns that cannot be reliably reconstructed:
- Wrapper scripts (e.g., `catalina.sh`, `startup.sh`) — the `ps` output shows the script, not the final `java` command.
- Processes where `java` doesn't appear in the command (e.g., custom native launchers).

For these, the installer prints a warning with the manual `-javaagent` instructions and skips automatic restart.

**Alternative considered:** Parsing `/proc/<pid>/cmdline` on Linux for exact argv. This is more reliable than `ps` but not available on macOS. Since we already have `ps ax` working cross-platform (Unix + macOS), we use that consistently.

### 5. Stop-and-restart flow

The instrumented launch requires JVM flags (`-javaagent`) that can only be set at JVM startup. Therefore:

1. Stop the selected process (send `SIGINT`, wait for graceful shutdown, fall back to `SIGKILL` after timeout).
2. Reconstruct the launch command with `-javaagent` flag inserted.
3. Start the new process with the reconstructed command and `OTEL_*` environment variables.

The user sees this in the plan preview and must confirm before execution. This is more invasive than the Python flow (which launches new processes alongside) because JVM instrumentation cannot be applied to a running process.

**Alternative considered:** Launch a parallel instrumented process instead of restarting. Rejected because we'd need the application's full startup context (database connections, ports, etc.) and running two instances would cause port conflicts.

### 6. Reuse `ManagedProcess` and `waitForServices` from existing infrastructure

After launching the instrumented process:
- `StartManagedProcess()` handles the lifecycle (PID tracking, log file, exit detection).
- `PrintProcessSummary()` shows status after the settle period (crashed / running / port detected).
- `waitForServices()` polls DQL to verify the service appears in Dynatrace.

This is exactly the same pattern as the Python installer. No new infrastructure needed.

### 7. Pass `platformToken` to `InstallOtelJava` for DQL verification

The function signature becomes `InstallOtelJava(envURL, token, platformToken, serviceName string, dryRun bool) error`. The `platformToken` is needed by `waitForServices()` to authenticate against the DQL endpoint (Platform URLs use `Bearer` auth). The Cobra command in `cmd/install.go` already resolves `platformTok` via `getDtEnvironment()` — it just needs to be passed through.

### 8. Enable Java by default in `dtwiz install otel`

In `pkg/installer/otel.go`, `detectAvailableRuntimes()` currently gates Java behind `allRuntimesEnabled()` (`DTWIZ_ALL_RUNTIMES=true`). Once the automated installer is complete, the `enabled` field for Java is set to `true` unconditionally, making Java appear in the multi-runtime project selection alongside Python.

### 9. File layout

| File | Responsibility |
|---|---|
| `otel_java.go` | `InstallOtelJava`, `DetectJavaPlan`, `JavaInstrumentationPlan`, plan/execute flow, JAR download, env var generation |
| `otel_java_process.go` (new) | `parseJavaVersion`, `validateJavaPrerequisites`, Java process detection/enrichment via `jps`, command reconstruction, `reconstructJavaCommand` |
| `otel_java_test.go` | Tests for project detection, plan detection (existing + new) |
| `otel_java_process_test.go` (new) | Tests for version parsing, command reconstruction, process detection |

Splitting process logic into its own file keeps `otel_java.go` focused on the install flow and makes the detection/reconstruction logic independently testable.

## Risks / Trade-offs

- **[Command reconstruction reliability]** → Parsing `ps` output to reconstruct a launch command is inherently fragile. Some processes will have been started by wrapper scripts, build tools, or init systems where the actual `java` invocation is hidden. Mitigation: support the three most common patterns (`-jar`, `-cp`, `-m`), warn and print manual instructions for anything else.
- **[Process restart is disruptive]** → Stopping and restarting a Java process causes downtime. Mitigation: explicit plan preview and confirmation prompt before execution. `--dry-run` shows the plan without executing.
- **[Java 8 version string format]** → Java 8 uses the old versioning scheme (`1.8.0_382`) while Java 9+ uses the new scheme (`17.0.1`). Mitigation: the parser handles both formats explicitly; unit tests cover all common patterns.
- **[JPS availability]** → `jps` is part of the JDK, not the JRE. Users with only a JRE won't benefit from the enriched process list. Mitigation: `jps` is optional — `ps`-based detection works without it, just with less readable process names.
- **[Large JAR download]** → The agent JAR is ~20MB. On slow connections this could be noticeable. Mitigation: show a progress indicator during download; skip download if already present (future optimization).
- **[No uninstall]** → First iteration has no `dtwiz uninstall otel-java`. The user must stop the instrumented process and restart without `-javaagent` manually. Mitigation: acceptable for MVP; uninstall can be added in a follow-up when the instrumented-process tracking infrastructure is built.
