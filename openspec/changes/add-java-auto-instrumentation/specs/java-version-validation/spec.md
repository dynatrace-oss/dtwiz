# Spec: Java Version Validation

## ADDED Requirements

### Requirement: Pre-flight validation for Java installer

The `InstallOtelJava()` function SHALL validate prerequisites before proceeding with installation. The minimum Java version is 8 because the OpenTelemetry Java agent requires Java 8+.

#### Scenario: Java not in PATH

- **WHEN** `java` is not found in PATH
- **THEN** the installer SHALL exit with a clear error message: "Java not found — install a JDK/JRE and ensure it is in PATH"

#### Scenario: Java version cannot be determined

- **WHEN** `java` is found in PATH but `java -version` fails or produces unrecognizable output
- **THEN** the installer SHALL exit with an error message: "unable to determine Java version"

#### Scenario: Java version below minimum (< 8)

- **WHEN** `java -version` reports a version lower than 8 (e.g., `java version "1.7.0_80"`)
- **THEN** the installer SHALL exit with an error message stating the detected version and that the OpenTelemetry Java agent requires Java 8 or later

#### Scenario: All prerequisites met

- **WHEN** `java` is in PATH and reports version >= 8
- **THEN** the installer SHALL proceed with the normal installation flow

### Requirement: Java version parsing

The version parser SHALL correctly extract the major version number from all common `java -version` output formats.

#### Scenario: Legacy versioning (Java 8 and earlier)

- **GIVEN** `java -version` outputs a line containing `version "1.8.0_382"` (or similar `1.X.Y_Z` pattern)
- **WHEN** the version string is parsed
- **THEN** the major version SHALL be extracted as the second component (8 in this case)

#### Scenario: Modern versioning (Java 9+)

- **GIVEN** `java -version` outputs a line containing `version "17.0.1"` (or similar `X.Y.Z` pattern)
- **WHEN** the version string is parsed
- **THEN** the major version SHALL be the first component (17 in this case)

#### Scenario: Short version string (Java 21+)

- **GIVEN** `java -version` outputs a line containing `version "21"` (single number, no dots)
- **WHEN** the version string is parsed
- **THEN** the major version SHALL be 21

#### Scenario: OpenJDK format

- **GIVEN** `java -version` outputs `openjdk version "11.0.20" 2023-07-18`
- **WHEN** the version string is parsed
- **THEN** the major version SHALL be 11

#### Scenario: Vendor-specific prefixes

- **GIVEN** `java -version` outputs lines from vendors like Amazon Corretto, GraalVM, or Azul Zulu that include additional text before or after the version string
- **WHEN** the version string is parsed
- **THEN** the parser SHALL correctly extract the quoted version string regardless of surrounding text

### Requirement: Debug logging for version validation

The validation and parsing steps SHALL emit debug-level log lines so users running with `--debug` can see exactly what `java -version` returned and what version was parsed.

#### Scenario: Java binary found

- **WHEN** `java` is located in PATH
- **THEN** a debug line SHALL be emitted: `"java binary found" path=<path>`

#### Scenario: Version string parsed

- **WHEN** `parseJavaVersion` successfully extracts a major version
- **THEN** a debug line SHALL be emitted: `"java version parsed" raw=<raw output> major=<n>`

#### Scenario: Version validation passed

- **WHEN** the parsed version is >= 8
- **THEN** a debug line SHALL be emitted: `"java version OK" major=<n>`

#### Scenario: Version validation failed

- **WHEN** the parsed version is < 8
- **THEN** a debug line SHALL be emitted: `"java version too old" major=<n> minimum=8` before the error is returned to the caller
