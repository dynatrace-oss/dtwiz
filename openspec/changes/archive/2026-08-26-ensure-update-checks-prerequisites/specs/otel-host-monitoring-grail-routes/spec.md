# Spec: otel-host-monitoring-grail-routes

## ADDED Requirements

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
