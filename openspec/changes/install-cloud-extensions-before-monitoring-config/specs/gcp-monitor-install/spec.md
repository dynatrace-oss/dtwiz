## ADDED Requirements

### Requirement: Extension package activation before monitoring configuration

The system SHALL install or activate the `com.dynatrace.extension.da-gcp` extension package before creating the GCP monitoring configuration. If the extension is already installed, the system SHALL treat that condition as success. Extension activation SHALL happen after the Dynatrace connection is finalized and before monitoring configuration schema/version lookup or monitoring configuration creation.

#### Scenario: Extension installed before config creation

- **GIVEN** the user confirmed `dtwiz install gcp`
- **WHEN** the authentication chain has been created and the Dynatrace connection has been finalized
- **THEN** the installer installs or activates `com.dynatrace.extension.da-gcp`
- **AND** only then creates the `da-gcp` monitoring configuration

#### Scenario: Already-installed extension is accepted

- **GIVEN** the GCP data-acquisition extension is already installed in the tenant
- **WHEN** `dtwiz install gcp` reaches extension activation
- **THEN** the installer treats the already-installed response as success
- **AND** continues to monitoring configuration creation

#### Scenario: Extension activation failure stops config creation

- **GIVEN** extension activation returns an error other than already installed
- **WHEN** `dtwiz install gcp` reaches extension activation
- **THEN** the install returns an error
- **AND** no GCP monitoring configuration is created
