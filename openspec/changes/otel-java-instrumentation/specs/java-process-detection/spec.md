## ADDED Requirements

### Requirement: Detect running Java processes
The system SHALL detect running Java processes and present them to the user for selection.

#### Scenario: Java processes found
- **WHEN** Java processes are running on the system
- **THEN** the system SHALL list them with PID, main class or JAR name, and working directory

#### Scenario: No Java processes found
- **WHEN** no Java processes are running
- **THEN** the system SHALL inform the user and offer to provide a manual command to instrument

#### Scenario: User selects a process
- **WHEN** the user selects a process from the list
- **THEN** the system SHALL use that process's command line as the basis for instrumented restart
