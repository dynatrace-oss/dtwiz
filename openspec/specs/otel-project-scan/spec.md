# Spec: OTel Project Scan

## ADDED Requirements

### Requirement: Scan scope limited to working directory

The scanner SHALL search only the working directory and its subdirectories for OTel project markers. It SHALL NOT traverse any ancestor directories of the working directory.

#### Scenario: Project in working directory is found

- **WHEN** the user runs a dtwiz OTel install command from a directory that contains a project marker (e.g. `requirements.txt`, `package.json`)
- **THEN** the project is detected and instrumentation proceeds

#### Scenario: Project in subdirectory is found

- **WHEN** the user runs a dtwiz OTel install command from a parent directory that contains subdirectories with project markers
- **THEN** all matching subdirectory projects are detected

#### Scenario: Parent directory is not scanned during install

- **WHEN** the user runs a dtwiz OTel install command from a subdirectory of their project root (e.g. `my-project/src/`)
- **THEN** the parent directory (`my-project/`) is NOT scanned and projects there are NOT detected

#### Scenario: Parent directory is not scanned during uninstall

- **WHEN** the user runs a dtwiz OTel uninstall command from a subdirectory
- **THEN** `.otel/` directories in parent directories are NOT found and NOT removed; only `.otel/` directories within the working directory tree are considered

---

### Requirement: Bundled examples directory is always included in project scan

The scanner SHALL include `~/.dtwiz/examples/` as an additional scan root alongside the working directory. This ensures that the demo app is visible in the project list regardless of the directory the user runs `dtwiz` from.

The bundled examples path is:

- macOS and Linux: `$HOME/.dtwiz/examples/`
- Windows: `%USERPROFILE%\.dtwiz\examples\`

If the path does not exist, it is silently skipped.

#### Scenario: Demo app is detected from any working directory

- **WHEN** `~/.dtwiz/examples/schnitzel/` exists
- **AND** the user runs `dtwiz setup` or `dtwiz install otel` from any directory
- **THEN** the schnitzel project SHALL appear in the list of detected projects alongside any projects found in the working directory

#### Scenario: Bundled examples path does not exist

- **WHEN** `~/.dtwiz/examples/` does not exist on the user's machine
- **THEN** the scanner SHALL skip it silently and only scan the working directory

#### Scenario: Project in bundled examples does not duplicate a CWD project

- **WHEN** the working directory is `~/.dtwiz/examples/` or a subdirectory of it
- **THEN** the scanner SHALL NOT list the same project twice

#### Scenario: CWD projects and bundled examples projects both appear in results

- **WHEN** the working directory contains one or more projects
- **AND** `~/.dtwiz/examples/` also contains one or more projects
- **THEN** the scanner SHALL return all projects from both locations in a single combined list
- **AND** each project SHALL appear exactly once

---

### Requirement: Demo app remains visible in dtwiz setup after install demo completes

After `dtwiz install demo` has run, the schnitzel project SHALL appear in the project list when the user runs `dtwiz setup` from any directory, so the user can manage or update the instrumentation without needing to navigate to the bundled path.

#### Scenario: Schnitzel appears in dtwiz setup after demo install from a different directory

- **GIVEN** `dtwiz install demo` has completed and the OTel Collector is configured for schnitzel
- **WHEN** the user opens a new terminal in a different directory
- **AND** runs `dtwiz setup` or `dtwiz install otel`
- **THEN** the schnitzel project at `~/.dtwiz/examples/schnitzel/` SHALL appear in the detected project list
- **AND** the user SHALL be able to select it to update or re-instrument it

#### Scenario: Schnitzel appears alongside user projects in the same setup session

- **GIVEN** the user's working directory contains their own Python or Node.js project
- **AND** `~/.dtwiz/examples/schnitzel/` also exists
- **WHEN** the user runs `dtwiz setup`
- **THEN** both the user's project and schnitzel SHALL appear in the project list
- **AND** the user SHALL be able to select either one
