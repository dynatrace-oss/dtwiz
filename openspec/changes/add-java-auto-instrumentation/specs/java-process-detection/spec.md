# Spec: Java Process Detection

## ADDED Requirements

### Requirement: Running Java process discovery

The installer SHALL detect running Java processes and present them in an interactive selection menu.

#### Scenario: Java processes detected via ps

- **GIVEN** one or more Java processes are running on the system
- **WHEN** the installer scans for running processes using `ps ax`
- **THEN** all processes whose command line contains `java` SHALL be listed with their PID and command

#### Scenario: JPS enrichment available

- **GIVEN** `jps` is available in PATH (JDK installed)
- **WHEN** the installer detects Java processes
- **THEN** process entries SHALL be enriched with the main class or JAR name from `jps` output for improved readability in the selection menu
- **AND** processes detected by `ps` but not present in `jps` output SHALL still be included

#### Scenario: JPS not available

- **WHEN** `jps` is not found in PATH (JRE-only installation)
- **THEN** the installer SHALL rely solely on `ps ax` for process detection without error or warning

#### Scenario: No Java processes running

- **WHEN** no running Java processes are detected
- **THEN** the installer SHALL inform the user: "No running Java processes detected"
- **AND** SHALL print manual instrumentation instructions (env vars + `-javaagent` flag)

#### Scenario: Process selection

- **GIVEN** multiple Java processes are detected
- **WHEN** the user is presented with the selection menu
- **THEN** each entry SHALL show the PID and a readable description (main class, JAR name, or truncated command)
- **AND** the user SHALL be able to select one process by number or skip

### Requirement: Command reconstruction from ps output

The installer SHALL reconstruct a restartable Java command from the selected process's `ps` output, inserting the `-javaagent` flag and OTEL environment variables.

#### Scenario: java -jar pattern

- **GIVEN** a process command is `java -Xmx512m -jar /path/to/app.jar --port 8080`
- **WHEN** the command is reconstructed
- **THEN** the result SHALL be `java -Xmx512m -javaagent:/path/to/opentelemetry-javaagent.jar -jar /path/to/app.jar --port 8080`
- **AND** existing JVM flags SHALL be preserved in their original position

#### Scenario: Classpath-based pattern

- **GIVEN** a process command is `java -cp lib/*:. com.example.Main`
- **WHEN** the command is reconstructed
- **THEN** the `-javaagent` flag SHALL be inserted before `-cp`
- **AND** the classpath and main class SHALL be preserved

#### Scenario: Module-based pattern

- **GIVEN** a process command is `java -m com.example/com.example.Main`
- **WHEN** the command is reconstructed
- **THEN** the `-javaagent` flag SHALL be inserted before `-m`

#### Scenario: Already instrumented process

- **GIVEN** a process command already contains `-javaagent:` pointing to `opentelemetry-javaagent.jar`
- **WHEN** the command is reconstructed
- **THEN** the existing `-javaagent` flag SHALL be replaced with the new path rather than adding a duplicate

#### Scenario: Unrecognized launch pattern

- **GIVEN** a process command does not match any known Java launch pattern (e.g., it was started via a wrapper script like `catalina.sh` or a systemd service)
- **WHEN** command reconstruction is attempted
- **THEN** the installer SHALL print a warning: "Cannot reconstruct launch command for this process"
- **AND** SHALL print manual `-javaagent` instructions for the user to apply themselves
- **AND** SHALL NOT attempt an automatic restart

### Requirement: Windows process detection

On Windows, the installer SHALL detect Java processes using OS-appropriate mechanisms.

#### Scenario: Windows process listing

- **GIVEN** the installer is running on Windows
- **WHEN** Java processes are scanned
- **THEN** the existing `detectProcesses` Windows implementation SHALL be used to find processes with `java` in the command line
- **AND** the selection menu and command reconstruction SHALL work the same as on Unix/macOS
