# Java Runtime Detection

## ADDED Requirements

### Requirement: Java runtime detection

The system SHALL detect Java installations by looking up `java` on PATH and verifying the version via `java -version`.

#### Scenario: Java is available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `java` is found on PATH
- **THEN** the system proceeds with project scanning

#### Scenario: Java is not available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `java` is not found on PATH
- **THEN** the system silently skips Java detection

### Requirement: Java project scanning

The system SHALL scan the filesystem for Java project markers (`pom.xml`, `build.gradle`, `build.gradle.kts`) starting from the current directory and common project locations.

#### Scenario: Maven project detected

- **GIVEN** Java is available on the system
- **WHEN** a directory contains `pom.xml`
- **THEN** the system adds it to the list of detected Java projects with marker `pom.xml`

#### Scenario: Gradle project detected

- **GIVEN** Java is available on the system
- **WHEN** a directory contains `build.gradle` or `build.gradle.kts`
- **THEN** the system adds it to the list of detected Java projects with the corresponding Gradle marker

#### Scenario: No Java projects found

- **GIVEN** Java is available on the system
- **WHEN** no directories contain recognized Java project markers
- **THEN** the user is not prompted and Java instrumentation is skipped

### Requirement: Java process detection

The system SHALL detect running `java` processes and attempt to match them to discovered projects by working directory. Detection is best-effort — it may fail on processes owned by other users or on systems with restricted permissions.

#### Scenario: Running Java process matched to project

- **GIVEN** one or more Java projects have been detected on the filesystem
- **WHEN** a running `java` process has a CWD matching a detected project directory
- **THEN** the project listing shows the associated PIDs

### Requirement: Java instrumentation output

The system SHALL guide the user through attaching the OpenTelemetry Java agent to their application. It SHALL display the agent JAR download URL, the required environment variables, and the `-javaagent` JVM flag. The user should download the JAR and restart their application.

#### Scenario: Plan preview shows project and agent info

- **GIVEN** the user selected a Java project
- **WHEN** the combined installation plan preview is shown
- **THEN** the Java section displays the project path, the agent JAR download URL, and the `-javaagent` JVM flag

#### Scenario: Post-install output guides agent setup

- **GIVEN** the user confirmed the combined installation plan
- **WHEN** the Java instrumentation step executes
- **THEN** the output shows the agent JAR download URL, the environment variable export statements, and the `-javaagent` JVM flag — with a clear note that the user downloads the JAR manually
