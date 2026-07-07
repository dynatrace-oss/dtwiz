# Cloud Install UX Improvement Tasks

## 1. Core Implementation

- [x] 1.1 Add `cloudInstall bool` param to `watchIngest` in `pkg/installer/ingest_watch.go`
- [x] 1.2 Render Clouds app footer when `cloudInstall` is true; keep QuickStart footer otherwise
- [x] 1.3 Add exported `WatchIngestCloud` function calling `watchIngest` with `cloudInstall: true`
- [x] 1.4 Update `WatchIngestAWS` to pass `cloudInstall: true`
- [x] 1.5 Update existing `WatchIngest` and `WatchIngestWithStatus` callers to pass `cloudInstall: false`

## 2. Call Site Updates

- [x] 2.1 Update `pkg/installer/gcp/install.go` — replace `installer.WatchIngest` with `installer.WatchIngestCloud`
- [x] 2.2 Update `pkg/installer/azure/install.go` — replace `installer.WatchIngest` with `installer.WatchIngestCloud`

## 3. Recommender UX (af173ca)

- [x] 3.1 Add `AzureConfigured`, `GCPConfigured` to `SystemInfo` in `pkg/analyzer/analyzer.go`
- [x] 3.2 Add `MethodAzureUpdate`, `MethodGCPUpdate` constants in `pkg/recommender/recommender.go`
- [x] 3.3 Emit update method + title when already configured in `GenerateRecommendations`
- [x] 3.4 Fix OTel titles: "patch" → "update", add "new" to collector title
- [x] 3.5 Move connection pre-check before recommendations in `cmd/setup.go`
- [x] 3.6 Dispatch on `MethodAzureUpdate`/`MethodGCPUpdate` in setup switch; remove local boolean state

## 4. Tests

- [x] 4.1 Verify build passes: `make build`
- [x] 4.2 Verify tests pass: `make test`

