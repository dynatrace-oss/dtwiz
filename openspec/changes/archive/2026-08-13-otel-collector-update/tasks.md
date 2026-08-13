# Tasks: Update Dynatrace OTel Collector

## 1. YAML config patching (upstream OTel path)

- [x] 1.1 Implement `mergeExporterIntoYAML()` — parse config as `yaml.Node` tree, inject `otlp_http/dynatrace` exporter, append exporter to every pipeline's exporters list, re-serialise preserving comments, key order, and flow/block style
- [x] 1.2 Implement `buildDTExporterNode()` — constructs the YAML node subtree for the Dynatrace OTLP exporter definition
- [x] 1.3 Implement `appendExporterToPipeline()` — appends `otlp_http/dynatrace` to a pipeline's exporters list without duplicating, preserving existing flow/block style
- [x] 1.4 Implement `nodeMappingGet()` and `nodeMappingSet()` helpers for `yaml.Node` mapping traversal and mutation
- [x] 1.5 Implement `backupFile()` — writes a timestamped `.bak.<unix>` copy before applying changes
- [x] 1.6 Implement `PatchConfigFile()` — read, backup, patch, write; returns `UpdateResult`

## 2. Config diff preview

- [x] 2.1 Implement `diffLines()` — LCS-based line diff producing keep/add/delete edits
- [x] 2.2 Implement `showConfigDiff()` — coloured diff output with up to 2 YAML ancestor lines as context; `...` between non-adjacent hunks; redacts `Authorization:` values

## 3. Dynatrace Collector update path

- [x] 3.1 Extract `renderOtelTemplate()` from the install path into a shared helper usable by both install and update
- [x] 3.2 Implement `portsFromConfig()` — reads grpc, http, metrics, and health_check ports from an existing collector config with defaults fallback
- [x] 3.3 Implement `updateDynatraceCollector()` — regenerate config from template with current credentials and preserved ports; byte-compare against existing config; return `ErrUpToDate` if unchanged; restart collector; call `WatchIngest()`
- [x] 3.4 Define `ErrUpToDate` sentinel and wire it into `cmd/update.go` and `cmd/setup.go` as a clean exit (alongside `ErrInstallCancelled`)

## 4. Port-aware verification

- [x] 4.1 Implement `otlpHTTPPortFromConfig()` — reads OTLP HTTP receiver port from config, fallback 4318
- [x] 4.2 Update `verifyOtelInstall()` to accept `httpPort int` parameter
- [x] 4.3 Update `waitForOtelCollectorReady()` to accept `httpPort int` and probe `127.0.0.1:<httpPort>` instead of hardcoded 4318
- [x] 4.4 Update `sendOtelVerificationLog()` to post to `http://127.0.0.1:<httpPort>/v1/logs`
- [x] 4.5 Pass `otlpHTTPPortFromConfig(configPath)` at all `verifyOtelInstall` call sites in the update path

## 5. Interactive update entry points

- [x] 5.1 Implement `updateOtelConfig()` — shared update implementation: reads config, builds diff preview, shows restart plan, detects connected services, confirms, writes backup, applies patch, restarts collectors, restarts services
- [x] 5.2 Implement `UpdateOtelConfig()` (public) — validates non-empty configPath, resolves to absolute path, matches running collectors, delegates to `updateDynatraceCollector` or `updateOtelConfig`
- [x] 5.3 Implement `UpdateOtelConfigInteractive()` — presents running collector picker, handles container config extraction, delegates to the correct update path

## 6. Connected service detection

- [x] 6.1 Define `connectedService` struct — pid, name, command, workDir, collectorPort, listenPorts, exportsTo, env, collectorEndpoint
- [x] 6.2 Implement `receiverPortsFromConfig()` — parse collector YAML config and return configured OTLP gRPC and HTTP receiver ports; fallback to [4317, 4318]
- [x] 6.3 Implement `collectorTenantsFromConfig()` — extract tenant IDs from all configured exporter endpoints
- [x] 6.4 Implement `detectConnectedServices()` — fan-out to `detectServicesOnPorts` and `detectInstrumentedServices`, deduplicate by PID, exclude collector PIDs and dtwiz's own PID
- [x] 6.5 Implement `detectServicesOnPorts()` (Unix) — `lsof -i TCP:<ports> -nP -Fn`; parse output to PID→port, enrich with process name and listen ports; no TCP state filter (covers short-lived HTTP OTLP connections)
- [x] 6.6 Implement `detectInstrumentedServices()` (Unix) — `ps -eww -o pid,comm,args,e`; filter by `OTEL_EXPORTER_OTLP_*` env vars matching any configured tenant or collector port; exclude `DT_ENVIRONMENT` from matching
- [x] 6.7 Implement `detectInstrumentedServices()` (Windows stub) — no-op; elevated `ReadProcessMemory` not available
- [x] 6.8 Implement env-var helpers: `otlpEndpointFromEnv()`, `endpointMatchesCollector()`, `hostPort()`, `isLoopback()`
- [x] 6.9 Implement `otlpSignalEndpointKeys` list and `reconcileExportEnv()` — update `OTEL_EXPORTER_OTLP_ENDPOINT` and Authorization header to current `DT_ENVIRONMENT` + `DT_PLATFORM_TOKEN`; drop per-signal overrides; no-op when endpoint is loopback
- [x] 6.10 Implement `retargetEnvToCollector()` — set `OTEL_EXPORTER_OTLP_ENDPOINT` to local collector endpoint and drop per-signal overrides; no-op when endpoint is already loopback
- [x] 6.11 Implement `envGet()`, `envSet()`, `envRemove()` — `KEY=VAL` slice helpers
- [x] 6.12 Implement `rebuildAuthHeader()` and `headerHasToken()` — update Authorization token in an OTLP headers string

## 7. Connected service preview and restart

- [x] 7.1 Implement `printConnectedServices()` — print PID, binary name, listen ports, and export endpoint for each detected service
- [x] 7.2 Implement `restartConnectedServices()` — SIGTERM→SIGKILL stop, then detached relaunch with original command/workdir/env (reconciled or retargeted); print result per service; warn on relaunch failure without aborting

## 8. Verification

- [x] 8.1 `make test` — all tests pass
- [x] 8.2 `make lint` — no new lint issues
