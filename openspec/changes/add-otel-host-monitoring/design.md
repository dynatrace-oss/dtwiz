# OTel Host Monitoring Design

## Context

`install otel` deploys a single dtwiz-managed Dynatrace OTel Collector at `~/opentelemetry/` with a config rendered from [otel.tmpl](../../../pkg/installer/otel/otel.tmpl). That config is an application gateway only: an `otlp` receiver, a `cumulativetodelta` processor, and an `otlp_http` exporter to `/api/v2/otlp`. The collector is started as a detached background process ([startOtelCollector](../../../pkg/installer/otel/collector.go)), then a round-trip verification log is sent through it and confirmed via DQL.

The Dynatrace [host-metrics reference config](https://github.com/Dynatrace/dynatrace-otel-collector/blob/main/config_examples/host-metrics.yaml) collects host signals for the **OpenTelemetry Host Monitoring** extension. It adds:

- Receivers: `hostmetrics/10s`, `hostmetrics/5m`, `hostmetrics/1h`, `journald`, `otlp`
- Processors: `filter`, `resource_detection`, `transform`, `filter/delete-metrics`, `cumulativetodelta`
- Pipelines: `metrics` (hostmetrics through processors to otlp_http), `logs` (otlp and journald through resource_detection to otlp_http)
- Extension: `health_check`

Both configs export to the same `/api/v2/otlp` endpoint with the same auth header. They differ only in receivers, processors, and pipeline wiring. A single OTel Collector process runs many pipelines keyed by `type/name`, so both concerns fit in one collector.

Note that the reference `host-metrics.yaml` is host-only: it has no traces pipeline and no `otlp` receiver feeding metrics. It cannot simply replace `otel.tmpl`, or app traces and app metrics would be lost. The combined template produced by this change is therefore a superset of the current `otel.tmpl`: it keeps every existing app pipeline unchanged and adds the host pipelines alongside them. A run of `install otel` after this change gives users everything a run before this change gave them (app traces, app metrics, app logs over OTLP), plus host metrics and, on Linux, host logs, all through the one managed collector. No second collector is needed for full coverage; dtwiz always deploys and manages its own single collector regardless of what else is running on the host, exactly as it does today for app-only monitoring (see Decision 2 for what happens when another collector already exists).

## Goals / Non-Goals

**Goals:**

- `install otel` collects host metrics and (on Linux) host logs in addition to app signals, in the format the Host Monitoring extension expects.
- Reuse the existing single managed collector, lifecycle, preview, and verification. No new service-management machinery.
- Deterministic, testable config generation from a template dtwiz fully controls.
- Cross-platform: full signal set on Linux; metrics-only on macOS and Windows, where no equivalent host-log receiver is enabled in this change.

**Non-Goals:**

- Persistent OS-service installation (systemd, launchd, Windows service). Lifecycle stays as the current detached process.
- Any interaction with other OTel Collectors running on the host. `install otel` only writes its own managed config and does not read, detect, or react to foreign collector configurations.
- Configurable scrape intervals, per-scraper toggles, or a permanent `--host-monitoring` opt-out flag. Host monitoring is on by default once promoted out of the temporary `--experimental` rollout gate (see Decision 5), matching the zero-config principle.
- Changes to `update otel` or `uninstall otel`.

## Decisions

### Decision 1: One collector, combined config (regenerate, don't text-merge)

Extend `otel.tmpl` into a single combined config carrying both app and host pipelines. Because dtwiz owns the template, we generate the merged structure directly rather than parsing and injecting.

- The existing app metrics pipeline is renamed `metrics/apps` (otlp to cumulativetodelta to otlp_http).
- New `metrics/host` pipeline: `hostmetrics/*` to filter to resource_detection to transform to filter/delete-metrics to cumulativetodelta to otlp_http.
- New `logs/host` pipeline (Linux): otlp and journald to resource_detection to otlp_http. The existing `logs` pipeline (otlp-only) is retained for app logs, or merged with journald where appropriate.
- Single shared `otlp_http` exporter and `health_check` extension. The reference config binds `health_check` to a fixed port (13133); the combined template instead gives it a dynamically probed port (an additional `HealthCheckPort` field alongside the existing `GRPCPort`, `HTTPPort`, and `MetricsPort` fields), so it does not conflict when a second dtwiz collector runs on the same host (see Decision 2).

Why over alternatives: deep-merging YAML (as [mergeDynatraceExporter](../../../pkg/installer/otel/update.go) does for a single exporter) is reliable for one key but fragile across five processors with ordering constraints and multiple named pipelines. Since dtwiz always writes its own config on `install otel`, regeneration is both simpler and deterministic. Two separate collectors were rejected for the managed case because only one collector per host should scrape `hostmetrics` (otherwise host metrics double-count), and a single process shares the exporter, ports story, and lifecycle.

**Existing installs upgrade for free by rerunning `install otel`.** `prepareCollectorPlan` ([collector.go](../../../pkg/installer/otel/collector.go)) always renders `otel.tmpl` from scratch and `execute()` always stops the running collector, overwrites `config.yaml` unconditionally, and restarts it. There is no "config already exists, skip" branch anywhere in this path. This means a user who installed the old app-only collector with a previous `dtwiz install otel` gets host monitoring automatically the next time they run `dtwiz install otel` after this change ships: the old process is stopped, the config is fully replaced with the new combined template, and the collector restarts with host pipelines included, using the same install directory, exporter, and token. No separate migration command or one-time upgrade task is needed; the existing overwrite-and-restart behavior is the upgrade path.

### Decision 2: Always regenerate dtwiz's managed collector; other running collectors are ignored

`install otel` manages `~/opentelemetry/config.yaml` and always regenerates the config with host pipelines included. What else is running on the host is irrelevant: no foreign collector is read, checked, or written to. If another OTel Collector already collects host metrics for the same tenant, duplicate data is a possible side-effect, but that is a user concern outside the scope of this command. Users who want to consolidate collectors can use `update otel` separately.

Why over the alternative (read-only conflict detection): the simpler model is appropriate for a new install. Conflict detection adds code paths, prompts, and test surface for a scenario that only arises on hosts that already have an existing OTel setup, not the target audience for `install otel`.

### Decision 3: Platform-aware templating

`journald` is Linux-only, and no equivalent host-log receiver (such as Windows Event Log or macOS unified logging) is added in this change, so macOS and Windows simply get no host-log pipeline rather than a substitute. This was investigated, not assumed: the Dynatrace OTel Collector distribution dtwiz downloads (its [manifest.yaml](https://github.com/Dynatrace/dynatrace-otel-collector/blob/main/manifest.yaml)) does not compile in `windowseventlogreceiver`, and OpenTelemetry Collector Contrib's `macosunifiedloggingreceiver` (merged upstream November 2025) is not in any Dynatrace distribution release yet either. dtwiz downloads a prebuilt collector binary ([downloadOtelCollector](../../../pkg/installer/otel/collector.go)) rather than compiling it, so neither receiver is something dtwiz's own code could add even if it wanted to; it is gated on the upstream Dynatrace distribution shipping them. Reuse dtwiz's OS-suffix and build-tag pattern: the template (or a small config-data struct field like `IncludeJournald bool`) conditionally includes the `journald` receiver and `logs/host` pipeline. On Linux, include host logs. On macOS and Windows, emit host metrics only. `hostmetrics` scrapers are gated per-platform where a scraper is unsupported.

The `journald` receiver and its `logs/host` pipeline reference must be gated together, in both the `receivers:` block and the `service.pipelines` list, from the same struct field. If the two guards ever drift apart, the collector either fails to start (a pipeline references a receiver that was never defined) or carries a dead, unused receiver definition. Tests must render and parse the macOS/Windows config, not just check that the string `journald` is absent, so a mismatched guard is caught rather than passing on a surface-level string check.

Separately, some per-process `hostmetrics` metrics are unavailable on specific platforms independent of any conditional gating, for example per-process disk IO is not implemented on macOS at all. The reference config already sets `mute_process_all_errors: true` on the `process` scraper so these gaps degrade to missing data points rather than startup failures; this behavior is carried through unchanged.

Why over alternatives: a single template with conditionals keeps one source of truth. Fully separate per-OS templates would duplicate the large shared metrics section. dtwiz already renders OS differences via template data elsewhere.

### Decision 4: Preview and verification unchanged

The combined config is printed inline in the existing preview ([printConfigPreview](../../../pkg/installer/otel/collector.go)) with the token masked, under the single `Apply? [Y/n]` prompt. The existing OTLP round-trip verification still exercises the shared `otlp` receiver and `otlp_http` exporter, so it validates the collector regardless of the added host pipelines. Host metrics arrival is not separately verified in this change; the DQL log round-trip remains the signal that the collector is up and exporting.

### Decision 5: Ship behind `--experimental` until fully implemented and tested

This entire change ships behind the existing `--experimental` / `DTWIZ_EXPERIMENTAL` feature flag, the same convention already used for `install docker` ([install.go](../../../cmd/install.go)) and `update otel` ([update.go](../../../cmd/update.go)). Unlike those two, host monitoring is not a separate subcommand; it is new behavior inside the existing `install otel` command, so the gate applies to config generation rather than command visibility:

- `generateOtelConfig` / `prepareCollectorPlan` ([collector.go](../../../pkg/installer/otel/collector.go)) SHALL check `featureflags.IsEnabled(featureflags.Experimental)`. When disabled (the default), the collector config, preview, and install flow are byte-for-byte the same as before this change: app-only pipelines, no `hostmetrics`/`journald`/`health_check` receivers or extensions, no foreign-collector conflict detection, no privilege notice.
- When enabled, the full combined config from Decisions 1–4 is generated: host pipelines added, conflict detection runs, and the privilege notice is shown.
- The install output makes the gate discoverable: when host monitoring is inactive because the flag is off, a single line notes it can be enabled with `--experimental` or `DTWIZ_EXPERIMENTAL=true`, instead of silently doing nothing.

Why: this change is substantial, untested-in-the-wild surface: cross-platform `hostmetrics` behavior (task 0.2/0.3 still open) and a template large enough to risk regressions in the existing app pipelines it must preserve unchanged. Shipping it gated avoids exposing every `install otel` user to that risk the moment the code merges, while still allowing it to be exercised end-to-end (including the manual per-platform verification in task 6.3) before promotion. This does not conflict with the "zero-config, on by default" goal; that is the target end state once the gate is lifted, not the initial rollout condition. Promotion (removing the gate so host monitoring is unconditionally on) happens once tasks 0–6 are complete and verified, not as part of the initial merge.

## Risks / Trade-offs

- **Privilege gaps, per platform:** full host metrics and logs need elevated privileges, but the mechanism differs by OS, not just Linux. On Linux, some `hostmetrics` scrapers (process, disk, filesystem) and `journald` need root or the `systemd-journal` group. On Windows, reading other processes' CPU, memory, and handle info generally needs Administrator or Debug privilege; without it, per-process metrics for services and other users' processes are silently skipped. On macOS, some per-process metrics are unavailable regardless of privilege level (see Decision 3). Mitigation: document the privilege requirement in the install output and info box, phrased per-OS rather than as a single Linux-centric note; do not silently claim full coverage; do not force elevated privileges in this change.
- **Health-check port collision:** the reference config binds the `health_check` extension to a fixed port (13133). If left fixed, a second dtwiz collector deployed under the foreign-collector fallback (Decision 2) would fail to bind that port and crash at startup, breaking the fallback it was meant to support. Mitigation: give `health_check` a dynamically probed port, the same way `GRPCPort`, `HTTPPort`, and `MetricsPort` already are.
- **Template complexity:** the combined template is substantially larger, with ordered processors and platform conditionals. Mitigation: unit-test rendered output per platform (golden-file style), asserting receiver, processor, and pipeline keys and ordering; validate the config parses as YAML.
- **Extension format staleness:** the `transform` and `filter` rules must match what the Host Monitoring extension expects, and the reference config may evolve over time. Mitigation: mirror the pinned reference config in the template and note its source URL and version in a comment.
- **Cross-platform coverage:** macOS and Windows have no host-log pipeline at all in this change (not a reduced or substitute one), since `journald` has no equivalent enabled here. This is a distribution limitation, not a scoping choice: the Dynatrace OTel Collector distribution dtwiz downloads does not currently bundle `windowseventlogreceiver` or `macosunifiedloggingreceiver` (see Decision 3), and dtwiz has no way to add a receiver to a binary it only downloads. Mitigation: metrics-only is explicit and documented; it is not a regression, since nothing is collected host-side today. Revisit if a future Dynatrace distribution release bundles either receiver.
- **Rollback:** `uninstall otel` already removes the managed collector and config, so no additional rollback path is required.
