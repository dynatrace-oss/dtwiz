# OTel Collector Update

## MODIFIED Requirements

### Requirement: Dynatrace OTel Collector config is regenerated from the install template

When the selected or matched running collector is identified as a Dynatrace OTel Collector, the update SHALL regenerate the full collector config from the install template with the current tenant credentials and receiver ports, rather than patching the exporter in as an additional entry. A Dynatrace OTel Collector is identified by a binary path that contains `dynatrace-otel-collector`.

Existing OTLP receiver port assignments SHALL be preserved from the current config so that connected app services keep their OTLP endpoint after the restart. The ports read are:

- `receivers.otlp.protocols.grpc.endpoint` (default 4317)
- `receivers.otlp.protocols.http.endpoint` (default 4318)
- `service.telemetry.metrics.readers[0].pull.exporter.prometheus.port` (default 8888)
- `extensions.health_check.endpoint` (default 13133)

The regenerated config SHALL always include host-monitoring collector settings.

If the freshly rendered config is byte-identical to the existing file and no platform token is available, the update prints `Collector configuration is up to date.` and returns `ErrUpToDate`. The `cmd/update.go` handler treats `ErrUpToDate` the same as `ErrInstallCancelled`: clean exit and no error printed.

If the freshly rendered config is byte-identical to the existing file and a platform token is available, the update SHALL still preview and apply tenant-side prerequisites for host monitoring, including extension activation and OpenPipeline route reconciliation. In that case, the update prints `Collector configuration is up to date.` after applying tenant-side prerequisites, returns success, and does not call `WatchIngest()` because no collector restart occurred.

After a successful collector restart and verification, `WatchIngest()` is called to poll until new telemetry data appears in Dynatrace.

#### Scenario: Dynatrace collector config is current and no platform token is available

- **GIVEN** a running `dynatrace-otel-collector` process is selected
- **AND** the existing config already reflects the current tenant and token
- **AND** no platform token is available
- **WHEN** `dtwiz update otel` is run
- **THEN** `Collector configuration is up to date.` is printed
- **THEN** the command exits cleanly (`ErrUpToDate`), no error is printed, and `WatchIngest` is not called

#### Scenario: Dynatrace collector config is current and tenant prerequisites still need reconciliation

- **GIVEN** a running `dynatrace-otel-collector` process is selected
- **AND** the existing config already reflects the current tenant and token
- **AND** a platform token is available
- **WHEN** `dtwiz update otel` is run and the user confirms the tenant-side prerequisite preview
- **THEN** the host monitoring extension activation step is attempted
- **THEN** the OpenPipeline route reconciliation step is attempted
- **THEN** `Collector configuration is up to date.` is printed
- **THEN** the command returns success rather than `ErrUpToDate`
- **THEN** `WatchIngest` is not called because no collector restart occurred

#### Scenario: Dynatrace collector with outdated credentials selected

- **GIVEN** a running `dynatrace-otel-collector` process is selected
- **AND** the existing config has a stale tenant URL or token
- **WHEN** the user confirms the preview
- **THEN** the config is regenerated from the template with the current tenant credentials
- **THEN** the existing OTLP receiver ports are preserved in the new config
- **THEN** the regenerated config includes host-monitoring collector settings
- **THEN** the collector is restarted and verified
- **THEN** `WatchIngest()` is called to confirm data arrives in Dynatrace
