# Tasks: Update Dynatrace OTel Collector

## 1. YAML config patching (upstream OTel path)

- [x] Parse config as a YAML node tree, inject the Dynatrace OTLP exporter, append it to every pipeline's exporters list, and re-serialise preserving comments, key order, and flow/block style
- [x] Back up the config file with a timestamped copy before applying changes
- [x] Read, backup, patch, and write the config; return an update result

## 2. Config diff preview

- [x] LCS-based line diff producing keep/add/delete edits
- [x] Coloured diff output with YAML ancestor context lines; `...` between non-adjacent hunks; redacts `Authorization:` values

## 3. Dynatrace Collector update path

- [x] Extract template rendering into a shared helper usable by both install and update
- [x] Read grpc, http, metrics, and health_check ports from an existing collector config with defaults fallback
- [x] Regenerate config from template with current credentials and preserved ports; byte-compare against existing config; return an up-to-date sentinel if unchanged; restart collector; poll until data arrives in Dynatrace
- [x] Wire up-to-date sentinel into the update and setup commands as a clean exit

## 4. Port-aware verification

- [x] Read OTLP HTTP receiver port from config, fallback 4318
- [x] Update collector verification and wait steps to probe the port read from the config instead of hardcoding 4318

## 5. Interactive update entry points

- [x] Shared update implementation: reads config, builds diff preview, shows restart plan, detects connected services, confirms, writes backup, applies patch, restarts collector, restarts services
- [x] Public update entry point: validates non-empty config path, resolves to absolute path, matches running collectors, delegates to the correct update path
- [x] Interactive entry point: presents running collector picker, handles container config extraction, delegates to the correct update path

## 6. Connected service detection

- [x] `connectedService` struct — pid, name, command, workDir, collectorPort, listenPorts, exportsTo, env, collectorEndpoint
- [x] Read configured OTLP gRPC and HTTP receiver ports from collector YAML config; fallback to 4317/4318
- [x] Extract tenant IDs from all configured exporter endpoints
- [x] Fan-out to TCP and env-var detection, deduplicate by PID, exclude collector PIDs and dtwiz's own PID
- [x] Unix: detect services with active TCP connections to any of the collector's OTLP ports; enrich with process name and listen ports
- [x] Unix: detect OTel-instrumented processes via `OTEL_EXPORTER_OTLP_*` env vars matching any configured tenant or collector port; exclude `DT_ENVIRONMENT` from matching
- [x] Windows: no-op stub (elevated privileges required for env-var detection)
- [x] Env-var helpers: resolve OTLP endpoint from env, check endpoint against collector, parse host/port, check loopback
- [x] Reconcile export env: update `OTEL_EXPORTER_OTLP_ENDPOINT` and Authorization header to current `DT_ENVIRONMENT` + `DT_PLATFORM_TOKEN`; drop per-signal overrides; no-op when endpoint is loopback
- [x] Retarget env to collector: set `OTEL_EXPORTER_OTLP_ENDPOINT` to local collector endpoint; drop per-signal overrides; no-op when endpoint is already loopback

## 7. Connected service preview and restart

- [x] Print PID, binary name, listen ports, and export endpoint for each detected service
- [x] SIGTERM→SIGKILL stop, then detached relaunch with original command/workdir/env (reconciled or retargeted); print result per service; warn on relaunch failure without aborting

## 8. Verification

- [x] All tests pass
- [x] No new lint issues
