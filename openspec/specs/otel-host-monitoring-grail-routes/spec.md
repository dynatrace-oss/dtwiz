# OTel Host Monitoring: Smartscape on Grail Routes Spec

## Purpose

Define how `dtwiz install otel` sets up dynamic Smartscape-on-Grail routes after a host-monitoring install.

## Requirements

### Requirement: Dynamic routes for Smartscape on Grail are set up after host-monitoring install

After the managed OTel Collector host-monitoring install completes successfully, `install otel` SHALL ensure a dynamic route exists for each of metrics, logs, and spans that routes OpenTelemetry host telemetry into the OTel host monitoring extension's pipeline, using the documented matching conditions.

#### Scenario: All three routes created when absent

- **GIVEN** the OTel host monitoring extension's pipeline exists for metrics, logs, and spans
- **AND** no equivalent dynamic route yet targets that pipeline for any of the three signal types
- **WHEN** `install otel` finishes installing the host-monitoring collector
- **THEN** a dynamic route SHALL be created for metrics with the condition `matchesValue(metric.key, {"system.*", "process.*"}) AND isNotNull(host.id)`
- **AND** a dynamic route SHALL be created for logs with the condition `isNotNull(host.id) and isNotNull(host.name) and matchesValue(dt.openpipeline.source, "/api/v2/otlp/v1/logs")`
- **AND** a dynamic route SHALL be created for spans with the condition `isNotNull(host.id) and isNotNull(host.name) and matchesValue(telemetry.sdk.name, {"opentelemetry", "odin", "otel"})`
- **AND** each route SHALL target the OTel host monitoring extension's pipeline for its signal type

#### Scenario: Route target resolved per environment

- **WHEN** `install otel` sets up the routes on a given environment
- **THEN** the route for each signal type SHALL target the pipeline provided by the OTel host monitoring extension for that signal type in the current environment

### Requirement: Route setup is additive and idempotent

Setting up routes SHALL only add routes that are missing and SHALL never modify or delete existing routes, so that re-running `install otel` after the routes exist makes no changes.

#### Scenario: Existing enabled route is left untouched

- **GIVEN** a dynamic route already targets the OTel host monitoring extension's pipeline for a signal type and is enabled
- **WHEN** `install otel` sets up the routes
- **THEN** that route SHALL NOT be modified or deleted
- **AND** no duplicate route SHALL be created for that signal type

#### Scenario: Disabled route is re-enabled

- **GIVEN** a dynamic route targets the OTel host monitoring extension's pipeline for a signal type but is disabled
- **WHEN** `install otel` sets up the routes
- **THEN** that route SHALL be re-enabled
- **AND** its matcher, description, and all other properties SHALL remain unchanged
- **AND** all other routes in the same signal table SHALL remain unchanged

#### Scenario: Re-running is a no-op

- **GIVEN** all three routes already exist from a previous run
- **WHEN** `install otel` is run again
- **THEN** no dynamic route SHALL be created, modified, or deleted

#### Scenario: User-authored routes are preserved

- **GIVEN** a user has manually created or broadened a dynamic route to the OTel host monitoring extension's pipeline for a signal type
- **WHEN** `install otel` sets up the routes
- **THEN** that user route SHALL be treated as already routing to the pipeline and left unchanged
- **AND** no second route SHALL be added for that signal type

### Requirement: Missing target pipeline is skipped safely

When the OTel host monitoring extension's pipeline is not found for a signal type at the time the routes are actually applied, `install otel` SHALL skip that route and continue, and SHALL NOT fail the install. The skip message SHALL NOT instruct the user to activate the extension themselves, since `install otel` already attempts that as part of the same run whenever the route step runs at all.

#### Scenario: Pipeline not found for a signal type

- **GIVEN** the OTel host monitoring extension's pipeline does not exist for a signal type when the routes are applied (for example, the extension could not be activated, or its pipelines have not yet propagated)
- **WHEN** `install otel` sets up the routes
- **THEN** the route for that signal type SHALL be skipped with a message directing the user to re-run `install otel`, not to activate the extension themselves
- **AND** routes for signal types whose pipeline does exist SHALL still be set up
- **AND** the overall `install otel` result SHALL remain successful

### Requirement: Route plan shown in the install preview; no separate confirmation

The planned route changes SHALL be shown as part of the main install preview before the existing "Proceed with installation?" prompt. No separate confirmation prompt SHALL be shown for routes.

#### Scenario: Route plan shown in install preview

- **WHEN** `install otel` prints its install preview
- **THEN** the planned action for each of metrics, logs, and spans SHALL be printed one line each, showing whether the route will be created, re-enabled, already exists, or is pending on the extension not being active yet
- **AND** this section SHALL appear before the single install confirmation prompt
- **AND** no additional confirmation prompt SHALL be shown for routes alone
- **AND** the preview SHALL NOT describe a route as skipped, since nothing has actually been attempted yet at preview time; a final skip is only ever reported after the routes have actually been applied

#### Scenario: Route plan is re-evaluated before applying

- **GIVEN** a signal's route was shown as skipped in the install preview because its pipeline did not exist yet
- **AND** the OTel host monitoring extension is installed and activated by this same `install otel` run, after the preview and before the routes are applied
- **WHEN** the routes are applied
- **THEN** the route decision for that signal SHALL be re-evaluated against the pipeline's current state rather than reusing the preview's decision
- **AND** the route SHALL be created if the pipeline now exists

#### Scenario: Bounded wait for a pipeline to become listable

- **WHEN** `install otel` sets up the routes
- **THEN** `install otel` SHALL wait, up to a bounded number of attempts, for at least one signal's pipeline to become listable before evaluating the route plan
- **AND** this wait SHALL apply the same way whether or not the extension was already installed before this run, so that a pipeline already present is found on the first attempt

#### Scenario: Dry-run writes nothing

- **GIVEN** `install otel --dry-run`
- **WHEN** the command runs
- **THEN** the route plan SHALL be shown as part of the install preview
- **AND** no dynamic route SHALL be created, modified, or deleted

#### Scenario: Route application failure shown as warning

- **GIVEN** a route write fails after the user confirms the install (for example, due to a transient API error)
- **WHEN** the route apply step runs
- **THEN** a warning SHALL be printed identifying the affected signal and the error
- **AND** the overall `install otel` result SHALL remain successful
- **AND** the collector install SHALL remain in place

### Requirement: Extension activation status shown in the install preview, before the route plan

The install preview SHALL show the current state of the OTel host monitoring extension activation step, read-only, before the OpenPipeline route plan section. This lets the user see, before confirming, that the extension is installed ahead of the routes being applied, since dynamic routes are only meaningful once the extension's pipeline exists.

#### Scenario: Extension already installed

- **GIVEN** the OTel host monitoring extension is already installed, in any activation state
- **WHEN** `install otel` prints its install preview
- **THEN** the preview SHALL show the extension as already installed
- **AND** this line SHALL appear before the OpenPipeline route plan section
- **AND** the preview SHALL NOT claim a specific activation outcome (active vs. inactive)

#### Scenario: Extension not installed

- **GIVEN** the OTel host monitoring extension is not installed on the tenant
- **WHEN** `install otel` prints its install preview
- **THEN** the preview SHALL show that the extension will be installed and activated

#### Scenario: Preview check failure does not block the install

- **GIVEN** the extension status cannot be determined (for example, an API or auth error)
- **WHEN** `install otel` prints its install preview
- **THEN** a warning SHALL be shown for the extension preview section
- **AND** the install preview and confirmation SHALL continue normally

#### Scenario: Preview check is read-only

- **WHEN** `install otel` builds the extension activation preview
- **THEN** no extension install or activation call SHALL be made
- **AND** this holds even when `--dry-run` is passed

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
