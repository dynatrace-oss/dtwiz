## MODIFIED Requirements

### Requirement: OTel Collector config update

After launching the instrumented Java process, the installer SHALL update the local OTel Collector configuration if one exists.

#### Scenario: OTel Collector config found

- **GIVEN** the dtwiz well-known collector config path (`~/opentelemetry/config.yaml`) exists on the machine
- **WHEN** the instrumented Java process has been started successfully
- **THEN** the installer SHALL patch the collector config silently using `PatchConfigFile` — no interactive prompt, no restart
- **AND** SHALL output a single summary line via `display.PrintStatusLine("collector", "config updated", display.ColorOK)` indicating the config was updated

#### Scenario: No OTel Collector config found

- **GIVEN** the dtwiz well-known collector config path does not exist
- **WHEN** the installer reaches the collector update step
- **THEN** the step SHALL be skipped silently with no output
- **AND** the Java agent SHALL export directly to Dynatrace via OTLP without a local collector
