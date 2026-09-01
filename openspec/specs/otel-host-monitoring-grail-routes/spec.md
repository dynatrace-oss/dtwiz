# OTel Host Monitoring: Smartscape on Grail Routes Spec

## Purpose

Define how `dtwiz install otel` sets up dynamic Smartscape-on-Grail routes after a host-monitoring install.

## Requirements

### Requirement: Dynamic routes for Smartscape on Grail are set up after host-monitoring install

When an OTel install flow has the tenant credentials needed to manage OpenPipeline routes, it SHALL attempt to set up the OpenTelemetry Host Monitoring dynamic routes after the user confirms and before the install emits verification telemetry or starts selected application instrumentation. Before telemetry is emitted, the install SHALL validate, with a bounded wait, that routes successfully applied by the install are visible as enabled. Final route status output SHALL be printed after validation, so successful create or re-enable messages are only shown after the corresponding route is visible as enabled. Route setup and validation failures SHALL remain advisory, but their warning messages SHALL be distinct: route setup failures indicate that creating or re-enabling the route failed, while route visibility validation failures indicate that the write succeeded but the route was not confirmed visible yet and may become active later.

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
- **THEN** the final route status output shows a validation warning for the affected signal instead of a successful create or re-enable message
- **AND** the warning explains that the route write succeeded but visibility could not be confirmed yet
- **AND** the warning explains that the route may become active later
- **AND** the collector install continues
- **AND** verification or selected application instrumentation proceeds after the warning

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

#### Scenario: Route setup failure does not block telemetry startup

- **GIVEN** a dynamic route cannot be created or re-enabled after the user confirms an OTel install (for example, due to a transient API error)
- **WHEN** the route apply step runs
- **THEN** a warning SHALL be printed identifying the affected signal and the error
- **AND** the warning SHALL be distinct from a route visibility validation warning
- **AND** the overall `install otel` result SHALL remain successful
- **AND** the collector install SHALL remain in place
- **AND** verification or selected application instrumentation proceeds after the warning

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

### Requirement: Dynamic routes are checked and set up after Dynatrace collector update

When `dtwiz update otel` runs on a Dynatrace-managed OTel Collector with the experimental flag enabled and a platform token provided, the update SHALL ensure dynamic OpenPipeline routes exist for metrics, logs, and spans after the user confirms, using the same additive and idempotent logic as `install otel`.

#### Scenario: All three routes missing

- **GIVEN** the experimental flag is enabled and a platform token is provided
- **AND** the OTel host monitoring extension's pipelines exist for metrics, logs, and spans
- **AND** no dynamic routes target those pipelines
- **WHEN** the user runs `dtwiz update otel` on a Dynatrace-managed collector and confirms
- **THEN** a dynamic route is created for each of metrics, logs, and spans targeting the extension's pipeline
- **AND** the collector update continues

#### Scenario: Routes already configured

- **GIVEN** all three routes already exist and are enabled
- **WHEN** the user runs `dtwiz update otel` and confirms
- **THEN** no dynamic route is created, modified, or deleted

#### Scenario: Disabled route is re-enabled

- **GIVEN** a dynamic route targets the extension's pipeline for a signal type but is disabled
- **WHEN** the user runs `dtwiz update otel` and confirms
- **THEN** that route is re-enabled
- **AND** its matcher, description, and other properties remain unchanged

#### Scenario: Pipeline not found for a signal type

- **GIVEN** the extension's pipeline does not exist for a signal type when routes are applied
- **WHEN** the user confirms
- **THEN** the route for that signal type is skipped
- **AND** routes for signal types whose pipeline does exist are still set up
- **AND** the overall update result remains successful

#### Scenario: Experimental flag disabled

- **GIVEN** the experimental flag is not enabled
- **WHEN** the user runs `dtwiz update otel`
- **THEN** no OpenPipeline dynamic route is read or written

#### Scenario: No platform token provided

- **GIVEN** the experimental flag is enabled but no platform token is available
- **WHEN** the user runs `dtwiz update otel`
- **THEN** no OpenPipeline dynamic route is read or written

#### Scenario: Dry run

- **GIVEN** the experimental flag is enabled and a platform token is provided
- **WHEN** the user runs `dtwiz update otel --dry-run`
- **THEN** the route plan is shown in the preview
- **AND** no dynamic route is created, modified, or deleted

#### Scenario: Route application failure shown as warning

- **GIVEN** a route write fails after the user confirms
- **WHEN** the route apply step runs
- **THEN** a warning is printed identifying the affected signal and the error
- **AND** the overall update result remains successful
- **AND** the collector update continues

### Requirement: Route plan shown in the Dynatrace collector update preview; no separate confirmation

When `dtwiz update otel` runs on a Dynatrace-managed OTel Collector with the experimental flag enabled and a platform token provided, the planned route changes SHALL be shown as part of the update preview before the existing confirmation prompt. No separate confirmation SHALL be shown for routes.

#### Scenario: Route plan shown in update preview

- **WHEN** `dtwiz update otel` prints its preview
- **THEN** the planned action for each of metrics, logs, and spans is printed one line each
- **AND** each line shows whether the route will be created, re-enabled, already exists, or is pending on the extension not being active yet
- **AND** this section appears after the extension status section and before the single confirmation prompt
- **AND** no additional confirmation prompt is shown for routes alone

#### Scenario: Route plan is re-evaluated before applying

- **GIVEN** a signal's route was shown as skipped in the preview because its pipeline did not exist yet
- **AND** the extension is installed and activated by this same run after the preview and before routes are applied
- **WHEN** routes are applied
- **THEN** the route decision for that signal is re-evaluated against the pipeline's current state
- **AND** the route is created if the pipeline now exists

#### Scenario: Bounded wait for pipelines before route plan is applied

- **WHEN** `dtwiz update otel` applies routes after confirmation
- **THEN** the update waits, up to a bounded number of attempts, for at least one signal's pipeline to become available before evaluating the final route plan
- **AND** a pipeline already present is found on the first attempt without unnecessary delay
