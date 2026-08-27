# OTel Host Monitoring Grail Routes

## MODIFIED Requirements

### Requirement: Dynamic routes for Smartscape on Grail are set up after host-monitoring install

After the managed OTel Collector host-monitoring install completes successfully, `install otel` SHALL ensure a dynamic route exists for each of metrics, logs, and spans that routes OpenTelemetry host telemetry into the OTel host monitoring extension's pipeline, using the documented matching conditions.

#### Scenario: Routes set up during install

- **GIVEN** the OTel host monitoring extension's pipeline exists for metrics, logs, and spans
- **WHEN** `install otel` runs without `--dry-run`
- **THEN** dtwiz sets up the three dynamic routes as described in this capability

## REMOVED Requirements

### Requirement: Route setup is gated behind the experimental flag

Route setup is released and no longer depends on `--experimental` or `DTWIZ_EXPERIMENTAL=true`.
