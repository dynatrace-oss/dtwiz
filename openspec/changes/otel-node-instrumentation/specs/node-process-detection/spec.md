## ADDED Requirements

### Requirement: Detect running Node.js processes
The system SHALL detect running Node.js processes and correlate them to project directories.

#### Scenario: Node processes found
- **WHEN** Node.js processes are running on the system
- **THEN** the system SHALL list them with PID, script path, and working directory

#### Scenario: Process-to-project correlation
- **WHEN** a running Node process's working directory matches a detected project directory
- **THEN** the system SHALL mark that project as "running" in the project list

#### Scenario: No Node processes found
- **WHEN** no Node.js processes are running
- **THEN** the system SHALL proceed with project selection without process information
