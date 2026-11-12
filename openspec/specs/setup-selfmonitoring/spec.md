# Spec: Setup Self-Monitoring

## Purpose

Instrument the `setup` wizard with self-monitoring telemetry events so that each wizard step (invocation, analysis, recommendation, install) is recorded. This enables session correlation, step-level duration measurement, and abandonment analysis across wizard runs.

## Requirements

### Requirement: Setup command fires an invocation event on entry

When the `setup` command starts and the self-monitoring feature flag is enabled, the system SHALL fire an `inv` step event immediately so that the timestamp of the run's start is recorded.

#### Scenario: Invocation event fires on setup entry

- **GIVEN** the self-monitoring feature flag is enabled
- **WHEN** the user runs `dtwiz setup`
- **THEN** a self-monitoring event with step `inv` and command `set` is fired asynchronously before any wizard logic executes

#### Scenario: Invocation event is suppressed without feature flag

- **GIVEN** the self-monitoring feature flag is disabled
- **WHEN** the user runs `dtwiz setup`
- **THEN** no self-monitoring events are fired at any step

### Requirement: Setup command fires an analysis event after system analysis

After system analysis completes or fails, the system SHALL fire an `ana` step event so that analysis duration and failure rate can be measured.

#### Scenario: Analysis event fires on success

- **GIVEN** the self-monitoring feature flag is enabled
- **WHEN** system analysis completes successfully
- **THEN** a self-monitoring event with step `ana`, command `set`, and no error field is fired

#### Scenario: Analysis event fires on failure

- **GIVEN** the self-monitoring feature flag is enabled
- **WHEN** system analysis returns an error
- **THEN** a self-monitoring event with step `ana`, command `set`, and error field `"err"` is fired

### Requirement: Setup command fires a recommendation event after user selection

After the user makes a valid selection from the recommendations menu, the system SHALL fire a `rec` step event carrying the selected method as the sub-identifier. If the user cancels or provides no selection, no `rec` event is fired — the absence of this event signals abandonment.

#### Scenario: Recommendation event fires with method sub-identifier on numeric selection

- **GIVEN** the self-monitoring feature flag is enabled and recommendations are displayed
- **WHEN** the user selects a valid numeric option (e.g., Kubernetes)
- **THEN** a self-monitoring event with step `rec`, command `set`, and sub-identifier matching the selected method shortcode (e.g., `"k8s"`) is fired

#### Scenario: Recommendation event fires with "uni" sub-identifier when user requests uninstall help

- **GIVEN** the self-monitoring feature flag is enabled
- **WHEN** the user enters `u` to view uninstall options
- **THEN** a self-monitoring event with step `rec`, command `set`, and sub-identifier `"uni"` is fired

#### Scenario: Recommendation event fires with "demo" sub-identifier for demo selection

- **GIVEN** the self-monitoring feature flag is enabled
- **WHEN** the user enters `d` to install the demo app
- **THEN** a self-monitoring event with step `rec`, command `set`, and sub-identifier `"demo"` is fired

#### Scenario: No recommendation event fires on cancellation

- **GIVEN** the self-monitoring feature flag is enabled
- **WHEN** the user enters `0` or an empty input to cancel
- **THEN** no `rec` step event is fired

### Requirement: Setup command fires an install report event when install work started

After the selected installer returns, the system SHALL fire an `ist` step event carrying the selected method if and only if install work started. The `ist` event SHALL NOT carry an error type or error attributes; the install error is reported by setup's terminal event. When the user requests uninstall help, no install event is fired since no installation occurs.

The `ist` event SHALL carry the install work time and the feature outcomes with the same meaning, form and presence rules as the `ist` event of `dtwiz install <method>`, including which features each install method tries (see the `install-selfmonitoring` spec).

Setup's update choices are not install methods, so their features are defined here. For all of them the work time ends before the post-install watch starts. The OTel Collector update SHALL try the collector config, and SHALL try Host Monitoring only when the running collector is a Dynatrace distribution. The Azure and GCP updates SHALL report all features as not tried.

#### Scenario: Install event fires on successful install

- **GIVEN** a method has been selected and the user confirmed
- **WHEN** the installer completes without error
- **THEN** an `ist` event identifying the setup command and the selected method is fired with no error

#### Scenario: Install event fires without error on installer failure

- **GIVEN** a method has been selected and the user confirmed
- **WHEN** the installer returns a non-cancellation error after install work started
- **THEN** an `ist` event identifying the setup command and the selected method is fired with no error type, carrying the work time and the feature outcomes reached so far

#### Scenario: No install event fires when the installer fails before work started

- **GIVEN** a method has been selected
- **WHEN** the installer returns an error before the install confirmation
- **THEN** no `ist` event is fired

#### Scenario: No install event fires when user cancels at installer confirmation

- **GIVEN** a method has been selected
- **WHEN** the user declines the install confirmation prompt
- **THEN** no `ist` event is fired

#### Scenario: No install event fires when the OTel Collector config is already up to date

- **GIVEN** the user selected the OTel Collector update
- **WHEN** the installer finds nothing to change and reports the configuration as up to date
- **THEN** no `ist` event is fired

#### Scenario: No install event fires on dry-run

- **GIVEN** `dtwiz setup --dry-run` and a selected method
- **WHEN** the installer's dry-run exits without executing anything
- **THEN** no `ist` event is fired

#### Scenario: Install event fires for demo path

- **GIVEN** the user selected the demo option and confirmed
- **WHEN** the demo installer completes
- **THEN** an `ist` event identifying the setup command and the demo is fired

#### Scenario: Install event carries work time and feature outcomes

- **GIVEN** the user selected the OTel Collector install
- **WHEN** the user confirms, the collector config is written, the Host Monitoring extension is activated, and the install work takes 25 seconds
- **THEN** the `ist` event carries a work time of about 25 seconds, reports OTel config and Host Monitoring as succeeded, and carries the same properties as a direct `dtwiz install otel-collector`

#### Scenario: Install event for a method without an install confirmation

- **GIVEN** the user selected OneAgent on a host without an existing OneAgent
- **WHEN** the OneAgent installer completes successfully
- **THEN** the `ist` event carries a work time measured from when the installer was called and reports Host Monitoring as succeeded

#### Scenario: Install event for a cloud update excludes the watch session

- **GIVEN** the user selected the Azure update
- **WHEN** the monitoring configuration is reconciled and the post-install watch then runs before the installer returns
- **THEN** the `ist` event's work time ends when the reconciliation finished, and every feature is reported as not tried

#### Scenario: Install event for an OTel Collector update of a non-Dynatrace collector

- **GIVEN** the user selected the OTel Collector update and the running collector is not a Dynatrace distribution
- **WHEN** the Dynatrace exporter is merged into its config and the config is written
- **THEN** the `ist` event reports OTel config as succeeded and Host Monitoring as not tried, because dtwiz does not try to enable host monitoring for that collector

### Requirement: Setup ends an install with exactly one terminal event

When the user selects a method, an update choice, or the demo, and the selected installer is reached, the system SHALL emit exactly one terminal event for the setup command when the installer returns, after the `ist` event if one is emitted:

- `com` (completed) with no error when the installer succeeds, including dry-run;
- `can` (cancelled) when the user declines the install confirmation, or when the OTel Collector update finds the configuration already up to date;
- `fai` (failed) with the classified error type and its error attributes when the installer returns any other error.

The terminal event is the only event of a setup run that reports the install error. When the post-install watch runs, the terminal event SHALL be sent before the watch session sends its first event, unless the installer runs the watch itself.

#### Scenario: Successful setup install ends with com

- **GIVEN** the user selected the OTel Collector install and confirmed
- **WHEN** the installer completes successfully
- **THEN** an `ist` event and a `com` event with no error are emitted for the setup command

#### Scenario: Failed setup install reports the error on fai

- **GIVEN** the user selected a method and confirmed
- **WHEN** the installer fails after install work started
- **THEN** an `ist` event without an error is emitted, and a `fai` event carrying the classified error type and its error attributes

#### Scenario: Declined confirmation in setup ends with can

- **GIVEN** the user selected a method
- **WHEN** the user declines the install confirmation
- **THEN** a `can` event with the user-cancelled error type is emitted, and no `ist` event

#### Scenario: Up-to-date OTel Collector config ends with can

- **GIVEN** the user selected the OTel Collector update
- **WHEN** the installer finds nothing to change and reports the configuration as up to date
- **THEN** a `can` event is emitted, and no `ist` event

#### Scenario: Setup dry-run ends with com

- **GIVEN** `dtwiz setup --dry-run` and a selected method
- **WHEN** the installer's dry-run exits without executing anything
- **THEN** a `com` event with no error is emitted, and no `ist` event

#### Scenario: Demo path ends with a terminal event

- **GIVEN** the user selected the demo option
- **WHEN** the demo installer completes successfully
- **THEN** an `ist` event identifying the setup command and the demo is emitted, and a `com` event for the setup command

### Requirement: Update-variant methods are normalized to distinct shortcodes

The system SHALL map the three update-variant method identifiers to distinct shortcodes so that telemetry sub-identifiers remain within the established character budget and distinguish update flows from install flows.

#### Scenario: OTel update method is normalized

- **GIVEN** the selected ingestion method is an OTel Collector update
- **WHEN** a self-monitoring event is built for that selection
- **THEN** the sub-identifier is `"otlu"`

#### Scenario: Azure update method is normalized

- **GIVEN** the selected ingestion method is an Azure update
- **WHEN** a self-monitoring event is built for that selection
- **THEN** the sub-identifier is `"azu"`

#### Scenario: GCP update method is normalized

- **GIVEN** the selected ingestion method is a GCP update
- **WHEN** a self-monitoring event is built for that selection
- **THEN** the sub-identifier is `"gcpu"`
