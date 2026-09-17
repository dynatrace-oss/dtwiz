# Spec: Setup Self-Monitoring

## ADDED Requirements

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

### Requirement: Setup command fires an install event after the installer returns

After the selected installer finishes (success or failure), the system SHALL fire an `ist` step event carrying the selected method as the sub-identifier and an error field if the installer failed. When the user requests uninstall help, no install event is fired since no installation occurs.

#### Scenario: Install event fires on successful install

- **GIVEN** the self-monitoring feature flag is enabled and a method has been selected
- **WHEN** the installer completes without error
- **THEN** a self-monitoring event with step `ist`, command `set`, the method sub-identifier, and no error field is fired

#### Scenario: Install event fires with error field on installer failure

- **GIVEN** the self-monitoring feature flag is enabled and a method has been selected
- **WHEN** the installer returns a non-cancellation error
- **THEN** a self-monitoring event with step `ist`, command `set`, the method sub-identifier, and error field `"err"` is fired

#### Scenario: No install event fires when user cancels at installer confirmation

- **GIVEN** the self-monitoring feature flag is enabled and a method has been selected
- **WHEN** the user declines the install confirmation prompt
- **THEN** no `ist` event is fired (cancellation is signaled by the missing event)

#### Scenario: Install event fires for demo path

- **GIVEN** the self-monitoring feature flag is enabled and the user selected the demo option
- **WHEN** the demo installer completes
- **THEN** a self-monitoring event with step `ist`, command `set`, and sub-identifier `"demo"` is fired

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
