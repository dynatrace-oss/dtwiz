# Spec: Project Config

## ADDED Requirements

### Requirement: Load project config from disk

The system SHALL read the project config file at `{project-dir}/.dtwiz/config.yaml` when requested. If the file does not exist, the system SHALL return an empty config without error.

#### Scenario: Config file exists

- **GIVEN** a project directory containing `.dtwiz/config.yaml` with stored values
- **WHEN** a caller requests the project config for that directory
- **THEN** the system returns the config populated with all stored values

#### Scenario: Config file does not exist

- **GIVEN** a project directory with no `.dtwiz/config.yaml` file
- **WHEN** a caller requests the project config for that directory
- **THEN** the system returns an empty config and no error

#### Scenario: Config file is malformed

- **GIVEN** a project directory whose `.dtwiz/config.yaml` contains invalid YAML
- **WHEN** a caller requests the project config
- **THEN** the system returns an error describing the parse failure

### Requirement: Save project config to disk

The system SHALL write the project config to `{project-dir}/.dtwiz/config.yaml`, creating the `.dtwiz/` directory if it does not exist. The written file SHALL be valid YAML.

#### Scenario: Directory does not exist

- **GIVEN** a project directory where `.dtwiz/` does not exist
- **WHEN** a caller saves the project config
- **THEN** the system creates the directory and writes the file

#### Scenario: File already exists

- **GIVEN** a project directory with an existing `.dtwiz/config.yaml`
- **WHEN** a caller saves a new project config
- **THEN** the system overwrites the file with the new content, preserving no stale keys

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
