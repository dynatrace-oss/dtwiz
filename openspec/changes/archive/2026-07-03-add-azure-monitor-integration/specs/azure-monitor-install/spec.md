# Azure Monitor Install

## ADDED Requirements

### Requirement: Install command and entry point

The system SHALL expose `dtwiz install azure` as the final runnable command for Azure installation, accepting no extra positional arguments. The command SHALL set up the Dynatrace Azure Monitor integration, resolve the Dynatrace environment URL and platform token from the standard sources (`--environment`/`DT_ENVIRONMENT`, `--platform-token`/`DT_PLATFORM_TOKEN`), and honor the shared `--dry-run` flag.

#### Scenario: Install command registered

- **GIVEN** the CLI is built
- **WHEN** the user runs `dtwiz install azure`
- **THEN** the Azure installer runs against the resolved environment and platform token
- **AND** `--dry-run` previews the workflow without making changes

### Requirement: Secret-free federated identity authentication

The system SHALL authenticate Dynatrace to Azure using a federated identity (workload identity) credential and SHALL NOT create or store any Azure client secret. The Service Principal SHALL be created without a password. The federated credential's subject SHALL be bound to the DT connection object ID, its issuer SHALL be derived from the Dynatrace environment URL, and its audience SHALL identify the `com.dynatrace.da` service.

#### Scenario: Service Principal created without a password

- **GIVEN** preflight has passed
- **WHEN** the installer registers the Azure Service Principal
- **THEN** it creates the SP with the `--create-password false` flag
- **AND** no client secret is generated, printed, or persisted

#### Scenario: Issuer derived from environment URL

- **GIVEN** the environment apps host `<id>.apps.dynatrace.com`
- **WHEN** the federated credential body is built
- **THEN** the issuer is `https://token.dynatrace.com`
- **AND** for `<id>.dev.apps.dynatracelabs.com` the issuer is `https://dev.token.dynatracelabs.com`
- **AND** for `<id>.sprint.apps.dynatracelabs.com` the issuer is `https://sprint.token.dynatracelabs.com`

### Requirement: Preflight checks before any mutation

The system SHALL run preflight checks before mutating any resource: the `az` CLI SHALL be present on `PATH`, and `az account show` SHALL succeed (the user is logged in). From the account the system SHALL read the subscription ID and tenant ID, and the integration SHALL operate at subscription scope. Failure of either check SHALL abort the install with a user-actionable message.

#### Scenario: Azure CLI missing

- **GIVEN** `az` is not on `PATH`
- **WHEN** the installer runs preflight
- **THEN** it aborts with a message pointing to the Azure CLI install page

#### Scenario: Not logged in

- **GIVEN** `az` is installed but `az account show` fails
- **WHEN** the installer runs preflight
- **THEN** it aborts with a message instructing the user to run `az login`

### Requirement: Advisory permissions check that never blocks

The system SHALL make a best-effort check that the signed-in principal can create role assignments at subscription scope. This check SHALL be advisory only: if it cannot be completed or does not confirm sufficient access, the system SHALL print a warning and continue. It SHALL NOT block the install.

#### Scenario: Permissions check cannot be resolved

- **GIVEN** the signed-in user cannot be resolved or the permissions check errors
- **WHEN** the check runs
- **THEN** it warns and returns without blocking the install

#### Scenario: Permissions check reports insufficient access

- **GIVEN** the check returns a result other than allowed
- **WHEN** the check runs
- **THEN** it prints a warning that Owner or User Access Administrator may be required and continues

### Requirement: Delegate to update when a complete integration already exists

The system SHALL check for an existing Dynatrace Azure connection named `dtwiz-azure` before installing. When exactly one such connection is found and it already carries its bound application ID, the system SHALL delegate to the in-place reconcile flow (see `azure-monitor-update`) instead of installing, leaving the authentication chain untouched. When the only matches are incomplete (missing their application ID) or there is more than one, the system SHALL abort with guidance to run `dtwiz uninstall azure` first, since that state cannot be safely auto-repaired.

#### Scenario: Existing complete connection delegates to update

- **GIVEN** a complete connection named `dtwiz-azure` already exists in the environment
- **WHEN** `dtwiz install azure` runs
- **THEN** it reconciles the `da-azure` monitoring configuration in place instead of installing
- **AND** it does not recreate the connection, Service Principal, federated credential, or role assignment

#### Scenario: Existing incomplete or duplicated connection blocks install

- **GIVEN** a connection named `dtwiz-azure` already exists but is missing its bound application ID, or more than one such connection exists
- **WHEN** `dtwiz install azure` runs
- **THEN** it aborts without mutating anything and tells the user to uninstall first

### Requirement: Preview and confirmation before applying

The system SHALL print a preview showing the environment, tenant, subscription, connection name, and configuration name, followed by the numbered list of commands to be executed, with the platform token masked. It SHALL then prompt a single `Apply?` confirmation (default yes). On `--dry-run` it SHALL print the preview and stop without prompting or mutating; on decline it SHALL cancel without making any changes.

#### Scenario: Dry run shows preview only

- **GIVEN** `--dry-run` is set
- **WHEN** the installer runs
- **THEN** it prints the preview and `[dry-run] No changes were made.` and makes no changes

#### Scenario: Token masked in preview

- **GIVEN** the preview lists steps that reference the platform token
- **WHEN** the steps are printed
- **THEN** every occurrence of the token is replaced with `***`

#### Scenario: User declines

- **GIVEN** the preview has been shown and `--dry-run` is not set
- **WHEN** the user answers no to `Apply?`
- **THEN** the install is cancelled without mutating anything

### Requirement: Seven-step installation workflow in order

The system SHALL execute the installation as seven ordered steps: (1) create the Dynatrace Azure connection; (2) create the Azure Service Principal; (3) create the federated credential bound to the connection ID; (4) retrieve the Service Principal object ID; (5) assign the Monitoring Reader role at subscription scope; (6) finalize the Dynatrace connection with the Azure tenant and application IDs; (7) create the `da-azure` monitoring configuration. The application ID from step 2 SHALL be threaded forward into steps 3, 5, 6, and 7.

#### Scenario: Steps run in order and thread identifiers forward

- **GIVEN** the user confirmed the install
- **WHEN** the workflow runs
- **THEN** the connection object ID from step 1 is used as the federated credential subject in step 3 and the connection update target in step 6
- **AND** the client ID from step 2 is used in steps 3, 5, 6, and 7
- **AND** the object ID from step 4 is used as the role assignment assignee in step 5

### Requirement: Monitoring configuration defaults derived from the live extension schema

The system SHALL determine the extension version to use by selecting the highest semantic version available for `com.dynatrace.extension.da-azure`. It SHALL fetch that version's monitoring-configuration schema and populate the configuration's location filtering from the location enum and the feature sets from the feature-sets enum, keeping only values ending in `_essential`. Subscription filtering SHALL be set to include the logged-in subscription, and the credential entry SHALL reference the connection object ID and Service Principal client ID using federated authentication. If no locations or no `_essential` feature sets are found, the system SHALL fail with a descriptive error rather than create a partial configuration.

#### Scenario: Defaults populated from schema enums

- **GIVEN** the latest `da-azure` schema exposes location and feature-set enums
- **WHEN** the monitoring configuration is created
- **THEN** location filtering contains all schema location values
- **AND** feature sets contains exactly the `*_essential` values from the schema

#### Scenario: Highest extension version selected

- **GIVEN** the extension lists multiple versions
- **WHEN** the installer chooses a version
- **THEN** it selects the highest by semantic-version comparison

#### Scenario: Empty enums fail fast

- **GIVEN** the schema yields no locations or no `_essential` feature sets
- **WHEN** the monitoring configuration would be created
- **THEN** the install fails with an error naming the missing enum and creates no configuration

### Requirement: Tolerate Azure propagation delays on SP lookup

The system SHALL retrieve the Service Principal object ID by retrying up to 5 times with a 3-second pause between attempts while the SP is reported as not found or not yet propagated. A permissions error SHALL fail immediately without further retries. An empty object ID SHALL be retried; exhausting all retries SHALL return an error.

#### Scenario: SP not yet propagated then succeeds

- **GIVEN** the SP lookup first returns not-found
- **WHEN** the lookup retries
- **THEN** it waits 3 seconds between attempts and returns the object ID once available

#### Scenario: Permissions error fails fast

- **GIVEN** the SP lookup returns a permissions error
- **WHEN** the lookup runs
- **THEN** it returns an error immediately without further retries

### Requirement: Replace a stale federated credential on conflict

When federated-credential creation (step 3) fails because one already exists, the system SHALL delete the existing credential and retry creation once. A delete that reports "not found" SHALL be treated as success.

#### Scenario: Pre-existing credential replaced

- **GIVEN** step 3 returns an "already exists" error
- **WHEN** the installer handles it
- **THEN** it deletes the `dtwiz-azure-Federated-Credential` and retries creation once

### Requirement: Retry connection finalization on federation propagation errors

The system SHALL retry the Dynatrace connection finalization (step 6) up to 10 times with a 5-second pause while the error indicates Azure has not yet propagated the federated credential. Any other error SHALL stop retrying immediately.

#### Scenario: Propagation error retried

- **GIVEN** step 6 returns a federation propagation error
- **WHEN** the update retries
- **THEN** it waits 5 seconds between attempts up to 10 times until the update succeeds or a different error occurs

#### Scenario: Permanent error stops retrying

- **GIVEN** step 6 returns a non-propagation error
- **WHEN** the update runs
- **THEN** the retry loop stops immediately and the error is returned

### Requirement: Partial-failure cleanup guidance

On failure after one or more mutating steps have completed, the system SHALL print the resources that were already created (Dynatrace connection, Azure App Registration, role assignment) with explicit commands to remove them, and SHALL note that re-running `dtwiz uninstall azure` removes them all automatically.

#### Scenario: Hint after connection and SP created

- **GIVEN** steps 1 and 2 succeeded but a later step failed
- **WHEN** the installer returns the error
- **THEN** it lists the created DT connection and Azure App Registration with their cleanup commands and points to `dtwiz uninstall azure`

### Requirement: Watch ingested data after a successful install

After a successful install, the system SHALL tail newly ingested Azure data starting from the recorded install start time. When the start time is zero (the unit-test path), the watch SHALL be skipped.

#### Scenario: Watch skipped in tests

- **GIVEN** the install start time is the zero value
- **WHEN** the install completes
- **THEN** no ingest watch is started
