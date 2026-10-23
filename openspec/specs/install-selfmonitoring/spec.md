# Spec: Install Self-Monitoring

## Purpose

Instrument each `dtwiz install <method>` subcommand with self-monitoring telemetry events so that install sessions are observable end-to-end: invocation, install completion or failure (including user cancellation), and first-data confirmation in the post-install watch session.

## Event flow per session

For every `dtwiz install <method>` invocation (not via `dtwiz setup`), the install command emits these self-monitoring events:

1. `inv` (invoked): fired immediately when the command starts, before any install logic. Ensures the invocation is recorded even if the process is killed mid-install.
2. `ist` (install): fired when the installer returns, only if install work started. Carries the install work time and the feature outcomes, never an error.
3. Exactly one terminal event when the installer returns: `com` (completed) on success, including dry-run; `can` (cancelled) when the user declines the install confirmation; `fai` (failed) with the classified error type on any other error.

Pre-install failures (credential lookup, token validation, client setup) emit a single `fai` and no `ist`: the installer was never reached.

A post-install watch session emits its own `snp` and `com` events under the `watch` command, sharing the install run's execution identifier. The watch session's `com` is not the install command's terminal event.

## Requirements

### Requirement: Install commands emit an inv event on invocation

When a `dtwiz install <method>` subcommand is invoked directly, the system SHALL emit an `inv` step event immediately when the command starts, before any install logic executes.

#### Scenario: inv fires before install logic

- **WHEN** the user runs `dtwiz install <method>`
- **THEN** an `inv` event is emitted immediately in `PersistentPreRun`, before credential resolution or install execution

### Requirement: Install commands emit an ist event when install work started

When a `dtwiz install <method>` subcommand is invoked directly (not via `dtwiz setup`), the system SHALL emit an `ist` step event when the installer returns, if and only if install work started. The `ist` event SHALL identify the install command and the method, and SHALL carry the install work time and the feature outcomes.

The `ist` event SHALL NOT carry an error type or error attributes. How the run ended is reported by the terminal event.

Install work has not started for a dry-run, for a declined install confirmation, or for a failure before the install confirmation. In those cases no `ist` event SHALL be emitted.

#### Scenario: Successful install emits ist

- **GIVEN** the user runs `dtwiz install <method>` and confirms
- **WHEN** the installer completes successfully
- **THEN** an `ist` event identifying the install command and the method is emitted with the work time and feature outcomes and no error

#### Scenario: Install failing after work started emits ist without error

- **GIVEN** the user runs `dtwiz install <method>` and confirms
- **WHEN** the installer fails after install work started
- **THEN** an `ist` event is emitted with the work time and the feature outcomes reached so far, and it carries no error type

#### Scenario: Declined confirmation emits no ist

- **GIVEN** the user runs `dtwiz install <method>`
- **WHEN** the user declines the install confirmation prompt
- **THEN** no `ist` event is emitted

#### Scenario: Dry-run emits no ist

- **GIVEN** `--dry-run` is passed
- **WHEN** the dry-run exits without executing anything
- **THEN** no `ist` event is emitted

#### Scenario: Failure before the confirmation emits no ist

- **GIVEN** the user runs `dtwiz install otel-node` on a host without npm
- **WHEN** the installer fails its dependency check before showing the install confirmation
- **THEN** no `ist` event is emitted

#### Scenario: Pre-install failure emits no ist

- **GIVEN** the user runs `dtwiz install <method>`
- **WHEN** credential resolution or token validation fails before the installer is called
- **THEN** no `ist` event is emitted

### Requirement: Install commands end with exactly one terminal event

When a `dtwiz install <method>` subcommand is invoked directly (not via `dtwiz setup`) and the installer is reached, the system SHALL emit exactly one terminal event for the install command when the installer returns, after the `ist` event if one is emitted:

- `com` (completed) with no error when the installer succeeds, including dry-run;
- `can` (cancelled) with the user-cancelled error type when the user declines the install confirmation;
- `fai` (failed) with the classified error type and its error attributes when the installer returns any other error.

The terminal event SHALL identify the install command and the method.

When a pre-install failure (credential resolution, token validation, client setup) occurs, the system SHALL emit a single `fai` event with the classified error type and SHALL NOT emit a second terminal event.

Every direct install run that is not interrupted SHALL therefore produce exactly one of `com`, `fai`, or `can` for the install command, so that a run whose `inv` has no matching terminal event identifies a process that ended early.

The `ist` and terminal events are sent independently; their arrival order in the tenant is not guaranteed. They carry no overlapping data and are related by the execution identifier.

#### Scenario: Successful install ends with com

- **GIVEN** the user runs `dtwiz install otel-collector` and confirms
- **WHEN** the installer completes successfully
- **THEN** an `ist` event and a `com` event with no error are emitted for the install command and method

#### Scenario: Dry-run ends with com only

- **GIVEN** the user runs `dtwiz install kubernetes --dry-run`
- **WHEN** the dry-run exits without executing anything
- **THEN** a `com` event with no error is emitted, and no `ist` event

#### Scenario: Declined confirmation ends with can only

- **GIVEN** the user runs `dtwiz install otel`
- **WHEN** the user declines the install confirmation
- **THEN** a `can` event with the user-cancelled error type is emitted, and no `ist` event

#### Scenario: Failure after work started ends with ist and fai

- **GIVEN** the user runs `dtwiz install otel-collector` and confirms
- **WHEN** the Host Monitoring extension is activated and the collector download then fails
- **THEN** an `ist` event reporting Host Monitoring as succeeded and OTel config as not tried is emitted, and a `fai` event carrying the classified error type

#### Scenario: Failure before the confirmation ends with fai only

- **GIVEN** the user runs `dtwiz install otel-node` on a host without npm
- **WHEN** the installer returns a dependency-missing error before the install confirmation
- **THEN** a `fai` event is emitted carrying the dependency-missing error type and the missing dependency name, and no `ist` event

#### Scenario: Pre-install failure sends a single fai

- **GIVEN** the user runs `dtwiz install oneagent` without a platform token
- **WHEN** credential resolution fails before the installer is called
- **THEN** exactly one `fai` event is emitted for the install command, and no `ist` event

#### Scenario: Terminal event is sent before a watch run by the command

- **GIVEN** the user runs `dtwiz install oneagent` and the install succeeds
- **WHEN** the command starts the post-install watch session
- **THEN** the install command's `com` event is sent before the watch session sends its first event

#### Scenario: Terminal event is sent after a watch run by the installer

- **GIVEN** the user runs `dtwiz install azure`, whose installer runs the post-install watch itself
- **WHEN** the watch session ends and the installer returns successfully
- **THEN** the watch session's events are sent under the `watch` command first, then the install command's `ist` and `com` events

#### Scenario: Experimental methods end with a terminal event

- **GIVEN** `DTWIZ_EXPERIMENTAL` is enabled
- **WHEN** `dtwiz install docker` completes successfully
- **THEN** an `ist` event and a `com` event are emitted for the install command and method

### Requirement: Post-install watch events are reported under the watch command

When a `dtwiz install <method>` subcommand runs a post-install watch session, the session SHALL emit its own events, using the same encoding as `dtwiz setup` and `dtwiz watch`: one `snp` event each time the set of signal types with data grows, and one `com` event when the session ends.

These events SHALL be reported under the `watch` command, not the install command, and SHALL share the execution identifier of the install run. The watch session's `com` event SHALL NOT be treated as the install command's terminal event; the install command emits its own terminal event.

#### Scenario: snp event fires on first data after install

- **GIVEN** `dtwiz install <method>` ran successfully
- **WHEN** the post-install watch session detects its first signal data
- **THEN** an `snp` event is emitted asynchronously under the `watch` command, carrying the time to first data per signal, without interrupting the watch display

#### Scenario: Watch com event fires when the session ends

- **GIVEN** `dtwiz install <method>` ran successfully and started a post-install watch session
- **WHEN** the session ends, because the user exits or the session times out
- **THEN** a `com` event is emitted under the `watch` command, carrying no signal timings

#### Scenario: Install and watch completion are distinguishable

- **GIVEN** `dtwiz install <method>` ran successfully and its post-install watch session ended
- **WHEN** the events of the run are queried by execution identifier
- **THEN** there is one `com` event for the install command and one `com` event for the `watch` command

### Requirement: Install methods cover all production and experimental subcommands

The following install subcommands SHALL be instrumented with `inv`, `ist` (when install work started), a terminal event (`com`, `fai`, or `can`), and, when a post-install watch session runs, the watch session's events:

- Production: `oneagent`, `kubernetes`, `otel`, `otel-collector`, `otel-python`, `otel-node`, `otel-java`, `aws`, `aws-lambda`, `azure`, `gcp`, `demo`
- Experimental (gated by `DTWIZ_EXPERIMENTAL`): `docker`

#### Scenario: All production methods emit ist

- **WHEN** any production install subcommand completes after install work started
- **THEN** an `ist` event with the correct `ins` command and method subcommand shortcode is emitted

#### Scenario: All production methods emit a terminal event

- **WHEN** any production install subcommand returns from its installer
- **THEN** exactly one terminal event with the correct `ins` command and method subcommand shortcode is emitted

#### Scenario: Experimental methods emit ist when experimental flag is enabled

- **GIVEN** `DTWIZ_EXPERIMENTAL` is enabled
- **WHEN** `dtwiz install docker` completes after install work started
- **THEN** an `ist` event with command `ins` and subcommand shortcode `dock` is emitted

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

The work time SHALL be present on every `ist` event, including when the installer failed after install work started. Because no `ist` event is emitted when no install work started, there is no `ist` event without a work time.

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
- **THEN** the `ist` event carries a work time of about 12 seconds and no error type, and the `fai` event carries the classified error type

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

All features SHALL be reported together on every `ist` event.

Only the OTel collector methods (`otel`, `otel-collector`, `demo`) SHALL try the collector config. Those methods and the OneAgent-based ones (`oneagent`, `docker`) SHALL try Host Monitoring. Every other install method SHALL report all features as not tried.

#### Scenario: Successful OTel collector install

- **GIVEN** the user runs `dtwiz install otel-collector`
- **WHEN** the collector config is written and the Host Monitoring extension is activated
- **THEN** the `ist` event reports OTel config and Host Monitoring as succeeded and the three reserved features as not tried

#### Scenario: Host Monitoring activation fails but the install succeeds

- **GIVEN** the user runs `dtwiz install otel`
- **WHEN** the collector config is written but the Host Monitoring extension cannot be activated, and the install otherwise completes
- **THEN** the `ist` event reports OTel config as succeeded and Host Monitoring as failed, and the terminal event is `com`

#### Scenario: Host Monitoring extension already active

- **GIVEN** the Host Monitoring extension is already installed and active in the tenant
- **WHEN** the user runs `dtwiz install otel-collector` and the install completes
- **THEN** the `ist` event reports Host Monitoring as succeeded

#### Scenario: Install fails before reaching a feature step

- **GIVEN** the user runs `dtwiz install otel-collector` and confirms
- **WHEN** the Host Monitoring extension is activated and the collector download then fails before the config is written
- **THEN** the `ist` event reports Host Monitoring as succeeded and OTel config as not tried, and the `fai` event carries the classified error type

#### Scenario: OneAgent install

- **GIVEN** the user runs `dtwiz install oneagent`
- **WHEN** the OneAgent installer completes successfully
- **THEN** the `ist` event reports Host Monitoring as succeeded and all other features as not tried

#### Scenario: OneAgent install fails

- **GIVEN** the user runs `dtwiz install oneagent`
- **WHEN** the OneAgent installer command fails
- **THEN** the `ist` event reports Host Monitoring as failed, and the `fai` event carries the classified error type

#### Scenario: OneAgent connectivity check only

- **GIVEN** the user runs `dtwiz install oneagent` with the connectivity-check-only option
- **WHEN** the connectivity check completes
- **THEN** the `ist` event reports Host Monitoring as not tried, because OneAgent was not installed

#### Scenario: Kubernetes install

- **GIVEN** the user runs `dtwiz install kubernetes`
- **WHEN** the Dynatrace Operator is deployed successfully
- **THEN** the `ist` event reports every feature as not tried

#### Scenario: Cancelled or dry-run install has no feature outcomes

- **GIVEN** the user runs `dtwiz install otel` with `--dry-run`, or declines the install confirmation
- **WHEN** the installer returns
- **THEN** no `ist` event is emitted, so no feature outcomes are reported, neither in the User-Agent nor in the body

#### Scenario: Language-specific OTel install

- **GIVEN** the user runs `dtwiz install otel-python`
- **WHEN** the install completes successfully
- **THEN** the `ist` event reports every feature as not tried, because no collector config is written and no host monitoring is enabled

#### Scenario: User-Agent stays within the capture limit

- **GIVEN** an `ist` event carrying the longest method identifier, the feature outcomes and a work time at the upper bound
- **WHEN** the User-Agent is assembled for a release version
- **THEN** the User-Agent fits within the capture limit
