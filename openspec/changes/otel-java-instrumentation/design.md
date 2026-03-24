## Context

`otel_java.go` exists as a stub with `detectJava()` (finds java, checks version) and `generateOtelJavaEnvVars()` (creates OTEL_* vars + JVM flags). The actual install flow prints a manual download URL and exits. The OpenTelemetry Java agent is a single JAR file (`opentelemetry-javaagent.jar`) that attaches via the `-javaagent:path/to/agent.jar` JVM flag — no source code changes required.

The Python implementation provides a strong pattern to follow: detect projects/processes → user selects → stop processes → install agent → restart with instrumentation → verify in Dynatrace.

## Goals / Non-Goals

**Goals:**
- Automatically download the OpenTelemetry Java agent JAR
- Detect running Java processes and let user select one
- Restart the selected process with `-javaagent` flag and OTEL_* env vars
- Implement uninstall to stop instrumented processes and remove the JAR
- Add pre-flight validation

**Non-Goals:**
- Supporting Java build tool plugins (Maven/Gradle OTel plugins)
- Modifying application source code
- Supporting multiple simultaneous Java process instrumentation (single-app only per meeting notes)

## Decisions

**1. Download JAR to a fixed location**
Download `opentelemetry-javaagent.jar` to `~/opentelemetry/java/opentelemetry-javaagent.jar`. Use the same directory pattern as the collector (`~/opentelemetry/`). Fetch the latest release from `https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest`.

Alternative: Download per-project — rejected, the agent JAR is project-independent.

**2. Process detection via `ps` + `jps`**
Detect running Java processes using `ps ax` filtered for `java` commands. If `jps` is available (ships with JDK), use it as a supplementary source. Show the user a list with PID, main class/JAR, and working directory.

Alternative: Only use `jps` — rejected because it requires JDK (not just JRE) and may not be in PATH.

**3. Restart by re-running the original command with added JVM flags**
Parse the original Java command from `ps`, prepend `-javaagent:` and `OTEL_*` env vars, terminate the old process, run the new command. This is the most straightforward approach since Java agent attachment doesn't require source changes.

Alternative: Dynamic attach via `com.sun.tools.attach` — rejected because it requires JDK tools.jar and is more fragile.

**4. Uninstall kills instrumented processes and removes the JAR directory**
Find processes with `-javaagent:.*opentelemetry-javaagent.jar` in their command line. Kill them. Remove `~/opentelemetry/java/`.

## Risks / Trade-offs

- [Command reconstruction may not capture all JVM args] → Parse carefully from `ps` output. Accept that complex launch scripts (systemd, docker-entrypoint) may not be fully reconstructible — print a warning in those cases.
- [Restarting may lose process context (env vars, ulimits)] → Document this limitation. For production, recommend using the env var approach instead.
- [Agent version pinning] → Always use `latest`. Don't pin versions — matches our zero-config philosophy.
