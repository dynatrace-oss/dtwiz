## ADDED Requirements

### Requirement: Collector setup detects all supported runtimes
The collector setup flow SHALL detect all supported language runtimes (Python, Java, Node.js, Go) using the analyzer's service detection, and present them as selectable options to the user.

#### Scenario: All runtimes detected
- **WHEN** the user runs `dtwiz install otel` and Python, Java, Node.js, and Go are all available in PATH
- **THEN** the collector setup SHALL display all four as numbered menu options

#### Scenario: Subset of runtimes detected
- **WHEN** the user runs `dtwiz install otel` and only Python and Java are in PATH
- **THEN** the collector setup SHALL display only Python and Java as options

#### Scenario: No runtimes detected
- **WHEN** the user runs `dtwiz install otel` and no supported runtimes are in PATH
- **THEN** the collector setup SHALL proceed with collector-only installation and inform the user that no supported runtimes were found

#### Scenario: Runtime detected but installer not yet implemented
- **WHEN** a runtime is detected but its `otel-<lang>` installer does not exist yet
- **THEN** the runtime SHALL be listed with a "(coming soon)" label and SHALL NOT be selectable
