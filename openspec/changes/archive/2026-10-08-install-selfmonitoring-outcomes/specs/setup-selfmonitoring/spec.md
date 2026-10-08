# Spec: Setup Selfmonitoring

## MODIFIED Requirements

### Requirement: Setup command fires an install event after the installer returns

After the selected installer finishes (success or failure), the system SHALL fire an `ist` step event carrying the selected method and, if the installer failed, a classified error type. When the user requests uninstall help, no install event is fired since no installation occurs.

The `ist` event SHALL carry the install work time and the feature outcomes with the same meaning, form and presence rules as the `ist` event of `dtwiz install <method>`, including which features each install method tries (see the `install-selfmonitoring` spec).

Setup's update choices are not install methods, so their features are defined here. For all of them the work time ends before the post-install watch starts. The OTel Collector update SHALL try the collector config, and SHALL try Host Monitoring only when the running collector is a Dynatrace distribution. The Azure and GCP updates SHALL report all features as not tried.

#### Scenario: Install event fires on successful install

- **GIVEN** a method has been selected
- **WHEN** the installer completes without error
- **THEN** an `ist` event identifying the setup command and the selected method is fired with no error

#### Scenario: Install event fires with error field on installer failure

- **GIVEN** a method has been selected
- **WHEN** the installer returns a non-cancellation error
- **THEN** an `ist` event identifying the setup command and the selected method is fired with the classified error type

#### Scenario: No install event fires when user cancels at installer confirmation

- **GIVEN** a method has been selected
- **WHEN** the user declines the install confirmation prompt
- **THEN** no `ist` event is fired (cancellation is signaled by the missing event)

#### Scenario: No install event fires when the OTel Collector config is already up to date

- **GIVEN** the user selected the OTel Collector update
- **WHEN** the installer finds nothing to change and reports the configuration as up to date
- **THEN** no `ist` event is fired

#### Scenario: Install event fires for demo path

- **GIVEN** the user selected the demo option
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
