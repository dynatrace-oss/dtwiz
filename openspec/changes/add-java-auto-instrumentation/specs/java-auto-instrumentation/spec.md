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

#### Scenario: Build-tool wrapper available

- **GIVEN** the project has a `mvnw` or `gradlew` wrapper but no built JAR artifact
- **WHEN** the installer scans for entrypoints
- **THEN** it SHALL offer a build-tool run command as a candidate (e.g., `./mvnw exec:java` or `./gradlew run`)

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

#### Scenario: No entrypoint found

- **WHEN** no built JAR with a `Main-Class` and no build-tool wrapper is found in the project
- **THEN** the installer SHALL inform the user that no runnable entrypoint was detected
- **AND** SHALL print build instructions (e.g., `mvn package` or `./gradlew build`) along with manual `-javaagent` instructions
- **AND** SHALL NOT attempt to start any process

### Requirement: Instrumented process launch

The installer SHALL stop any running instance of the selected project and start the application fresh with the `-javaagent` flag and OTEL_* environment variables configured for Dynatrace.

#### Scenario: Successful instrumented launch from project artifact

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

#### Scenario: No platform token provided

- **GIVEN** `platformToken` is empty
- **WHEN** the installer reaches the DQL verification step
- **THEN** DQL verification SHALL be skipped
- **AND** the installer SHALL print: `Skipping Dynatrace verification — no platform token provided (set DT_PLATFORM_TOKEN to enable)`

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

Java SHALL no longer be gated behind the `DTWIZ_ALL_RUNTIMES` feature flag.

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
