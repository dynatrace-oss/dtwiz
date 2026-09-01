## Why

After a fresh OneAgent install, `dtwiz watch` can show no Smartscape host entities even when host monitoring was installed successfully. This makes the install look ineffective at exactly the moment the user expects confirmation that data is arriving.

## What Changes

- Add a `Hosts` section to the ingest watch output.
- List regular Dynatrace hosts and OpenTelemetry hosts together under `Hosts`; no separate OTel host section is introduced.
- Render individual host names as deep links to their Dynatrace host detail pages when host data is available.
- Reorder watch sections so `Kubernetes` appears before `Cloud`, matching the recommender order.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `ingest-watch`: Add host entities to the watch summary and update the display order.

## Impact

- Affects `pkg/installer/ingest_watch.go` polling, parsing, and rendering.
- Adds focused unit coverage in `pkg/installer/ingest_watch_test.go`.
- No new dependencies, flags, or breaking CLI changes.