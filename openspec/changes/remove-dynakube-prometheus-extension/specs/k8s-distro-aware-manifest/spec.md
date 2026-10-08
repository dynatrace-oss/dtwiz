# K8s Distro-Aware Manifest

## MODIFIED Requirements

### Requirement: Templates block belongs only in DynaKube #2 (agents), not DynaKube #1 (monitoring)

DynaKube #1 (monitoring) SHALL contain only `activeGate` with the `kubernetes-monitoring` capability and the optional KSPM block. All extension and image templates (EEC, OTel collector, log module), `telemetryIngest`, and `logMonitoring: {}` SHALL belong exclusively in DynaKube #2 (agents). When KSPM is enabled, only `kspmNodeConfigurationCollector` SHALL appear under `templates` in DynaKube #1.

#### Scenario: DynaKube #1 templates limited to kspmNodeConfigurationCollector

- **GIVEN** `InstallKubernetes()` is called with a KSPM-enabled distro
- **WHEN** the manifest is rendered
- **THEN** DynaKube #1 `templates` contains only `kspmNodeConfigurationCollector`; no EEC, OTel, or logModule entries

#### Scenario: DynaKube #1 has no templates block when KSPM disabled

- **GIVEN** `InstallKubernetes()` is called with a non-KSPM distro
- **WHEN** the manifest is rendered
- **THEN** DynaKube #1 has no `templates` block at all

#### Scenario: DynaKube #2 carries all extension templates

- **GIVEN** `InstallKubernetes()` is called for any distro
- **WHEN** the manifest is rendered
- **THEN** DynaKube #2 `templates` contains EEC, OTel collector, and log module image refs; `telemetryIngest` and `logMonitoring: {}` are also present in DynaKube #2
