## ADDED Requirements

### Requirement: Pre-flight validation for Java installer
The `InstallOtelJava()` function SHALL validate prerequisites before proceeding.

#### Scenario: Java not in PATH
- **WHEN** `java` is not found in PATH
- **THEN** the installer SHALL exit with a clear error message indicating Java is required

#### Scenario: Java version too old
- **WHEN** `java -version` reports a version older than Java 8
- **THEN** the installer SHALL exit with a clear error message indicating Java 8+ is required

#### Scenario: All prerequisites met
- **WHEN** `java` is in PATH and version is 8 or newer
- **THEN** the installer SHALL proceed with the installation flow
