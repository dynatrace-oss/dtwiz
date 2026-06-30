## ADDED Requirements

### Requirement: Scan scope limited to working directory
The scanner SHALL search only the working directory and its subdirectories for OTel project markers. It SHALL NOT traverse any ancestor directories of the working directory.

#### Scenario: Project in working directory is found
- **WHEN** the user runs a dtwiz OTel install command from a directory that contains a project marker (e.g. `requirements.txt`, `package.json`)
- **THEN** the project is detected and instrumentation proceeds

#### Scenario: Project in subdirectory is found
- **WHEN** the user runs a dtwiz OTel install command from a parent directory that contains subdirectories with project markers
- **THEN** all matching subdirectory projects are detected

#### Scenario: Parent directory is not scanned
- **WHEN** the user runs a dtwiz OTel install command from a subdirectory of their project root (e.g. `my-project/src/`)
- **THEN** the parent directory (`my-project/`) is NOT scanned and projects there are NOT detected
