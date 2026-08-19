## MODIFIED Requirements

### Requirement: Dynatrace OTel Collector config is regenerated from the install template

When the selected or matched running collector is identified as a Dynatrace OTel Collector, the update SHALL regenerate the full collector config from the install template with the current tenant credentials and receiver ports, rather than patching the exporter in as an additional entry. A Dynatrace OTel Collector is identified by a binary path that contains `dynatrace-otel-collector`.

Existing OTLP receiver port assignments SHALL be preserved from the current config so that connected app services keep their OTLP endpoint after the restart. The ports read are:

- `receivers.otlp.protocols.grpc.endpoint` (default 4317)
- `receivers.otlp.protocols.http.endpoint` (default 4318)
- `service.telemetry.metrics.readers[0].pull.exporter.prometheus.port` (default 8888)
- `extensions.health_check.endpoint` (default 13133)

The regenerated config SHALL include host-monitoring collector settings by default.

If the freshly rendered config is byte-identical to the existing file, the update prints `Collector configuration is up to date.` and returns `ErrUpToDate`. The `cmd/update.go` handler treats `ErrUpToDate` the same as `ErrInstallCancelled`: clean exit, no error printed, and `WatchIngest` is not called.

After a successful restart of a Dynatrace collector, `WatchIngest()` is called to poll until new telemetry data appears in Dynatrace.

#### Scenario: Dynatrace collector with current credentials selected

- **GIVEN** a running `dynatrace-otel-collector` process is selected
- **AND** the existing config already reflects the current tenant and token
- **WHEN** `dtwiz update otel` is run
- **THEN** `Collector configuration is up to date.` is printed
- **THEN** the command exits cleanly (`ErrUpToDate`), no error is printed, and `WatchIngest` is not called

#### Scenario: Dynatrace collector with outdated credentials selected

- **GIVEN** a running `dynatrace-otel-collector` process is selected
- **AND** the existing config has a stale tenant URL or token
- **WHEN** the user confirms the preview
- **THEN** the config is regenerated from the template with the current tenant credentials
- **THEN** the existing OTLP receiver ports are preserved in the new config
- **THEN** the regenerated config includes host-monitoring collector settings
- **THEN** the collector is restarted and verified
- **THEN** `WatchIngest()` is called to confirm data arrives in Dynatrace
