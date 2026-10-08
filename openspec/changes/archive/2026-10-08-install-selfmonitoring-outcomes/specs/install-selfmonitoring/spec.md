# Spec: Install Selfmonitoring

## MODIFIED Requirements

### Requirement: Install commands emit an ist event on completion, failure, or cancellation

When a `dtwiz install <method>` subcommand is invoked directly (not via `dtwiz setup`), the system SHALL emit an `ist` step event when the installer returns: whether it succeeded, failed, or was cancelled by the user.

#### Scenario: Successful install emits ist with no error field

- **GIVEN** the user runs `dtwiz install <method>`
- **WHEN** the installer completes successfully
- **THEN** an `ist` event identifying the install command and the method is emitted with no error

#### Scenario: Failed install emits ist with classified error type

- **GIVEN** the user runs `dtwiz install <method>`
- **WHEN** the installer returns a non-cancellation error
- **THEN** an `ist` event is emitted carrying the classified error type from the error taxonomy

#### Scenario: Cancelled install emits ist with error type user_cancelled

- **GIVEN** the user runs `dtwiz install <method>`
- **WHEN** the user declines the install confirmation prompt
- **THEN** an `ist` event is emitted with the user-cancelled error type, and it carries neither an install work time nor feature outcomes, because no install work started

#### Scenario: Dry-run emits ist with no duration

- **GIVEN** `--dry-run` is passed
- **WHEN** the dry-run exits without executing anything
- **THEN** an `ist` event is emitted with no error, and it carries neither an install work time nor feature outcomes

#### Scenario: Pre-install failure emits fai instead of ist

- **GIVEN** the user runs `dtwiz install <method>`
- **WHEN** credential resolution or token validation fails before the installer is called
- **THEN** a `fai` event is emitted with a classified error type, and no `ist` event is emitted

## ADDED Requirements

### Requirement: The ist event carries the install work time

The `ist` event SHALL carry the install work time: the time dtwiz spent doing install work, from the moment the user confirmed the install to the moment the installer's own work ended.

The work time SHALL exclude:

- the time the user spends answering any prompt shown after the install confirmation, while the work done before and after that prompt is kept;
- the post-install watch session, including when the installer runs the watch itself.

The work time SHALL end:

- for AWS, when both the CloudFormation deploy and the Lambda instrumentation have finished, whichever finishes later;
- for Azure and GCP, when the integration setup has finished, before the post-install watch starts;
- for all other methods, when the installer returns. For Kubernetes this includes waiting for the deployed components to become ready.

For methods without an install confirmation, the work time SHALL start when the installer is called.

The work time SHALL be carried both in the event body, with millisecond precision, and in the compact User-Agent form, in whole seconds with an upper bound.

The work time SHALL be present whenever install work started, including when the installer then failed. It SHALL be absent when no install work started: dry-run, cancellation at the install confirmation, or failure before the confirmation.

#### Scenario: Work time excludes think time at the install confirmation

- **GIVEN** the user runs `dtwiz install otel-collector` and waits 30 seconds before confirming
- **WHEN** the installer finishes 20 seconds after the confirmation
- **THEN** the `ist` event's work time is about 20 seconds

#### Scenario: Work time excludes think time at a later prompt

- **GIVEN** the user confirmed `dtwiz install otel-python`, and after 5 seconds of work dtwiz asks whether to recreate a stale virtualenv
- **WHEN** the user answers after 40 seconds and the remaining work takes 10 seconds
- **THEN** the `ist` event's work time is about 15 seconds: the work before the prompt plus the work after it

#### Scenario: Work time excludes prompts shown before the install confirmation

- **GIVEN** the user runs `dtwiz install otel` and is asked to select another project before the install confirmation
- **WHEN** the user takes a minute to choose, then confirms, and the install takes 20 seconds
- **THEN** the `ist` event's work time is about 20 seconds

#### Scenario: Work time for Azure and GCP excludes the watch session

- **GIVEN** the user runs `dtwiz install azure` or `dtwiz install gcp`
- **WHEN** the integration setup finishes and the post-install watch runs for 10 minutes before the installer returns
- **THEN** the `ist` event's work time ends when the integration setup finished and does not include the watch session

#### Scenario: Work time for AWS ends when the slower of deploy and Lambda finishes

- **GIVEN** the user runs `dtwiz install aws`
- **WHEN** the Lambda instrumentation finishes after 20 seconds, the post-install watch starts, and the CloudFormation deploy finishes in the background after 4 minutes
- **THEN** the `ist` event's work time is about 4 minutes, ending when the CloudFormation deploy finished, not when the watch ended

#### Scenario: Work time without an install confirmation

- **GIVEN** the user runs `dtwiz install oneagent` on a host without an existing OneAgent
- **WHEN** the installer completes after 90 seconds
- **THEN** the `ist` event's work time is about 90 seconds, measured from when the installer was called

#### Scenario: Work time is present on failure

- **GIVEN** the user confirmed the install
- **WHEN** the installer fails after 12 seconds of work
- **THEN** the `ist` event carries the classified error type and a work time of about 12 seconds

#### Scenario: Very long work time is bounded only in the User-Agent

- **GIVEN** an install whose work time exceeds the User-Agent upper bound
- **WHEN** the `ist` event is assembled
- **THEN** the User-Agent carries the upper bound and the event body carries the exact work time

### Requirement: The ist event carries feature outcomes

The `ist` event SHALL report, for each feature dtwiz can enable during an install, one of three outcomes:

- **succeeded**: dtwiz tried to enable the feature and it is on afterwards, including when it was already enabled or already current;
- **failed**: dtwiz tried to enable the feature and it is not on afterwards. This applies even when the install as a whole succeeds, because a feature failure does not fail the install;
- **not tried**: dtwiz did not try to enable the feature, because the method does not support it, the conditions for enabling it were not met, or the install failed before reaching that step.

Outcomes SHALL reflect what actually happened during this run, not a fixed value per method.

The features are:

- OTel collector config written
- Host Monitoring enabled
- RUM enabled
- Synthetic Monitoring enabled
- RDS extensions enabled

RUM, Synthetic Monitoring and RDS extensions are reserved: they SHALL always be reported as not tried until an installer enables them.

All features SHALL be reported together, whenever install work started. They SHALL be absent when no install work started: dry-run, cancellation at the install confirmation, or failure before the confirmation.

Only the OTel collector methods (`otel`, `otel-collector`, `demo`) SHALL try the collector config. Those methods and the OneAgent-based ones (`oneagent`, `docker`) SHALL try Host Monitoring. Every other install method SHALL report all features as not tried.

#### Scenario: Successful OTel collector install

- **GIVEN** the user runs `dtwiz install otel-collector`
- **WHEN** the collector config is written and the Host Monitoring extension is activated
- **THEN** the `ist` event reports OTel config and Host Monitoring as succeeded and the three reserved features as not tried

#### Scenario: Host Monitoring activation fails but the install succeeds

- **GIVEN** the user runs `dtwiz install otel`
- **WHEN** the collector config is written but the Host Monitoring extension cannot be activated, and the install otherwise completes
- **THEN** the `ist` event has no error, reports OTel config as succeeded and Host Monitoring as failed

#### Scenario: Host Monitoring extension already active

- **GIVEN** the Host Monitoring extension is already installed and active in the tenant
- **WHEN** the user runs `dtwiz install otel-collector` and the install completes
- **THEN** the `ist` event reports Host Monitoring as succeeded

#### Scenario: Install fails before reaching a feature step

- **GIVEN** the user runs `dtwiz install otel-collector` and confirms
- **WHEN** the Host Monitoring extension is activated and the collector download then fails before the config is written
- **THEN** the `ist` event carries the classified error type, reports Host Monitoring as succeeded, and reports OTel config as not tried

#### Scenario: OneAgent install

- **GIVEN** the user runs `dtwiz install oneagent`
- **WHEN** the OneAgent installer completes successfully
- **THEN** the `ist` event reports Host Monitoring as succeeded and all other features as not tried

#### Scenario: OneAgent install fails

- **GIVEN** the user runs `dtwiz install oneagent`
- **WHEN** the OneAgent installer command fails
- **THEN** the `ist` event carries the classified error type and reports Host Monitoring as failed

#### Scenario: OneAgent connectivity check only

- **GIVEN** the user runs `dtwiz install oneagent` with the connectivity-check-only option
- **WHEN** the connectivity check completes
- **THEN** the `ist` event reports Host Monitoring as not tried, because OneAgent was not installed

#### Scenario: Kubernetes install

- **GIVEN** the user runs `dtwiz install kubernetes`
- **WHEN** the Dynatrace Operator is deployed successfully
- **THEN** the `ist` event reports every feature as not tried

#### Scenario: Language-specific OTel install

- **GIVEN** the user runs `dtwiz install otel-python`
- **WHEN** the install completes successfully
- **THEN** the `ist` event reports every feature as not tried, because no collector config is written and no host monitoring is enabled

#### Scenario: Cancelled or dry-run install has no feature outcomes

- **GIVEN** the user runs `dtwiz install otel` with `--dry-run`, or declines the install confirmation
- **WHEN** the `ist` event is assembled
- **THEN** the event carries no feature outcomes, neither in the User-Agent nor in the body

#### Scenario: User-Agent stays within the capture limit

- **GIVEN** an `ist` event carrying the longest method identifier, an error type, the feature outcomes and a work time at the upper bound
- **WHEN** the User-Agent is assembled for a release version
- **THEN** the User-Agent fits within the capture limit

## REMOVED Requirements

### Requirement: The ist event carries install.duration_s excluding user think time

**Reason**: Replaced by "The ist event carries the install work time". The old value counted the post-install watch for cloud methods, lost work done before a later prompt, and was only carried in the event body.

**Migration**: Queries switch to the new work time property; see the design's wire format for the property name and the conversion from the old value.

### Requirement: The ist event carries per-method feature flags

**Reason**: Replaced by "The ist event carries feature outcomes". The old properties were fixed per method and reported host monitoring as enabled even when activation failed.

**Migration**: Queries switch to the new feature outcome properties and treat only "succeeded" as enabled; see the design's wire format for the property mapping.
