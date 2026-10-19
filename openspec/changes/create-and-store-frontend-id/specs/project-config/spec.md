# Spec: Project Config

## ADDED Requirements

### Requirement: Project config file location

The system SHALL store project configuration in a single YAML file at `{project-dir}/.dtwiz/config.yaml`, referred to below as the "project config file". All other requirements in this spec refer to this file by that name.

#### Scenario: File located inside the project directory

- **GIVEN** a project directory `P`
- **WHEN** the system reads or writes the project config for `P`
- **THEN** it uses the file `P/.dtwiz/config.yaml` and no other location

### Requirement: Project config file schema

The project config file SHALL contain a top-level `frontends` map keyed by the full Dynatrace environment URL. Each entry SHALL hold the frontend application `id` and its `frontendName`. Unknown keys SHALL NOT cause loading to fail, so that fields can be added in later versions.

#### Scenario: Frontend entry layout

- **GIVEN** a project config with a frontend entry for environment `https://abc12345.apps.dynatrace.com`
- **WHEN** the config is saved
- **THEN** the file contains `frontends.<environment URL>.id` and `frontends.<environment URL>.frontendName`

### Requirement: Load project config from disk

The system SHALL read the project config file when requested. If the file does not exist, the system SHALL return an empty config without error.

#### Scenario: Config file exists

- **GIVEN** a project with a project config file containing stored values
- **WHEN** a caller requests the project config for that project
- **THEN** the system returns the config populated with all stored values

#### Scenario: Config file does not exist

- **GIVEN** a project with no project config file
- **WHEN** a caller requests the project config for that project
- **THEN** the system returns an empty config and no error

#### Scenario: Config file is malformed

- **GIVEN** a project whose project config file contains invalid YAML
- **WHEN** a caller requests the project config
- **THEN** the system returns an error describing the parse failure

### Requirement: Save project config to disk

The system SHALL write the project config to the project config file, creating the `.dtwiz/` directory if it does not exist. The written file SHALL be valid YAML and SHALL be readable and writable by the owning user only (`0o600` on platforms that support Unix permissions).

#### Scenario: Directory does not exist

- **GIVEN** a project where the `.dtwiz/` directory does not exist
- **WHEN** a caller saves the project config
- **THEN** the system creates the directory and writes the project config file

#### Scenario: File already exists

- **GIVEN** a project with an existing project config file
- **WHEN** a caller saves a new project config
- **THEN** the system overwrites the file with the new content, preserving no stale keys

#### Scenario: File permissions

- **GIVEN** a project on a platform that supports Unix permissions
- **WHEN** a caller saves the project config
- **THEN** the project config file has mode `0o600`

### Requirement: Updating one environment preserves all others

When a caller loads the config, modifies an entry for one environment URL, and saves it back, the system SHALL preserve all entries for other environment URLs unchanged.

#### Scenario: Update one environment, retain another

- **GIVEN** a project config containing entries for two different environment URLs
- **WHEN** a caller loads the config, updates the entry for environment A, and saves
- **THEN** the saved config contains the updated entry for environment A and the unchanged entry for environment B

### Requirement: Config keyed by environment URL

The project config SHALL store values keyed by the full Dynatrace environment URL so that a single project directory can hold independent configuration for multiple environments.

#### Scenario: Two environments stored in one file

- **GIVEN** a project config containing entries for two different environment URLs
- **WHEN** the config is read
- **THEN** reading for environment A returns A's values and reading for environment B returns B's values, independently

#### Scenario: Unknown environment key

- **GIVEN** a project config containing an entry for one environment URL
- **WHEN** a caller reads the config for an environment URL that has no stored entry
- **THEN** the system returns a zero-value entry and no error
