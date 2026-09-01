# Spec: Setup Project Detection

## Purpose

`dtwiz setup` scans the current working directory for well-known technology indicator
files at analysis time and surfaces the results inline on the host recommendation entry.
This spec covers which indicator files are recognised, how paths are normalised, and
how detected technologies are presented in the setup menu.

---

## Requirements

### Requirement: Project tech stack is detected from the current working directory

At setup time the tool SHALL scan the current working directory for well-known indicator
files and populate a list of detected technologies, each carrying the technology name
and the full path to the indicator file with the home-directory prefix replaced by `~`.
If no indicator files are found, the list SHALL be empty and no tech information is shown.

Supported indicator patterns SHALL include at minimum:

- `package.json` → Node.js
- `go.mod` → Go
- `requirements.txt`, `pyproject.toml`, `setup.py` → Python
- `pom.xml`, `build.gradle` → Java
- `Cargo.toml` → Rust
- `Gemfile` → Ruby
- `composer.json` → PHP
- `*.csproj`, `*.fsproj`, `*.sln` → .NET

#### Scenario: Go project directory

- **GIVEN** the current working directory contains `go.mod`
- **WHEN** the system is analyzed
- **THEN** the project tech list contains one entry with name `Go` and path `~/path/to/go.mod`

#### Scenario: Multiple indicator files present

- **GIVEN** the current working directory contains both `package.json` and `requirements.txt`
- **WHEN** the system is analyzed
- **THEN** the project tech list contains entries for Node.js and Python, each with their shortened paths

#### Scenario: No indicator files in cwd

- **GIVEN** the current working directory contains none of the supported indicator files
- **WHEN** the system is analyzed
- **THEN** the project tech list is empty

#### Scenario: Home directory prefix is shortened

- **GIVEN** the current working directory is inside the user's home directory
- **WHEN** a tech indicator file path is recorded
- **THEN** the path begins with `~` rather than the absolute home-directory path

---

### Requirement: Detected project tech is shown inline on the host recommendation

The host recommendation in `dtwiz setup` SHALL display the detected project technologies
as part of its detection-context line, formatted as `<tech> (<path>)` entries separated
by `·`. If no project techs are detected, the tech portion is omitted and only the
hostname, OS, and architecture are shown.

#### Scenario: Project tech detected

- **GIVEN** one or more project technologies are detected from the current working directory
- **WHEN** `dtwiz setup` renders the host recommendation entry
- **THEN** the detection-context line includes each tech name and its shortened path

#### Scenario: No project tech detected

- **GIVEN** the project tech list is empty
- **WHEN** `dtwiz setup` renders the host recommendation entry
- **THEN** the detection-context line shows only hostname, OS, and architecture

#### Scenario: Detection runs concurrently

- **GIVEN** a system analysis is initiated
- **WHEN** project tech detection runs alongside platform, Docker, and cloud detectors
- **THEN** the overall analysis latency is not increased by the file-scan time
