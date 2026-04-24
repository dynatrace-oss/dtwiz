# Spec: Java Auto-Instrumentation

## ADDED Requirements

### Requirement: Agent JAR download

The installer SHALL automatically download the OpenTelemetry Java agent JAR from the official GitHub releases to a well-known location.

#### Scenario: First-time download

- **WHEN** `dtwiz install otel-java` is run and `~/opentelemetry/java/opentelemetry-javaagent.jar` does not exist
- **THEN** the installer SHALL create the directory `~/opentelemetry/java/` if it does not exist
- **AND** SHALL download the JAR from `https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar`
- **AND** SHALL place it at `~/opentelemetry/java/opentelemetry-javaagent.jar`

#### Scenario: JAR already exists

- **WHEN** `~/opentelemetry/java/opentelemetry-javaagent.jar` already exists
- **THEN** the installer SHALL re-download the JAR (the "latest" URL may point to a newer version)

#### Scenario: Download failure

- **WHEN** the download fails (network error, non-200 HTTP status)
- **THEN** the installer SHALL exit with a clear error message including the URL and the HTTP status or error

### Requirement: Java entrypoint detection

The installer SHALL detect runnable entrypoints for the selected Java project without requiring the application to already be running.

#### Scenario: Fat JAR found in Maven target directory

- **GIVEN** the selected project has a `pom.xml`
- **WHEN** the installer scans for entrypoints
- **THEN** it SHALL look for `*.jar` files in `target/` that contain a `Main-Class` entry in their `MANIFEST.MF`
- **AND** SHALL offer the discovered JAR(s) as launch candidates

#### Scenario: Fat JAR found in Gradle build directory

- **GIVEN** the selected project has a `build.gradle` or `build.gradle.kts`
- **WHEN** the installer scans for entrypoints
- **THEN** it SHALL look for `*.jar` files in `build/libs/` that contain a `Main-Class` entry in their `MANIFEST.MF`
- **AND** SHALL offer the discovered JAR(s) as launch candidates

#### Scenario: Build-tool wrapper available — Spring Boot Maven project

- **GIVEN** the project has a `mvnw` or `mvn` wrapper but no built JAR artifact
- **AND** `pom.xml` contains a reference to `spring-boot`
- **WHEN** the installer scans for entrypoints
- **THEN** it SHALL offer `./mvnw spring-boot:run` as a candidate

#### Scenario: Build-tool wrapper available — Spring Boot Gradle project

- **GIVEN** the project has a `gradlew` or `gradle` wrapper but no built JAR artifact
- **AND** `build.gradle` or `build.gradle.kts` references `springframework.boot` or `spring-boot`
- **WHEN** the installer scans for entrypoints
- **THEN** it SHALL offer `./gradlew bootRun` as a candidate

#### Scenario: Build-tool wrapper available — non-Spring Boot Gradle project

- **GIVEN** the project has a `gradlew` or `gradle` wrapper but no built JAR artifact
- **AND** the Gradle build file does not reference Spring Boot
- **WHEN** the installer scans for entrypoints
- **THEN** it SHALL offer `./gradlew run` as a candidate

#### Scenario: Build-tool wrapper available — non-Spring Boot Maven project

- **GIVEN** the project has a `mvnw` or `mvn` wrapper but no built JAR artifact
- **AND** `pom.xml` does not reference Spring Boot
- **WHEN** the installer scans for entrypoints
- **THEN** NO wrapper candidate SHALL be offered (generic `exec:java` requires `mainClass` POM config absent in most projects)
- **AND** the installer SHALL print build instructions and exit (see "No entrypoint found" scenario)

#### Scenario: Single entrypoint candidate

- **GIVEN** exactly one runnable JAR or wrapper is found in the project
- **WHEN** the entrypoint selection step runs
- **THEN** the installer SHALL auto-select the single entrypoint without prompting the user
- **AND** SHALL print the selected entrypoint's description and command for transparency

#### Scenario: Multiple entrypoint candidates

- **GIVEN** multiple runnable JARs or wrappers are found in the project
- **WHEN** the entrypoint selection step runs
- **THEN** the installer SHALL present a numbered menu for the user to select one
- **AND** SHALL allow the user to skip (which falls back to manual instructions)

#### Scenario: No entrypoint found — auto-build attempted

- **WHEN** no built JAR with a `Main-Class` and no build-tool wrapper is found in the project
- **AND** a Maven or Gradle wrapper (`mvnw`, `mvn`, `gradlew`, `gradle`) is present
- **THEN** the installer SHALL attempt an auto-build (`./mvnw clean package -DskipTests` or `./gradlew build -x test`)
- **AND** SHALL print the build command before running it
- **AND** if the build succeeds SHALL re-scan for entrypoints and continue normally
- **AND** if the build fails SHALL print `Auto-build failed: <error>` and direct the user to fix the build error and re-run

#### Scenario: No entrypoint found — no build tool available

- **WHEN** no built JAR with a `Main-Class` is found and no Maven or Gradle wrapper is present
- **THEN** the installer SHALL inform the user that no runnable entrypoint was detected and no build tool is available
- **AND** SHALL NOT attempt to start any process

### Requirement: Entrypoint detection debug logging

The installer SHALL emit debug-level log lines throughout the entrypoint detection pipeline so that users running with `--debug` can understand exactly what was scanned, what was found, and why a candidate was accepted or rejected.

#### Scenario: JAR scan — directory not found

- **WHEN** `target/` (Maven) or `build/libs/` (Gradle) does not exist in the project directory
- **THEN** a debug line SHALL be emitted: `"<dir> not found, skipping JAR scan" dir=<path>`

#### Scenario: JAR scan — candidate accepted

- **WHEN** a JAR file is found and `isExecutableJar` returns true
- **THEN** a debug line SHALL be emitted: `"executable JAR found" jar=<path>`

#### Scenario: JAR scan — candidate rejected

- **WHEN** a JAR file is found but `isExecutableJar` returns false (no `Main-Class` in `MANIFEST.MF`)
- **THEN** a debug line SHALL be emitted: `"skipping JAR — no Main-Class in MANIFEST.MF" jar=<path>`

#### Scenario: Spring Boot detection

- **WHEN** `isSpringBootMaven` or `isSpringBootGradle` is evaluated
- **THEN** a debug line SHALL be emitted: `"Spring Boot detection" file=<path> result=<true|false>`

#### Scenario: Wrapper fallback chosen

- **WHEN** no fat JAR was found and a build-tool wrapper candidate is selected
- **THEN** a debug line SHALL be emitted: `"no fat JAR found, using wrapper fallback" command=<command>`

#### Scenario: No entrypoint found

- **WHEN** `detectJavaEntrypoints` returns an empty slice
- **THEN** a debug line SHALL be emitted: `"no entrypoint found" project=<path> scanned=<list of dirs/files checked>`

#### Scenario: Entrypoint auto-selected

- **WHEN** exactly one candidate is found and auto-selected
- **THEN** a debug line SHALL be emitted: `"auto-selected single entrypoint" command=<command>`

#### Scenario: Auto-build triggered

- **WHEN** no entrypoint is found and an auto-build is attempted
- **THEN** a debug line SHALL be emitted: `"attempting auto-build" command=<build command> project=<path>`

#### Scenario: Auto-build result

- **WHEN** the auto-build completes
- **THEN** a debug line SHALL be emitted: `"auto-build succeeded" project=<path>` on success, or `"auto-build failed" project=<path> error=<error>` on failure

---

### Requirement: Instrumented process launch

The installer SHALL stop any running instance of the selected project and start the application fresh with the `-javaagent` flag and OTEL_* environment variables configured for Dynatrace.

#### Scenario: Successfully instrumented launch from project artifact

- **GIVEN** the user has selected a project and a launch entrypoint and confirmed the plan
- **WHEN** the installer executes the plan
- **THEN** any running processes matched to the project SHALL be stopped first (SIGINT, then SIGKILL after timeout)
- **AND** a new process SHALL be started using the resolved entrypoint command with `-javaagent:~/opentelemetry/java/opentelemetry-javaagent.jar` prepended to the JVM flags
- **AND** the process SHALL inherit OTEL_* environment variables: `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`, `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`, `OTEL_LOGS_EXPORTER`

#### Scenario: Process crash after instrumented launch

- **GIVEN** the instrumented process is launched
- **WHEN** the process crashes within the startup settle window (2 seconds)
- **THEN** the summary line SHALL show `[crashed: <exit error> — check log for details]` and the log filename
- **AND** the Dynatrace traffic-waiting prompt SHALL NOT be shown

#### Scenario: Process starts successfully

- **GIVEN** the instrumented process is launched
- **WHEN** the process is still running after the settle period and has bound a TCP port
- **THEN** the summary line SHALL show the listening URL
- **AND** the installer SHALL proceed to OTel Collector update and Dynatrace verification

### Requirement: OTel Collector config update

After launching the instrumented Java process, the installer SHALL update the local OTel Collector configuration if one exists.

#### Scenario: OTel Collector config found

- **GIVEN** the dtwiz well-known collector config path (`<cwd>/opentelemetry/config.yaml`) exists on the machine
- **WHEN** the instrumented Java process has been started successfully
- **THEN** the installer SHALL patch the collector config silently using `PatchConfigFile` — no interactive prompt, no restart
- **AND** SHALL print a single summary line indicating the config was updated

#### Scenario: No OTel Collector config found

- **GIVEN** the dtwiz well-known collector config path does not exist
- **WHEN** the installer reaches the collector update step
- **THEN** the step SHALL be skipped silently with no output
- **AND** the Java agent SHALL export directly to Dynatrace via OTLP without a local collector

### Requirement: Dynatrace verification via DQL

After launching the instrumented process, the installer SHALL verify the service appears in Dynatrace.

#### Scenario: Service appears in Dynatrace

- **GIVEN** an instrumented Java process is running and sending telemetry
- **WHEN** the installer polls DQL for the service name
- **THEN** the installer SHALL print a confirmation when the service appears: `✓ "<service-name>" appeared in Dynatrace`

#### Scenario: Service does not appear within timeout

- **GIVEN** an instrumented Java process is running
- **WHEN** the service does not appear in Dynatrace within 120 seconds
- **THEN** the installer SHALL print a timeout message indicating the service may take more time to appear

#### Scenario: DQL verification uses access token with Bearer auth

- **GIVEN** the installer reaches the DQL verification step
- **WHEN** `waitForServices` is called
- **THEN** the access token SHALL be used with Bearer auth for the DQL endpoint

#### Scenario: All processes crashed

- **GIVEN** the instrumented process crashed during startup
- **WHEN** the installer checks for alive processes
- **THEN** DQL verification SHALL be skipped
- **AND** the installer SHALL print: "No services are running — check the logs above for errors."

### Requirement: Plan preview and confirmation

The installer SHALL show a compact preview of all actions before execution and require user confirmation.

#### Scenario: Plan preview contents — no running process

- **GIVEN** a Java project and entrypoint have been selected and no running process is matched to the project
- **WHEN** the plan preview is displayed
- **THEN** it SHALL show: the project path, the resolved launch command (with `-javaagent` inserted), the agent JAR download URL, and the OTEL_* environment variables
- **AND** if the project is multi-module, the preview SHALL show the build command (if build is needed) and the per-module launch command for each sub-module instead of a single launch command
- **AND** the preview SHALL never show `your_app.jar` — the actual resolved command SHALL always be shown

#### Scenario: Plan preview contents — running process will be stopped

- **GIVEN** a Java project and entrypoint have been selected and one or more running processes are matched to the project
- **WHEN** the plan preview is displayed
- **THEN** it SHALL explicitly list the PID(s) and process description(s) that will be stopped
- **AND** the preview SHALL make clear that stopping the process will cause a brief interruption
- **SO THAT** the user is not surprised when their running application goes down

#### Scenario: User confirms — no running process

- **WHEN** no running process is matched to the selected project
- **AND** the user confirms the plan (Enter or "y")
- **THEN** execution SHALL proceed

#### Scenario: User confirms — running process will be stopped

- **WHEN** one or more running processes are matched to the selected project
- **THEN** the confirmation prompt SHALL name the process(es) being stopped, e.g. `Stop PID 1234 (myapp) and proceed with installation?`
- **AND** execution SHALL only proceed after the user confirms

#### Scenario: User cancels

- **WHEN** the user enters "n" at the confirmation prompt
- **THEN** the installer SHALL exit without making any changes

### Requirement: Dry-run support

`dtwiz install otel-java --dry-run` SHALL show what would be done without executing any changes.

#### Scenario: Dry-run output

- **WHEN** `--dry-run` is passed
- **THEN** the installer SHALL print the API URL, service name, agent JAR download URL, environment variables, and the `-javaagent` JVM flag
- **AND** SHALL NOT download the JAR, stop any processes, or start any processes

### Requirement: Java enabled by default in multi-runtime selection

Java SHALL no longer be gated behind the `DTWIZ_ALL_RUNTIMES` feature flag once all implementation tasks are complete. During development, the flag continues to hide Java from `dtwiz install otel` — removing the gate is the final step before release.

#### Scenario: Java appears in `dtwiz install otel` project list

- **GIVEN** `java` is in PATH
- **WHEN** `dtwiz install otel` scans for projects
- **THEN** Java projects SHALL appear in the multi-runtime selection menu alongside Python projects
- **AND** `DTWIZ_ALL_RUNTIMES` SHALL NOT be required

### Requirement: Log output for instrumented processes

Each instrumented Java process SHALL have its stdout/stderr redirected to a log file.

#### Scenario: Log file created

- **GIVEN** a Java process is launched with instrumentation
- **WHEN** the process starts
- **THEN** a log file SHALL be created at `<project-path>/<service-name>.log`
- **AND** the process's stdout and stderr SHALL be redirected to this file

#### Scenario: Log file referenced in summary

- **GIVEN** the process summary is printed after launch
- **WHEN** the summary line is displayed
- **THEN** it SHALL include `[log: <filename>]` so the user knows where to find output

### Requirement: Waiting for traffic terminates on detection

The "Waiting for traffic" prompt SHALL terminate when traces/logs land in Dynatrace, not only on timeout.

#### Scenario: Traces detected before timeout

- **GIVEN** the instrumented Java service is sending telemetry
- **WHEN** `waitForServices()` detects the service in Dynatrace via DQL
- **THEN** the waiting prompt SHALL terminate immediately
- **AND** SHALL print: `All services are reporting to Dynatrace.`

### Requirement: Multi-module project detection and instrumentation

When a Java project is detected as a multi-module Maven or Gradle project, the installer SHALL
instrument all sub-modules as independent services rather than attempting to run the root project.

#### Scenario: Maven multi-module project detected

- **GIVEN** the selected project has a `pom.xml` with `<packaging>pom</packaging>` and `<modules>` entries
- **WHEN** the installer analyzes the project
- **THEN** it SHALL detect it as multi-module
- **AND** SHALL resolve each declared `<module>` directory as a separate sub-module

#### Scenario: Gradle multi-project detected

- **GIVEN** the selected project has a `settings.gradle` or `settings.gradle.kts` with one or more `include` directives
- **WHEN** the installer analyzes the project
- **THEN** it SHALL detect it as a multi-project build
- **AND** SHALL resolve each included sub-project path as a separate sub-module

#### Scenario: Plan preview for multi-module project

- **GIVEN** a multi-module project has been selected
- **WHEN** the plan preview is displayed
- **THEN** it SHALL show the build command (if build is needed)
- **AND** SHALL show the instrumented launch command for each sub-module with its service name
- **AND** SHALL show the OTEL_* env vars for each sub-module (with distinct `OTEL_SERVICE_NAME` values)

#### Scenario: JARs already built — skip build step

- **GIVEN** all sub-modules have fat JARs in their `target/` (Maven) or `build/libs/` (Gradle) directories
- **WHEN** the installer checks for build necessity
- **THEN** the build step SHALL be skipped
- **AND** the plan preview SHALL NOT show a build command

#### Scenario: Auto-build when JARs are missing

- **GIVEN** at least one sub-module is missing a fat JAR
- **WHEN** the user confirms the plan
- **THEN** the installer SHALL run the build command at the project root (e.g. `./mvnw clean package -DskipTests`)
- **AND** SHALL abort with an error message if the build fails, instructing the user to fix the build error
- **AND** SHALL proceed to launch all modules after a successful build

#### Scenario: No build wrapper available and JARs missing

- **GIVEN** sub-modules are missing JARs AND no `mvnw`, `mvn`, `gradlew`, or `gradle` wrapper is found
- **THEN** the installer SHALL print a message instructing the user to build the project manually and exit

#### Scenario: All sub-modules launched as independent services

- **GIVEN** the plan is confirmed and build (if needed) succeeds
- **WHEN** the installer executes the plan
- **THEN** each sub-module's fat JAR SHALL be started as a separate process with `-javaagent` and its own `OTEL_SERVICE_NAME`
- **AND** each process SHALL have its own log file at `<sub-module-path>/<sub-module-name>.log`

#### Scenario: Partial failure — some modules crash at startup

- **GIVEN** some sub-modules crash during the settle period
- **WHEN** the process summary is printed
- **THEN** crashed modules SHALL show `[crashed: <exit error>]` in the summary
- **AND** the installer SHALL continue with the surviving modules
- **AND** DQL verification SHALL only track the surviving services

### Requirement: Entrypoint resolved before preview

The installer SHALL resolve the launch entrypoint before showing the plan preview, so the user
can see exactly what command will be executed.

#### Scenario: Entrypoint resolution in multi-runtime flow

- **GIVEN** `dtwiz install otel` (multi-runtime) selects a Java project
- **WHEN** the plan is built
- **THEN** the installer SHALL detect entrypoints (or multi-module structure) immediately
- **AND** the preview SHALL show the resolved launch command — never a placeholder like `java -javaagent:... -jar your_app.jar`

### Requirement: Uninstall Java instrumentation via `dtwiz uninstall otel`

`dtwiz uninstall otel` SHALL be extended to also stop all Java processes instrumented by dtwiz and remove the downloaded OTel agent JAR directory, as an additional section alongside the existing OTel Collector cleanup. No separate `dtwiz uninstall otel-java` command is added.

#### Scenario: Instrumented processes found

- **GIVEN** one or more Java processes are running with `-javaagent:...opentelemetry-javaagent.jar` in their command line
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** the preview SHALL include a Java instrumentation section listing the PIDs and process descriptions to be stopped and the agent directory to be removed
- **AND** SHALL prompt for confirmation (once, covering all sections in the preview) before making any changes
- **AND** upon confirmation SHALL stop all matched Java processes (SIGINT → SIGKILL fallback)
- **AND** SHALL remove `~/opentelemetry/java/` if it exists

#### Scenario: No instrumented processes, but agent JAR exists

- **GIVEN** no Java processes with the agent flag are running
- **AND** `~/opentelemetry/java/` exists on disk
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** the preview SHALL include a Java section showing only the directory removal
- **AND** SHALL remove `~/opentelemetry/java/` after confirmation

#### Scenario: Nothing Java-related to remove

- **GIVEN** no instrumented Java processes are running
- **AND** `~/opentelemetry/java/` does not exist
- **WHEN** `dtwiz uninstall otel` is run
- **THEN** the Java section SHALL be absent from the preview and output
- **AND** existing OTel Collector cleanup behavior SHALL be unchanged

#### Scenario: Dry-run

- **WHEN** `dtwiz uninstall otel --dry-run` is run
- **THEN** the preview SHALL include the Java section (processes and/or directory) if applicable, without stopping anything or removing any files

#### Scenario: Only dtwiz-instrumented processes are stopped

- **GIVEN** multiple Java processes are running
- **WHEN** `dtwiz uninstall otel` discovers processes
- **THEN** it SHALL only include processes whose command line contains the exact dtwiz agent path (`~/opentelemetry/java/opentelemetry-javaagent.jar`)
- **AND** the preview SHALL note that the list is best-effort and ask the user to verify before confirming
- **AND** SHALL NOT stop processes that do not reference that specific path
