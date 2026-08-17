# Azure Monitor Update

## Purpose

Define the `dtwiz update azure` command that reconciles an existing Dynatrace Azure Monitor integration to the latest monitoring configuration.

## Requirements

### Requirement: Update is an in-place monitoring-config reconcile

The system SHALL refresh an existing integration by reconciling only the `da-azure` monitoring configuration to the latest schema-derived defaults. The authentication chain (connection, SP, federated credential, role assignment) SHALL NOT be modified. The update SHALL be reachable via `dtwiz update azure`, from `dtwiz setup`, and from `dtwiz install azure` when a complete connection exists.

#### Scenario: Direct subcommand invocation

- **GIVEN** an Azure connection named `dtwiz-azure` already exists
- **WHEN** the user runs `dtwiz update azure`
- **THEN** it runs the same in-place reconcile as the other entry points

#### Scenario: Setup routes to update when configured

- **GIVEN** an Azure connection named `dtwiz-azure` already exists
- **WHEN** the user selects Azure in `dtwiz setup`
- **THEN** the setup flow runs an in-place reconcile instead of a fresh install
- **AND** the Azure entry in the setup list is marked as already configured

#### Scenario: Setup installs fresh when not configured

- **GIVEN** no Azure connection exists
- **WHEN** the user selects Azure in `dtwiz setup`
- **THEN** the setup flow calls the full installer

#### Scenario: Install delegates to update when configured

- **GIVEN** a complete Azure connection named `dtwiz-azure` already exists
- **WHEN** the user runs `dtwiz install azure`
- **THEN** it runs the same in-place reconcile instead of aborting

#### Scenario: Authentication chain is never touched

- **GIVEN** an update runs against an existing integration
- **WHEN** the monitoring configuration is reconciled
- **THEN** the connection, Service Principal, federated credential, and role assignment are left unchanged

### Requirement: Concurrent discovery and account lookup

The system SHALL, in parallel, discover existing monitoring configurations, discover existing connections, and resolve the Azure account (CLI present, logged in, subscription and tenant from `az account show`). Any of the three failing SHALL abort the update. Because an update never creates a role assignment, it SHALL NOT run the role-assignment permissions advisory.

#### Scenario: Account lookup failure aborts update

- **GIVEN** `az account show` fails during the parallel lookup
- **WHEN** the update runs
- **THEN** it aborts before previewing or mutating anything

### Requirement: Require a complete existing connection

The system SHALL require exactly one `dtwiz-azure` connection with a bound application ID. If none exists or matches lack the application ID, it SHALL abort with guidance to run `dtwiz install azure`. If more than one complete connection is found, it SHALL abort with guidance to uninstall then reinstall for a clean single integration.

#### Scenario: No complete connection aborts with install guidance

- **GIVEN** the only `dtwiz-azure` connection has no bound application ID
- **WHEN** the update runs
- **THEN** it aborts and tells the user to run `dtwiz install azure`

#### Scenario: Multiple connections abort with reinstall guidance

- **GIVEN** two complete `dtwiz-azure` connections exist
- **WHEN** the update runs
- **THEN** it aborts and tells the user to uninstall then install for a clean single integration

### Requirement: Preview and confirmation

The system SHALL present a preview showing environment, tenant, subscription, connection name (unchanged), and configuration name, followed by numbered steps. Authentication is left unchanged. All credentials SHALL be masked. It SHALL prompt a single `Apply?` confirmation; `--dry-run` stops after preview and decline cancels without changes.

#### Scenario: Dry run previews only

- **GIVEN** `--dry-run` is set
- **WHEN** the update runs
- **THEN** it prints the preview and `[dry-run] No changes were made.` without mutating any configuration

#### Scenario: Credentials not exposed in preview

- **GIVEN** the preview output references the platform token or any other credential
- **WHEN** the preview is printed
- **THEN** every occurrence of the credential value is replaced with `***`

#### Scenario: Decline cancels

- **GIVEN** the preview has been shown and `--dry-run` is not set
- **WHEN** the user answers no to `Apply?`
- **THEN** the update is cancelled without mutating anything

### Requirement: Reconcile every monitoring configuration to schema-derived defaults

After confirmation, the system SHALL rewrite each discovered monitoring configuration in place using the latest schema-derived defaults: highest extension version, all schema locations, schema default feature sets, and the logged-in subscription. The configuration SHALL reference the existing connection object ID and SP client ID using federated auth. When none exists, it SHALL create one. Failure leaves the prior configuration intact and the authentication chain untouched.

#### Scenario: Existing configuration updated in place

- **GIVEN** one `da-azure` monitoring configuration exists
- **WHEN** the update runs
- **THEN** that configuration is updated in place with the latest defaults and no configuration is created or deleted

#### Scenario: Duplicate configurations all reconciled

- **GIVEN** two `dtwiz-azure` monitoring configurations exist from an interrupted prior run
- **WHEN** the update runs
- **THEN** both are reconciled in place

#### Scenario: Missing configuration created

- **GIVEN** a complete connection exists but no monitoring configuration
- **WHEN** the update runs
- **THEN** a monitoring configuration is created with the schema-derived defaults

#### Scenario: Failed update leaves auth and prior config intact

- **GIVEN** the monitoring-configuration update call fails
- **WHEN** the update returns the error
- **THEN** the connection is not deleted or modified and the error is surfaced

### Requirement: Watch ingested data after a successful update

After a successful reconcile the system SHALL start the Azure ingest watch from the recorded start time, skipped when the start time is zero (the unit-test path).

#### Scenario: Watch skipped in tests

- **GIVEN** the update start time is the zero value
- **WHEN** the update completes
- **THEN** no ingest watch is started

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
