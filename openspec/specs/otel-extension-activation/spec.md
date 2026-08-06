# Spec: otel-extension-activation

## Purpose

Ensure the Dynatrace OpenTelemetry Host Monitoring extension is installed and active on a tenant before the OTel Collector starts, so that host and process entities are created and Infrastructure & Operations visualizations appear.

## Requirements

### Requirement: Extension activation

The system SHALL provide a way to activate a specific version of a Dynatrace extension in an environment. Activating a version that is already active SHALL be treated as success.

#### Scenario: Activate an extension version

- **GIVEN** a Dynatrace extension is installed in the environment
- **WHEN** activation is requested for a specific version
- **THEN** that version becomes the active environment configuration

#### Scenario: Activating an already-active version is a no-op

- **GIVEN** a specific extension version is already active in the environment
- **WHEN** activation is requested for that same version
- **THEN** the operation succeeds without error

#### Scenario: Activation failure is reported

- **GIVEN** the Dynatrace API is unavailable or rejects the request
- **WHEN** activation is requested
- **THEN** an error is returned to the caller

### Requirement: OTel install ensures host monitoring extension is active

When `dtwiz install otel` runs with the experimental flag enabled, the installer SHALL ensure the OpenTelemetry Host Monitoring extension is installed and active in the tenant before the OTel Collector starts. If the extension cannot be installed or activated, the installer SHALL warn the user and proceed rather than abort.

#### Scenario: Extension not present on tenant

- **GIVEN** the experimental flag is enabled and the extension is not installed on the tenant
- **WHEN** the user runs `dtwiz install otel`
- **THEN** the extension is installed and activated, and the OTel Collector install continues

#### Scenario: Extension already installed and active

- **GIVEN** the experimental flag is enabled and the extension is already installed and active
- **WHEN** the user runs `dtwiz install otel`
- **THEN** the OTel Collector install proceeds without re-installing or re-activating the extension

#### Scenario: Extension installed but not active

- **GIVEN** the experimental flag is enabled and the extension is installed but not yet active
- **WHEN** the user runs `dtwiz install otel`
- **THEN** the extension is activated and the OTel Collector install continues

#### Scenario: Extension activation fails

- **GIVEN** the experimental flag is enabled and activation fails
- **WHEN** the user runs `dtwiz install otel`
- **THEN** the user sees a warning and the OTel Collector install continues

#### Scenario: Experimental flag disabled

- **GIVEN** the experimental flag is not enabled
- **WHEN** the user runs `dtwiz install otel`
- **THEN** no extension install or activation is attempted

#### Scenario: Dry run

- **GIVEN** the experimental flag is enabled
- **WHEN** the user runs `dtwiz install otel --dry-run`
- **THEN** no extension install or activation occurs
