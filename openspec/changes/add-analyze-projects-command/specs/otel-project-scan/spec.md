# Spec Delta

## MODIFIED Requirements

### Requirement: Bundled examples directory is always included in project scan

The scanner SHALL include `~/.dtwiz/examples/` as an additional scan root alongside the working directory in the interactive project selection of `dtwiz setup` and `dtwiz install otel`. This ensures that the demo app is visible in the project list regardless of the directory the user runs `dtwiz` from. The project inventory listing (`dtwiz analyze projects`) SHALL NOT add this root; it scans only its own scan root.

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

#### Scenario: Project inventory does not add the bundled examples root

- **GIVEN** `~/.dtwiz/examples/schnitzel/` exists and the working directory is `~/work/shop`
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** the schnitzel project SHALL NOT be listed
