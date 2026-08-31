## MODIFIED Requirements

### Requirement: Dynamic routes for Smartscape on Grail are set up before OTel install telemetry is emitted

When an OTel install flow has the tenant credentials needed to manage OpenPipeline routes, it SHALL attempt to set up the OpenTelemetry Host Monitoring dynamic routes after the user confirms and before the install emits verification telemetry or starts selected application instrumentation. Before telemetry is emitted, the install SHALL validate, with a bounded wait, that routes successfully applied by the install are visible as enabled. Route setup and validation failures SHALL remain advisory: the user is warned and the install continues.

#### Scenario: Collector-only install sends verification after routes are applied

- **GIVEN** the OpenTelemetry Host Monitoring pipelines exist for logs, metrics, and spans
- **AND** the required dynamic routes do not yet exist
- **WHEN** the user runs `dtwiz install otel` and does not select a project to instrument
- **THEN** the dynamic routes are set up before the collector verification log is sent
- **AND** successfully applied routes are validated as visible and enabled before the verification log is sent
- **AND** the collector install continues after route setup

#### Scenario: Selected project instrumentation starts after routes are applied

- **GIVEN** the OpenTelemetry Host Monitoring pipelines exist for logs, metrics, and spans
- **AND** the required dynamic routes do not yet exist
- **WHEN** the user runs `dtwiz install otel` and selects a project to instrument
- **THEN** the dynamic routes are set up before the selected application instrumentation is started
- **AND** successfully applied routes are validated as visible and enabled before the selected application instrumentation is started
- **AND** the collector install and application instrumentation continue after route setup

#### Scenario: Route visibility validation times out

- **GIVEN** a route create or re-enable operation succeeds
- **AND** the route is not visible as enabled before the bounded validation wait ends
- **WHEN** the install validates route setup
- **THEN** the user sees a warning that route validation failed
- **AND** the collector install continues
- **AND** verification or selected application instrumentation proceeds after the warning

#### Scenario: Route setup failure does not block telemetry startup

- **GIVEN** a dynamic route cannot be created or re-enabled after the user confirms an OTel install
- **WHEN** the install applies the route plan
- **THEN** the user sees a warning for the affected signal
- **AND** the collector install continues
- **AND** verification or selected application instrumentation proceeds after the warning

#### Scenario: Dry-run emits no telemetry and writes no routes

- **GIVEN** the user runs an OTel install with `--dry-run`
- **WHEN** the install preview is rendered
- **THEN** no verification telemetry is emitted
- **AND** no selected application instrumentation is started
- **AND** no dynamic route is created, modified, or deleted