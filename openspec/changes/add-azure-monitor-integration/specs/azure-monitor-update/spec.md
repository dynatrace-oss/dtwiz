# Azure Monitor Update

## ADDED Requirements

### Requirement: Update is an in-place monitoring-config reconcile reached through setup

The system SHALL provide an `UpdateAzure` entry point that refreshes an existing integration in place by reconciling **only** the `da-azure` monitoring configuration to the latest schema-derived defaults. The authentication chain SHALL NOT be modified by an update: the Dynatrace connection, the Azure Service Principal, the federated credential, and the Monitoring Reader role assignment. There SHALL intentionally be no `dtwiz update azure` subcommand; the update path SHALL be reached from `dtwiz setup` when an Azure connection already exists.

#### Scenario: Setup routes to update when configured

- **GIVEN** an Azure connection named `dtwiz-azure` already exists
- **WHEN** the user selects Azure in `dtwiz setup`
- **THEN** the setup flow runs an in-place reconcile instead of a fresh install
- **AND** the Azure entry in the setup list is badged as already configured

#### Scenario: Setup installs fresh when not configured

- **GIVEN** no Azure connection exists
- **WHEN** the user selects Azure in `dtwiz setup`
- **THEN** the setup flow calls the full installer

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

The system SHALL require exactly one discovered `dtwiz-azure` connection that already carries its bound application ID, since the monitoring configuration references both the connection object ID and the Service Principal client ID. If no such connection exists, or the only matches are missing their application ID, the system SHALL abort with guidance to run `dtwiz install azure`. If more than one complete connection is found, the system SHALL abort with guidance to run `dtwiz uninstall azure` and then `dtwiz install azure` for a clean single integration.

#### Scenario: No complete connection aborts with install guidance

- **GIVEN** the only `dtwiz-azure` connection has no bound application ID
- **WHEN** the update runs
- **THEN** it aborts and tells the user to run `dtwiz install azure`

#### Scenario: Multiple connections abort with reinstall guidance

- **GIVEN** two complete `dtwiz-azure` connections exist
- **WHEN** the update runs
- **THEN** it aborts and tells the user to uninstall then install for a clean single integration

### Requirement: Preview and confirmation

The system SHALL present a preview with the environment, tenant, subscription, connection name (marked unchanged), and configuration name, followed by the numbered monitoring-configuration steps (one update per existing configuration, or a single create when none exists). It SHALL note that authentication is left unchanged. Any credentials or tokens that appear in the preview SHALL be masked so they are never printed in plaintext. It SHALL prompt a single `Apply?` confirmation; `--dry-run` SHALL stop after the preview and decline SHALL cancel without making any changes.

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

After confirmation, the system SHALL rewrite each discovered monitoring configuration in place using the latest schema-derived defaults (highest extension version, all schema locations, all `*_essential` feature sets, subscription filtering scoped to the logged-in subscription, and a federated credential referencing the existing connection object ID and Service Principal client ID). When no monitoring configuration exists, the system SHALL create one with the same defaults. The same empty-enum fail-fast as install SHALL apply. Because each configuration is rewritten with a single atomic write, a failure SHALL leave the prior configuration intact and SHALL NOT have touched the authentication chain.

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
