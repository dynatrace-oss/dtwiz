# GCP Monitor Update

## Purpose

Define the `dtwiz update gcp` command that reconciles an existing Dynatrace GCP integration to the latest monitoring configuration.

## Requirements

### Requirement: Update is an in-place monitoring-config reconcile reachable from `dtwiz update gcp`, `dtwiz install gcp`, and `dtwiz setup`

The system SHALL refresh an existing integration in place by reconciling only the `da-gcp` monitoring configuration to the latest schema-derived defaults. It SHALL NOT modify the authentication chain. The update flow SHALL be reachable via `dtwiz update gcp`, via `dtwiz install gcp` when a connection already exists, and via `dtwiz setup` when GCP is fully configured.

#### Scenario: Update leaves authentication chain unchanged

- **GIVEN** an update runs against an existing GCP integration
- **WHEN** the monitoring configuration is reconciled
- **THEN** the connection, service account, project Viewer binding, and impersonation binding are left unchanged

### Requirement: `dtwiz update gcp` subcommand

The system SHALL expose `dtwiz update gcp` as the CLI command for GCP update, accepting no positional arguments. It SHALL resolve the Dynatrace environment URL and platform token from the standard sources, validate the platform token before proceeding, and honor the shared `--dry-run` flag.

#### Scenario: `dtwiz update gcp` registered

- **GIVEN** the CLI is built
- **WHEN** the user runs `dtwiz update gcp`
- **THEN** the GCP update flow runs against the resolved environment and platform token

#### Scenario: Platform token validated before update

- **GIVEN** the platform token is provided
- **WHEN** `dtwiz update gcp` runs
- **THEN** the platform token is validated against the environment before any update logic runs

#### Scenario: Setup routes to update when a complete connection exists

- **GIVEN** a connection named `dtwiz-gcp` already exists and carries a bound service-account email
- **WHEN** the user selects GCP in `dtwiz setup`
- **THEN** the setup flow runs an in-place reconcile instead of a fresh install
- **AND** the GCP entry in the setup list is marked as already configured

#### Scenario: Setup installs (or resumes) when not completely configured

- **GIVEN** no GCP connection exists, or only an incomplete one (no bound service-account email) exists
- **WHEN** the user selects GCP in `dtwiz setup`
- **THEN** the setup flow calls the full installer, which creates a fresh connection or resumes the incomplete one

#### Scenario: Authentication chain is never touched

- **GIVEN** an update runs against an existing integration
- **WHEN** the monitoring configuration is reconciled
- **THEN** the connection, service account, project Viewer binding, and impersonation binding are left unchanged

### Requirement: Concurrent discovery and account lookup

The system SHALL, in parallel, discover existing monitoring configurations, discover existing connections, and resolve the active `gcloud` project and account. Any of the three failing SHALL abort the update. Because an update never creates or modifies IAM bindings, it SHALL NOT run any permission-hint or role-grant logic.

#### Scenario: Project lookup failure aborts update

- **GIVEN** resolving the active `gcloud` project fails during the parallel lookup
- **WHEN** the update runs
- **THEN** it aborts before previewing or mutating anything

### Requirement: Require exactly one complete existing connection

The system SHALL require exactly one `dtwiz-gcp` connection with a bound service-account email. If none exists, it SHALL abort and direct the user to run `dtwiz install gcp`. If more than one exists, it SHALL abort and direct the user to uninstall then reinstall.

#### Scenario: No complete connection aborts with install guidance

- **GIVEN** the only `dtwiz-gcp` connection has no bound service-account email
- **WHEN** the update runs
- **THEN** it aborts and tells the user to run `dtwiz install gcp` (or uninstall then install to repair the partial connection)

#### Scenario: Multiple complete connections abort with reinstall guidance

- **GIVEN** two complete `dtwiz-gcp` connections exist
- **WHEN** the update runs
- **THEN** it aborts and tells the user to uninstall then install for a clean single integration

### Requirement: Preview and confirmation

The system SHALL present a preview with the environment, project, service account, connection name (marked unchanged), and configuration name, followed by the numbered monitoring-configuration steps (one update per existing configuration, or a single create when none exists). It SHALL note that authentication is left unchanged. It SHALL prompt a single `Apply?` confirmation; `--dry-run` SHALL stop after the preview and decline SHALL cancel without making any changes.

#### Scenario: Dry run previews only

- **GIVEN** `--dry-run` is set
- **WHEN** the update runs
- **THEN** it prints the preview and `[dry-run] No changes were made.` without mutating any configuration

#### Scenario: Decline cancels

- **GIVEN** the preview has been shown and `--dry-run` is not set
- **WHEN** the user answers no to `Apply?`
- **THEN** the update is cancelled without mutating anything

### Requirement: Reconcile every monitoring configuration to schema-derived defaults

After confirmation, the system SHALL rewrite each discovered monitoring configuration using the latest schema-derived defaults (highest extension version, all default feature sets, project scoped to the active `gcloud` project). When no configuration exists, it SHALL create one. A failure SHALL leave the prior configuration intact and SHALL NOT touch the authentication chain.

#### Scenario: Existing configuration updated in place

- **GIVEN** one `da-gcp` monitoring configuration exists
- **WHEN** the update runs
- **THEN** that configuration is updated in place with the latest defaults and no configuration is created or deleted

#### Scenario: Duplicate configurations all reconciled

- **GIVEN** two `dtwiz-gcp` monitoring configurations exist from an interrupted prior run
- **WHEN** the update runs
- **THEN** both are reconciled in place, each numbered against the total step count

#### Scenario: Missing configuration created

- **GIVEN** a complete connection exists but no monitoring configuration
- **WHEN** the update runs
- **THEN** a monitoring configuration is created with the schema-derived defaults

#### Scenario: Failed update leaves auth and prior config intact

- **GIVEN** the monitoring-configuration update call fails
- **WHEN** the update returns the error
- **THEN** the connection is not deleted or modified and the error is surfaced

### Requirement: Watch ingested data after a successful update

After a successful reconcile the system SHALL start the GCP ingest watch from the recorded start time, skipped when the start time is zero (the unit-test path).

#### Scenario: Watch skipped in tests

- **GIVEN** the update start time is the zero value
- **WHEN** the update completes
- **THEN** no ingest watch is started

### Requirement: Extension package activation before monitoring reconcile

The system SHALL install or activate the `com.dynatrace.extension.da-gcp` extension package before creating or updating GCP monitoring configurations during update/reconcile. If the extension is already installed, the system SHALL treat that condition as success. Extension activation SHALL happen after user confirmation and before monitoring configuration schema/version lookup or monitoring configuration mutation.

#### Scenario: Extension installed before reconcile

- **GIVEN** the user confirmed `dtwiz update gcp`
- **WHEN** the update is ready to reconcile GCP monitoring configurations
- **THEN** the update installs or activates `com.dynatrace.extension.da-gcp`
- **AND** only then creates or updates GCP monitoring configurations

#### Scenario: Already-installed extension is accepted

- **GIVEN** the GCP data-acquisition extension is already installed in the tenant
- **WHEN** `dtwiz update gcp` reaches extension activation
- **THEN** the update treats the already-installed response as success
- **AND** continues to monitoring configuration reconcile

#### Scenario: Extension activation failure stops reconcile

- **GIVEN** extension activation returns an error other than already installed
- **WHEN** `dtwiz update gcp` reaches extension activation
- **THEN** the update returns an error
- **AND** no GCP monitoring configuration is created or updated
