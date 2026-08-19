## MODIFIED Requirements

### Requirement: OTel install ensures host monitoring extension is active

When `dtwiz install otel` runs, the installer SHALL ensure the OpenTelemetry Host Monitoring extension is installed and active in the tenant before the OTel Collector starts. If the extension cannot be installed or activated, the installer SHALL warn the user and proceed rather than abort.

#### Scenario: Extension not present on tenant

- **GIVEN** the extension is not installed on the tenant
- **WHEN** the user runs `dtwiz install otel`
- **THEN** the extension is installed and activated, and the OTel Collector install continues

#### Scenario: Extension already installed and active

- **GIVEN** the extension is already installed and active
- **WHEN** the user runs `dtwiz install otel`
- **THEN** the OTel Collector install proceeds without re-installing or re-activating the extension

#### Scenario: Extension installed but not active

- **GIVEN** the extension is installed but not yet active
- **WHEN** the user runs `dtwiz install otel`
- **THEN** the extension is activated and the OTel Collector install continues

#### Scenario: Extension activation fails

- **GIVEN** activation fails
- **WHEN** the user runs `dtwiz install otel`
- **THEN** the user sees a warning and the OTel Collector install continues

#### Scenario: Dry run

- **GIVEN** the user runs `dtwiz install otel --dry-run`
- **WHEN** the install preview is rendered
- **THEN** no extension install or activation occurs
