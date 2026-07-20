## ADDED Requirements

### Requirement: Extension package activation before monitoring reconcile

The system SHALL install or activate the `com.dynatrace.extension.da-azure` extension package before creating or updating Azure monitoring configurations during update/reconcile. If the extension is already installed, the system SHALL treat that condition as success. Extension activation SHALL happen after user confirmation and before monitoring configuration schema/version lookup or monitoring configuration mutation.

#### Scenario: Extension installed before reconcile

- **GIVEN** the user confirmed `dtwiz update azure`
- **WHEN** the update is ready to reconcile Azure monitoring configurations
- **THEN** the update installs or activates `com.dynatrace.extension.da-azure`
- **AND** only then creates or updates Azure monitoring configurations

#### Scenario: Already-installed extension is accepted

- **GIVEN** the Azure data-acquisition extension is already installed in the tenant
- **WHEN** `dtwiz update azure` reaches extension activation
- **THEN** the update treats the already-installed response as success
- **AND** continues to monitoring configuration reconcile

#### Scenario: Extension activation failure stops reconcile

- **GIVEN** extension activation returns an error other than already installed
- **WHEN** `dtwiz update azure` reaches extension activation
- **THEN** the update returns an error
- **AND** no Azure monitoring configuration is created or updated
