# OTel Host Monitoring: Smartscape on Grail Routes Spec

## ADDED Requirements

### Requirement: Dynamic routes for Smartscape on Grail are set up after host-monitoring install

After the managed OTel Collector host-monitoring install completes successfully, `install otel` SHALL ensure a dynamic route exists for each of metrics, logs, and spans that routes OpenTelemetry host telemetry into the "OpenTelemetry Host Monitoring" pipeline, using the documented matching conditions.

#### Scenario: All three routes created when absent

- **GIVEN** the "OpenTelemetry Host Monitoring" pipeline exists for metrics, logs, and spans
- **AND** no equivalent dynamic route yet targets that pipeline for any of the three signal types
- **WHEN** `install otel` finishes installing the host-monitoring collector
- **THEN** a dynamic route SHALL be created for metrics with the condition `matchesValue(metric.key, {"system.*", "process.*"}) AND isNotNull(host.id)`
- **AND** a dynamic route SHALL be created for logs with the condition `isNotNull(host.id) and isNotNull(host.name) and matchesValue(dt.openpipeline.source, "/api/v2/otlp/v1/logs")`
- **AND** a dynamic route SHALL be created for spans with the condition `isNotNull(host.id) and isNotNull(host.name) and matchesValue(telemetry.sdk.name, {"opentelemetry", "odin", "otel"})`
- **AND** each route SHALL target the "OpenTelemetry Host Monitoring" pipeline for its signal type

#### Scenario: Route target resolved per environment

- **WHEN** `install otel` sets up the routes
- **THEN** the target pipeline for each signal type SHALL be resolved by locating the "OpenTelemetry Host Monitoring" pipeline in that signal type's dynamic-routing configuration in the current environment
- **AND** the route SHALL reference that resolved pipeline rather than any hardcoded identifier

### Requirement: Route setup is additive and idempotent

Setting up routes SHALL only add routes that are missing and SHALL never modify or delete existing routes, so that re-running `install otel` after the routes exist makes no changes.

#### Scenario: Existing enabled route is left untouched

- **GIVEN** a dynamic route already targets the "OpenTelemetry Host Monitoring" pipeline for a signal type and is enabled
- **WHEN** `install otel` sets up the routes
- **THEN** that route SHALL NOT be modified or deleted
- **AND** no duplicate route SHALL be created for that signal type

#### Scenario: Disabled route is re-enabled

- **GIVEN** a dynamic route targets the "OpenTelemetry Host Monitoring" pipeline for a signal type but is disabled
- **WHEN** `install otel` sets up the routes
- **THEN** that route SHALL be re-enabled
- **AND** its matcher, description, and all other fields SHALL remain unchanged
- **AND** all other routes in the same signal table SHALL remain unchanged

#### Scenario: Re-running is a no-op

- **GIVEN** all three routes already exist from a previous run
- **WHEN** `install otel` is run again
- **THEN** no dynamic route SHALL be created, modified, or deleted

#### Scenario: User-authored routes are preserved

- **GIVEN** a user has manually created or broadened a dynamic route to the "OpenTelemetry Host Monitoring" pipeline for a signal type
- **WHEN** `install otel` sets up the routes
- **THEN** that user route SHALL be recognized as already routing to the pipeline and left unchanged
- **AND** no second route SHALL be added for that signal type

### Requirement: Missing target pipeline is skipped safely

When the "OpenTelemetry Host Monitoring" pipeline cannot be resolved for a signal type, `install otel` SHALL skip that route and continue, and SHALL NOT fail the install.

#### Scenario: Pipeline not found for a signal type

- **GIVEN** the "OpenTelemetry Host Monitoring" pipeline does not exist for a signal type (for example the extension is not activated)
- **WHEN** `install otel` sets up the routes
- **THEN** the route for that signal type SHALL be skipped with an informational message
- **AND** routes for signal types whose pipeline does exist SHALL still be set up
- **AND** the overall `install otel` result SHALL remain successful

### Requirement: Route plan shown in the install preview; no separate confirmation

The planned route changes SHALL be shown as part of the main install preview before the existing "Proceed with installation?" prompt. No separate confirmation prompt SHALL be shown for routes.

#### Scenario: Route plan shown in install preview

- **WHEN** `install otel` prints its install preview
- **THEN** the planned action for each of metrics, logs, and spans SHALL be printed one line each, showing whether the route will be created, re-enabled, already exists, or is skipped
- **AND** this section SHALL appear before the single "Proceed with installation?" prompt
- **AND** no additional confirmation prompt SHALL be shown for routes alone

#### Scenario: Dry-run writes nothing

- **GIVEN** `install otel --dry-run`
- **WHEN** the command runs
- **THEN** the route plan SHALL be shown as part of the install preview
- **AND** no dynamic route SHALL be created, modified, or deleted

#### Scenario: Route application failure shown as warning

- **GIVEN** a route write fails after the user confirms the install (for example due to a transient API error)
- **WHEN** the route apply step runs
- **THEN** a warning SHALL be printed identifying the affected signal and the error
- **AND** the overall `install otel` result SHALL remain successful
- **AND** the collector install SHALL remain in place

### Requirement: Route setup is gated behind the experimental flag

`install otel` SHALL only set up OpenPipeline dynamic routes when the `--experimental` flag or `DTWIZ_EXPERIMENTAL=true` environment variable is enabled. When the flag is not enabled, `install otel` SHALL make no OpenPipeline changes and SHALL behave exactly as it did before this change.

#### Scenario: Routes not touched when flag disabled

- **GIVEN** `--experimental` is not set and `DTWIZ_EXPERIMENTAL` is not `true`
- **WHEN** `install otel` runs
- **THEN** no OpenPipeline dynamic route SHALL be read or written
- **AND** no route preview or route confirmation prompt SHALL be shown

#### Scenario: Routes set up when flag enabled

- **GIVEN** `--experimental` is passed or `DTWIZ_EXPERIMENTAL=true` is set
- **AND** the host-monitoring collector install has completed successfully
- **WHEN** `install otel` runs without `--dry-run`
- **THEN** dtwiz SHALL set up the three dynamic routes as described in the requirements above
