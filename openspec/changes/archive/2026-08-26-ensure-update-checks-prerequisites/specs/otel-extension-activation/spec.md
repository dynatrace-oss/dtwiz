# Spec: otel-extension-activation

## ADDED Requirements

### Requirement: OTel Dynatrace collector update ensures host monitoring extension is active

When `dtwiz update otel` runs on a Dynatrace-managed OTel Collector with the experimental flag enabled, the update SHALL ensure the OpenTelemetry Host Monitoring extension is installed and active in the tenant after the user confirms. If the extension cannot be installed or activated, the update SHALL warn the user and proceed rather than abort.

#### Scenario: Extension not present on tenant

- **GIVEN** the experimental flag is enabled and a platform token is provided
- **AND** the extension is not installed on the tenant
- **WHEN** the user runs `dtwiz update otel` on a Dynatrace-managed collector and confirms
- **THEN** the extension is installed and activated
- **AND** the collector update continues

#### Scenario: Extension already installed and active

- **GIVEN** the experimental flag is enabled and a platform token is provided
- **AND** the extension is already installed and active
- **WHEN** the user runs `dtwiz update otel` on a Dynatrace-managed collector and confirms
- **THEN** the update proceeds without re-installing or re-activating the extension

#### Scenario: Extension already installed but inactive

- **GIVEN** the experimental flag is enabled and a platform token is provided
- **AND** the extension is installed but not active
- **WHEN** the user runs `dtwiz update otel` on a Dynatrace-managed collector and confirms
- **THEN** the extension is activated
- **AND** the collector update continues

#### Scenario: Extension activation fails

- **GIVEN** the experimental flag is enabled and a platform token is provided
- **AND** activation fails due to an API error
- **WHEN** the user confirms
- **THEN** the user sees a warning
- **AND** the collector update continues

#### Scenario: Experimental flag disabled

- **GIVEN** the experimental flag is not enabled
- **WHEN** the user runs `dtwiz update otel`
- **THEN** no extension install or activation is attempted

#### Scenario: No platform token provided

- **GIVEN** the experimental flag is enabled but no platform token is available
- **WHEN** the user runs `dtwiz update otel`
- **THEN** no extension install or activation is attempted

#### Scenario: Dry run

- **GIVEN** the experimental flag is enabled and a platform token is provided
- **WHEN** the user runs `dtwiz update otel --dry-run`
- **THEN** no extension install or activation occurs

### Requirement: Extension activation status shown in the Dynatrace collector update preview

When `dtwiz update otel` runs on a Dynatrace-managed OTel Collector with the experimental flag enabled and a platform token provided, the update preview SHALL show the current state of the OTel Host Monitoring extension before the OpenPipeline route plan section and before the confirmation prompt.

#### Scenario: Extension already installed and active — preview shows it

- **GIVEN** the experimental flag is enabled and a platform token is provided
- **AND** the extension is already installed and active
- **WHEN** `dtwiz update otel` prints its preview
- **THEN** the preview shows the extension as already installed and active
- **AND** this section appears before the OpenPipeline route plan section

#### Scenario: Extension not installed — preview shows what will happen

- **GIVEN** the experimental flag is enabled and a platform token is provided
- **AND** the extension is not installed on the tenant
- **WHEN** `dtwiz update otel` prints its preview
- **THEN** the preview shows that the extension will be installed and activated

#### Scenario: Preview check failure does not block the update

- **GIVEN** the extension status cannot be determined due to an API or auth error
- **WHEN** `dtwiz update otel` prints its preview
- **THEN** a warning is shown for the extension preview section
- **AND** the update preview and confirmation continue normally

#### Scenario: Preview check is read-only

- **WHEN** `dtwiz update otel` builds the extension activation preview
- **THEN** no extension install or activation call is made
- **AND** this holds even when `--dry-run` is passed
