# Java Runtime Detection

## ADDED Requirements

### Requirement: Java runtime detection

The system SHALL detect Java installations by looking up `java` on PATH and verifying the version via `java -version`.

#### Scenario: Java is available

- **GIVEN** the user selected Java from the runtime selection menu
- **WHEN** `java` is found on PATH and `java -version` succeeds
- **THEN** the system reports the Java path and version and proceeds with project scanning

#### Scenario: Java is not available

- **GIVEN** the system is checking for available runtimes
- **WHEN** `java` is not found on PATH
- **THEN** the system silently skips Java detection and returns nil

### Requirement: Java project scanning

The system SHALL scan the filesystem for Java project markers (`pom.xml`, `build.gradle`, `build.gradle.kts`) starting from the current directory and common project locations. Follows the same scanning pattern established by `DetectPythonPlan` in `pkg/installer/otel_python.go`.

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
- **THEN** `DetectJavaPlan` returns nil without prompting the user

### Requirement: Java process detection

The system SHALL detect running `java` processes and attempt to match them to discovered projects by working directory.

#### Scenario: Running Java process matched to project

- **GIVEN** one or more Java projects have been detected on the filesystem
- **WHEN** a running `java` process has a CWD matching a detected project directory
- **THEN** the project listing shows the associated PIDs

### Requirement: Java project selection prompt

The system SHALL present discovered Java projects and prompt the user to select one or skip.

#### Scenario: User selects a project

- **GIVEN** one or more Java projects are listed
- **WHEN** the user enters a valid project number
- **THEN** the system creates a `JavaInstrumentationPlan` for that project

#### Scenario: User skips

- **GIVEN** one or more Java projects are listed
- **WHEN** the user presses Enter without selecting
- **THEN** `DetectJavaPlan` returns nil

### Requirement: JavaInstrumentationPlan struct

The system SHALL define a `JavaInstrumentationPlan` struct with fields for the selected project, the Java agent JAR path, OTel environment variables, `EnvURL`, and `PlatformToken`. It SHALL implement `PrintPlanSteps()` and `Execute()` methods. Follows the pattern established by `PythonInstrumentationPlan` in `pkg/installer/otel_python.go`.

#### Scenario: PrintPlanSteps displays plan

- **GIVEN** a `JavaInstrumentationPlan` was created for a selected project
- **WHEN** `PrintPlanSteps()` is called
- **THEN** it prints the project path, agent JAR download URL, and environment variables to be set

#### Scenario: Execute performs instrumentation

- **GIVEN** the user confirmed the combined installation plan
- **WHEN** `Execute()` is called
- **THEN** it downloads the OTel Java agent JAR, sets the required environment variables, and provides the `-javaagent` JVM flag to the user
