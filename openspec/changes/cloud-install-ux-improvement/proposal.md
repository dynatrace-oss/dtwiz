# Proposal: Cloud Install UX Improvement

## Why

Two UX gaps for cloud installs: the watch screen footer links to QuickStart (which doesn't show cloud resources yet), and the setup menu doesn't distinguish install vs update for Azure/GCP. OTel recommendation wording also needed clarification.

## What Changes

- Watch screen footer shows "See your cloud resources in the Clouds app" with an "Open Clouds" link for AWS, GCP, and Azure installs.
- All other install methods keep the existing QuickStart footer.
- Setup menu shows "Azure cloud services (update)" / "GCP cloud services (update)" when already configured.
- Recommender emits distinct `MethodAzureUpdate` / `MethodGCPUpdate` methods so setup dispatches correctly without local boolean state.
- OTel wording: "patch existing" → "update existing"; "via OpenTelemetry" → "via new OpenTelemetry Collector".

## Capabilities

### New Capabilities

- `cloud-install-footer`: Footer variant for cloud installs linking to the Clouds app.
- `cloud-recommend-ux`: Recommender and setup menu distinguish install vs update for Azure/GCP; OTel wording fixes.

### Modified Capabilities

## Impact

- `pkg/installer/ingest_watch.go` — footer rendering logic
- `pkg/installer/gcp/install.go`, `pkg/installer/azure/install.go` — call site changes
- `pkg/recommender/recommender.go` — new methods, updated titles
- `pkg/analyzer/analyzer.go` — `AzureConfigured`, `GCPConfigured` fields on `SystemInfo`
- `cmd/setup.go` — connection pre-check moved earlier, dispatches on new method constants

