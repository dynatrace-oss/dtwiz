# Design: Update Dynatrace OTel Collector

## Context

The `dtwiz update otel` command was a stub. Its only real action was `updateOtelCollectorIfPresent` — a silent background helper that searched a fixed path for a dtwiz-managed config and patched it in-place with no user interaction, no preview, and no restart.

This change implements the full interactive update flow and introduces a split between two update strategies based on the collector binary.

## Goals / Non-Goals

**Goals:**

- Interactive update flow for `dtwiz update otel` and the `dtwiz setup` path
- Dynatrace OTel Collector: regenerate config from the install template so the output matches the install contract
- Upstream OTel Collector: inject the Dynatrace exporter using YAML node-level edit that preserves comments, key order, and flow/block style
- Detect and restart connected app services as part of the update
- Port-aware verify/wait: use the OTLP HTTP port from the config file, not a hardcoded 4318

**Non-Goals:**

- Windows connected-service detection via env-var tenant match (requires elevated `ReadProcessMemory` — implemented as a no-op on Windows)
- Container-based connected-service detection (only native processes are detected)
- Simultaneous update of multiple running collectors

## Decisions

### 1. Two update paths: Dynatrace Collector vs upstream OTel Collector

Dynatrace-distributed collectors are identified by binary name (checking for the `dynatrace-otel-collector` substring). When detected, the update regenerates the full config from the install template. This ensures the output always matches the canonical Dynatrace Collector configuration contract rather than accumulating stale or conflicting YAML from successive patch operations.

Upstream collectors get the exporter-merge path: only the `otlp_http/dynatrace` exporter is injected; the rest of the config is preserved as-is.

### 2. YAML node-level editing preserves comments, key order, and flow/block style

A plain unmarshal → marshal roundtrip discards YAML comments and may reorder keys or normalize sequence styles (e.g. collapsing flow sequences to block style). The update uses a `yaml.Node` tree edit — walking the existing tree and setting nodes in-place — paired with an LCS-based line diff for the preview. Neither step requires a full roundtrip through Go structs.

### 3. Connected service detection via two independent signals

Active TCP connections to the collector's OTLP ports catch services that actively send OTLP data to the collector. OTLP env-var tenant matching catches services that export directly to Dynatrace without going through a local collector — they would be invisible to the TCP signal. Both signals are applied and deduplicated by PID.

`DT_ENVIRONMENT` is intentionally excluded from the env-var search: dtwiz and all child processes inherit it, which would produce false matches.

### 4. Up-to-date as a clean-exit sentinel

The Dynatrace Collector up-to-date check uses byte comparison of the freshly rendered template against the existing file. Using a typed sentinel error rather than `nil` lets callers distinguish "nothing to do" from "update applied" without an extra boolean return value. The update and setup commands treat it the same as user cancellation — clean exit, no error printed.

### 5. Port-aware verify and wait

When multiple collectors run concurrently they use different OTLP HTTP ports. Hardcoding 4318 would cause the verify step to probe the wrong collector after a restart. The OTLP HTTP port is read from the patched or regenerated config, falling back to 4318 only when the field cannot be parsed.

### 6. Retargeting vs reconciling connected services

Two different post-restart corrections apply depending on the update path:

- **Upstream OTel update**: connected services keep their existing OTLP endpoint (the local collector is already handling routing). Only stale tenant references in `OTEL_EXPORTER_OTLP_ENDPOINT` are corrected to the current `DT_ENVIRONMENT` and `DT_PLATFORM_TOKEN`.
- **Dynatrace Collector update**: the config may now export to additional tenants. Services that export directly to any Dynatrace tenant (non-loopback) are retargeted to the local collector HTTP endpoint so their data reaches all configured export destinations.

## Risks / Trade-offs

- **[lsof availability]** `lsof` is not installed on all Linux distributions. When absent, `detectServicesOnPorts` returns no results and the update proceeds without service restart. → Acceptable: the collector itself is still updated; the operator can restart services manually.
- **[Short-lived OTLP connections]** HTTP OTLP exporters close the connection after each batch. Filtering on ESTABLISHED state would miss them between export intervals. No TCP state filter is applied, which means `lsof` may report connections in CLOSE_WAIT or TIME_WAIT. These are harmless — the service is still a client of the collector and will be included.
- **[Windows env-var detection]** `ps -eww` is Unix-only. Reading process environment on Windows requires `ReadProcessMemory` and elevated privileges. `detectInstrumentedServices` is a no-op on Windows; tenant-matched services will not be detected or restarted automatically.
