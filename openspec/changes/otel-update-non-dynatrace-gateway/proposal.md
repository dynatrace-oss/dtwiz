# OTel Update: Non-Dynatrace Collector Gateway

## Why

`dtwiz update otel` (hidden behind `--experimental` / `DTWIZ_EXPERIMENTAL=true`) currently treats every detected OTel Collector identically, regardless of distro: it patches the config file in place and kills the process directly before relaunching it as `binaryPath --config configPath` ([update.go](../../../pkg/installer/otel/update.go)). That is safe for the Dynatrace distro dtwiz manages, but not for a foreign (non-Dynatrace) collector — it fights whatever process supervisor (systemd, docker-compose) already owns that process, assumes OTel components (needed for host monitoring) are present when they may not be, and would inline a Dynatrace token into a config file dtwiz does not own.

Today, a user running a non-Dynatrace collector (Datadog Agent OTel distro, Grafana Alloy, `otelcol-contrib`, etc.) has no supported way to also send that data to Dynatrace, or to add host monitoring, without hand-editing their config and understanding OTel Collector internals themselves.

## What Changes

- When `update otel` targets a **non-Dynatrace** collector, dtwiz first checks whether the collector's effective config resolves to a single, writable, local YAML file (not multiple `--config` merges, not an `env:`/`yaml:` inline provider, not a config baked into a container image with no durable write-back path). If not, dtwiz makes **no changes at all** and shows a docs link explaining how to add the Dynatrace exporter and host monitoring manually.
- If the config source check passes, dtwiz:
  1. Creates a timestamped backup of the current config and prints its path.
  2. Detects the process supervisor (systemd, docker/docker-compose, or bare/manual) and, for the bare/manual case, captures the full launch context (argv, env, cwd) needed for a faithful relaunch.
  3. Deploys a new, dedicated **Dynatrace Gateway Collector** on `localhost:4319`, configured with host monitoring (reusing the host-monitoring-capable config generation shipped in `add-otel-host-monitoring`).
  4. Patches the existing collector's config with exactly **one additive change**: a new `otlp` exporter (no auth, no secrets) pointed at `localhost:4319`, appended to each existing pipeline's exporter list. No receivers, processors, or pipelines are added to the foreign config, and nothing existing is modified or removed.
- dtwiz then determines whether it can safely restart the foreign collector itself:
  - **Systemd unit or docker/docker-compose container:** dtwiz restarts it via the supervisor's own mechanism (`systemctl restart <unit>` / `docker restart <container>`), never a raw `kill`.
  - **Bare/manual process with a fully captured launch context:** dtwiz restarts it using that captured context.
  - **Otherwise** (ambiguous supervisor, or Kubernetes pod/ConfigMap-sourced — out of scope this iteration): dtwiz does not attempt a restart.
- **If dtwiz restarts the collector and it comes back up:** run the existing `WatchIngest` flow so the user sees ingest confirmed live.
- **If the restart fails, or dtwiz could not attempt one:** print detailed manual restart instructions (new config path + backup path) and show links to the Dynatrace Services and Distributed Tracing apps instead of running `WatchIngest`, since dtwiz cannot know if or when the user completes a manual restart.

## Capabilities

### New Capabilities

- `otel-gateway-collector-update`: `update otel`, when a non-Dynatrace collector is selected, deploys a dedicated Dynatrace Gateway Collector (with host monitoring) and applies a minimal, additive forwarding patch to the existing collector, with supervisor-aware restart handling and manual fallbacks.

### Modified Capabilities

None. The existing Dynatrace-collector `update otel` path (`mergeDynatraceExporter` and related code) is unchanged.

## Impact

- **Code:**
  - `pkg/installer/otel/update.go` — new branch triggered when the collector selected via `selectCollector` is identified as non-Dynatrace.
  - New file(s) under `pkg/installer/otel/` for gateway-collector deployment, reusing `prepareCollectorPlan` / `generateOtelConfig` / `downloadOtelCollector` / `startOtelCollector` from `collector.go`, and the host-monitoring config generation from `add-otel-host-monitoring`.
  - `pkg/analyzer/detect_otel.go` — extend process inspection to capture full launch context (argv, env, cwd), not just binary path + a single `--config` value, needed for the bare/manual restart path.
  - New supervisor-detection code (systemd unit via `/proc/<pid>/cgroup` on Linux; reuse existing container detection for docker/docker-compose).
- **Unaffected:** the existing Dynatrace-collector `update otel` flow; `install otel` and its host-monitoring behavior (`add-otel-host-monitoring`); `uninstall otel`.
- **Out of scope (this iteration):**
  - Kubernetes pod/ConfigMap-sourced collectors — always routed to the "config source is not a single writable file" docs-link branch; no ConfigMap patch or `kubectl rollout restart` automation yet.
  - Reconciling the gateway collector's host-monitoring block against a newer reference config on repeat runs (the gateway is freshly deployed each time this flow runs for a given host).
  - Any dual-export isolation beyond the plain forwarding exporter (e.g., a `forward` connector to isolate processing) — the gateway collector receives the same data the foreign collector's existing pipelines already carry and does all Dynatrace-specific processing itself.
- **Dependencies:** none new. Reuses the Dynatrace OTel Collector distribution already downloaded for `install otel`.
- **Limitations and risk:** the foreign collector still needs one restart to pick up the new exporter, causing a brief telemetry gap — the same trade-off `update otel` already accepts today. A second collector process now runs on the host. See design for full risk discussion.
- **Rollout:** gated by `featureflags.Experimental` (`--experimental` / `DTWIZ_EXPERIMENTAL`), consistent with `update otel`'s existing gate.
