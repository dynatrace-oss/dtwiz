# Azure Monitor Uninstall

## ADDED Requirements

### Requirement: Uninstall command and entry point

The system SHALL expose `dtwiz uninstall azure` as the CLI command for Azure removal, accepting no positional arguments. It SHALL remove the Dynatrace Azure Monitor integration created by the installer, resolve the environment URL and platform token from the standard sources, and honor the shared `--dry-run` flag. The package SHALL also expose a helper for callers (such as `dtwiz setup`) to detect whether an existing integration is present.

#### Scenario: Uninstall command registered

- **GIVEN** the CLI is built
- **WHEN** the user runs `dtwiz uninstall azure`
- **THEN** the uninstaller discovers and removes dtwiz-created Azure integration resources

### Requirement: Discover all dtwiz-named resources

The system SHALL discover every Dynatrace connection and every `da-azure` monitoring configuration whose name equals `dtwiz-azure`, returning all matches (not just the first) so leftovers from interrupted runs are also removed. Connection and monitoring-configuration lookups SHALL run concurrently.

#### Scenario: Duplicate leftovers all discovered

- **GIVEN** two connections named `dtwiz-azure` exist from an interrupted prior run
- **WHEN** the uninstaller discovers resources
- **THEN** both connection object IDs are returned for deletion

### Requirement: Ownership-verified Azure app discovery

The system SHALL gather the Azure application IDs to delete from two sources: those bound to a discovered dtwiz connection (trusted, always included), and those found by display-name lookup of the `dtwiz-azure` name. A display-name-only match SHALL be deleted only if the app carries dtwiz's federated credential fingerprint (a credential named `dtwiz-azure-Federated-Credential` issued by the expected Dynatrace token endpoint); apps that fail or cannot complete verification SHALL be skipped with a warning. The resulting set SHALL be de-duplicated. Azure lookup failures SHALL NOT block deletion of already-known resources.

#### Scenario: Connection-bound app trusted

- **GIVEN** a discovered connection is bound to a client ID
- **WHEN** client IDs are gathered
- **THEN** that ID is included without further verification

#### Scenario: Same-name app without fingerprint skipped

- **GIVEN** an App Registration named `dtwiz-azure` lacks the dtwiz federated credential
- **WHEN** client IDs are gathered
- **THEN** that app is skipped with a warning and not deleted

#### Scenario: Verification error skips the app

- **GIVEN** the federated-credential ownership check errors for a display-name match
- **WHEN** client IDs are gathered
- **THEN** that app is skipped with a warning and not deleted

### Requirement: Nothing-to-do short-circuit

When no monitoring configurations, no connections, and no client IDs are discovered, the system SHALL print that there is nothing to uninstall and return without prompting or mutating.

#### Scenario: No resources found

- **GIVEN** the environment has no dtwiz-named Azure resources
- **WHEN** the uninstaller runs
- **THEN** it prints a "nothing to uninstall" message and exits successfully

### Requirement: Preview and confirmation before deletion

The system SHALL print a preview listing the environment, each connection, each App Registration, and each monitoring configuration found (or noting when one is absent), followed by the numbered deletion steps. Any credentials or tokens that appear in the preview SHALL be masked so they are never printed in plaintext. It SHALL prompt a single `Apply?` confirmation (default yes). On `--dry-run` it SHALL print the preview and stop; on decline it SHALL cancel without making any changes.

#### Scenario: Dry run previews deletions only

- **GIVEN** resources exist and `--dry-run` is set
- **WHEN** the uninstaller runs
- **THEN** it prints the deletion preview and `[dry-run] No changes were made.` without deleting

#### Scenario: Credentials not exposed in preview

- **GIVEN** the preview output references the platform token or any other credential
- **WHEN** the preview is printed
- **THEN** every occurrence of the credential value is replaced with `***`

### Requirement: Best-effort deletion that continues past failures

The system SHALL delete resources in order: monitoring configurations, then per app the Monitoring Reader role assignment and the App Registration, then connections. Deleting the App Registration SHALL be the single call that also removes its Service Principal and federated credentials. A failed step SHALL print a warning and the run SHALL continue with remaining steps; all step errors SHALL be collected and returned together. Role-assignment and app deletions that report "not found" or "no matched assignments", and connection deletes that return a 404, SHALL be treated as success.

#### Scenario: One failure does not strand the rest

- **GIVEN** three deletion steps where the second fails
- **WHEN** the uninstaller runs
- **THEN** steps one and three still execute and the second step's error is returned

#### Scenario: Already-gone resource treated as success

- **GIVEN** a connection delete returns 404 (already deleted)
- **WHEN** that step runs
- **THEN** it is treated as success with no error

#### Scenario: App deletion cascades

- **GIVEN** an App Registration is deleted
- **WHEN** the step completes
- **THEN** its Service Principal and federated credentials are removed by the same delete call

### Requirement: Step count reflects discovered resources

The system SHALL compute the total number of deletion steps as one per monitoring configuration, two per app (role assignment plus app registration), and one per connection, and SHALL number the printed steps continuously against that total.

#### Scenario: Step total computed

- **GIVEN** 1 monitoring config, 1 app, and 1 connection are discovered
- **WHEN** the step count is computed
- **THEN** it equals 4 (1 + 2 + 1)
