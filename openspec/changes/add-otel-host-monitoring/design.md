# OTel Host Monitoring Design

## Context

`install otel` deploys a single dtwiz-managed Dynatrace OTel Collector at `~/opentelemetry/` with a config rendered from [otel.tmpl](../../../pkg/installer/otel/otel.tmpl). That config is an application gateway only: an `otlp` receiver, a `cumulative_to_delta` processor, and an `otlp_http` exporter to `/api/v2/otlp`. The collector is started as a detached background process ([startOtelCollector](../../../pkg/installer/otel/collector.go)), then a round-trip verification log is sent through it and confirmed via DQL.

The Dynatrace [host-metrics reference config](https://github.com/Dynatrace/dynatrace-otel-collector/blob/main/config_examples/host-metrics.yaml) collects host signals for the **OpenTelemetry Host Monitoring** extension. It adds:

- Receivers: `hostmetrics/10s`, `hostmetrics/5m`, `hostmetrics/1h`, `journald`, `otlp`
- Processors: `filter`, `resource_detection`, `transform`, `filter/delete-metrics`, `cumulative_to_delta`
- Pipelines: `metrics` (hostmetrics through processors to otlp_http), `logs` (otlp and journald through resource_detection to otlp_http)
- Extension: `health_check`

Both configs export to the same `/api/v2/otlp` endpoint with the same auth header. They differ only in receivers, processors, and pipeline wiring. A single OTel Collector process runs many pipelines keyed by `type/name`, so both concerns fit in one collector.

Note that the reference `host-metrics.yaml` is host-only: it has no traces pipeline and no `otlp` receiver feeding metrics. It cannot simply replace `otel.tmpl`, or app traces and app metrics would be lost. The combined template produced by this change is therefore a superset of the current `otel.tmpl`: it keeps every existing app pipeline unchanged and adds the host pipelines alongside them. A run of `install otel` after this change gives users everything a run before this change gave them (app traces, app metrics, app logs over OTLP), plus host metrics and a host logs pipeline on every platform, with systemd journal logs additionally collected on Linux via the `journald` receiver, all through the one managed collector. No second collector is needed for full coverage; dtwiz always deploys and manages its own single collector regardless of what else is running on the host, exactly as it does today for app-only monitoring (see Decision 2 for what happens when another collector already exists).

## Goals / Non-Goals

**Goals:**

- `install otel` collects host metrics and (on Linux) host logs in addition to app signals, in the format the Host Monitoring extension expects.
- Reuse the existing single managed collector, lifecycle, preview, and verification. No new service-management machinery.
- Deterministic, testable config generation from a template dtwiz fully controls.
- Cross-platform: host metrics and a host logs pipeline on all platforms; the `journald` receiver is added to the logs pipeline on Linux only, as no equivalent host-log receiver is enabled for macOS or Windows in this change.

**Non-Goals:**

- Persistent OS-service installation (systemd, launchd, Windows service). Lifecycle stays as the current detached process.
- Any interaction with other OTel Collectors running on the host. `install otel` only writes its own managed config and does not read, detect, or react to foreign collector configurations.
- Configurable scrape intervals, per-scraper toggles, or a permanent `--host-monitoring` opt-out flag. Host monitoring is on by default once promoted out of the temporary `--experimental` rollout gate (see Decision 5), matching the zero-config principle.
- Changes to `update otel` or `uninstall otel`.
- Activating the **OpenTelemetry Host Monitoring** extension in the Dynatrace environment. This change ships host data in the shape the extension expects, but activating the extension (so host/process entities and Infrastructure & Operations visualizations appear) is a separate change.
- Kubernetes node host monitoring (DaemonSet-based collection). This change targets the host-local collector process only; K8s node coverage is out of scope.

## Decisions

### Decision 1: One collector, combined config (regenerate, don't text-merge)

Extend `otel.tmpl` into a single combined config carrying both app and host pipelines. Because dtwiz owns the template, we generate the merged structure directly rather than parsing and injecting.

- The existing app metrics pipeline is renamed `metrics/apps` (otlp to cumulative_to_delta to otlp_http).
- New `metrics/host` pipeline: `hostmetrics/*` to filter to resource_detection to transform to filter/delete-metrics to cumulative_to_delta to otlp_http.
- New `logs/host` pipeline (all platforms): resource_detection to otlp_http, fed by otlp and — on Linux only — journald. The existing `logs` pipeline (otlp-only) is retained for app logs, or merged with journald where appropriate.
- Single shared `otlp_http` exporter and `health_check` extension. The reference config binds `health_check` to a fixed port (13133); the combined template instead gives it a dynamically probed port (an additional `HealthCheckPort` field alongside the existing `GRPCPort`, `HTTPPort`, and `MetricsPort` fields), so it does not conflict when a second dtwiz collector runs on the same host (see Decision 2).

Why over alternatives: deep-merging YAML (as [mergeDynatraceExporter](../../../pkg/installer/otel/update.go) does for a single exporter) is reliable for one key but fragile across five processors with ordering constraints and multiple named pipelines. Since dtwiz always writes its own config on `install otel`, regeneration is both simpler and deterministic. Two separate collectors were rejected for the managed case because only one collector per host should scrape `hostmetrics` (otherwise host metrics double-count), and a single process shares the exporter, ports story, and lifecycle.

**Existing installs upgrade for free by rerunning `install otel`.** `prepareCollectorPlan` ([collector.go](../../../pkg/installer/otel/collector.go)) always renders `otel.tmpl` from scratch and `execute()` always stops the running collector, overwrites `config.yaml` unconditionally, and restarts it. There is no "config already exists, skip" branch anywhere in this path. This means a user who installed the old app-only collector with a previous `dtwiz install otel` gets host monitoring automatically the next time they run `dtwiz install otel` after this change ships: the old process is stopped, the config is fully replaced with the new combined template, and the collector restarts with host pipelines included, using the same install directory, exporter, and token. No separate migration command or one-time upgrade task is needed; the existing overwrite-and-restart behavior is the upgrade path.

### Decision 2: Always regenerate dtwiz's managed collector; other running collectors are ignored

`install otel` manages `~/opentelemetry/config.yaml` and always regenerates the config with host pipelines included. What else is running on the host is irrelevant: no foreign collector is read, checked, or written to. If another OTel Collector already collects host metrics for the same tenant, duplicate data is a possible side-effect, but that is a user concern outside the scope of this command. Users who want to consolidate collectors can use `update otel` separately.

Why over the alternative (read-only conflict detection): the simpler model is appropriate for a new install. Conflict detection adds code paths, prompts, and test surface for a scenario that only arises on hosts that already have an existing OTel setup, not the target audience for `install otel`.

### Decision 3: Platform-aware templating

The `logs/host` pipeline is included on all platforms. `journald` is Linux-only: per Dynatrace docs, running host monitoring on macOS or Windows requires removing all `journald` references from the pipeline. The pipeline still runs on those platforms using the `otlp` receiver and `resource_detection` processor; it just collects no journald-sourced logs. A small config-data struct field (`IncludeJournald bool`) gates the `journald` receiver definition only; the pipeline itself is always present.

This gating is mandatory for correctness, not just completeness. The Dynatrace docs are explicit that a `journald` receiver present on a non-Linux OS causes the Collector to return an error and exit on startup. Leaving journald in the config on macOS or Windows therefore crashes the collector outright rather than degrading to missing logs, so both the receiver definition and its pipeline reference must be absent on those platforms.

Note: native platform log receivers (`windowseventlogreceiver` for Windows, `macosunifiedloggingreceiver` for macOS) are not in the Dynatrace OTel Collector distribution dtwiz downloads (see task 0.3), so no substitute for journald is added in this change.

The `journald` receiver definition and its reference in the `logs/host` pipeline must be gated by the same struct field. If the two guards drift apart, the collector either fails to start (pipeline references an undefined receiver) or carries a dead receiver definition. Tests must unmarshal the rendered YAML and check the `receivers` and `service.pipelines` maps directly, not just scan for the string `journald`, so any mismatch is caught.

Separately, some `hostmetrics` metrics have platform or privilege constraints independent of any conditional gating. Per the Dynatrace docs: `process.disk.io` is captured only when the collector runs with privileged access, otherwise it is dropped; `system.processes.created` is available on Linux only; on macOS, both `system.processes.created` and `process.disk.io` are unavailable regardless of privilege level and will not appear in Dynatrace (macOS does not expose per-process disk I/O through standard APIs, and `system.processes.created` reads from Linux's `/proc/stat`). The reference config already sets `mute_process_all_errors: true` on the `process` scraper so these gaps degrade to missing data points rather than startup failures; this behavior is carried through unchanged.

Why over alternatives: a single template with conditionals keeps one source of truth. Fully separate per-OS templates would duplicate the large shared metrics section. dtwiz already renders OS differences via template data elsewhere.

### Decision 4: Preview truncated by default; full config on --debug

The combined config is printed inline in the existing preview ([printConfigPreview](../../../pkg/installer/otel/collector.go)) with the token masked, under the single `Apply? [Y/n]` prompt. The existing OTLP round-trip verification still exercises the shared `otlp` receiver and `otlp_http` exporter, so it validates the collector regardless of the added host pipelines. Host metrics arrival is not separately verified in this change; the DQL log round-trip remains the signal that the collector is up and exporting.

The combined config is substantially larger than the previous app-only config (~250 lines vs ~35 lines). Printing it verbatim as part of the confirmation prompt produces excessive terminal output that obscures the install flow. The preview is therefore truncated to the first 20 lines by default — enough to show the collector's listening endpoints (health check, gRPC, HTTP) — with a note directing users to rerun with `--debug` to see the full config. With `--debug` the full config is printed verbatim, matching what is written to disk. The config file itself is always written in full regardless of the preview setting.

Why over alternatives: a structural summary (receiver/processor names only) was considered but rejected as too vague — customers need to see what will actually be collected. Filtering `enabled: false` entries (which only describe what won't happen) or stripping `enabled:` flags entirely were considered but rejected as making the preview diverge from the file on disk in a non-obvious way. Head truncation with a `--debug` escape hatch keeps the preview honest (every shown line is verbatim from the file) while containing the output size.

### Decision 5: Ship behind `--experimental` until fully implemented and tested

This entire change ships behind the existing `--experimental` / `DTWIZ_EXPERIMENTAL` feature flag, the same convention already used for `install docker` ([install.go](../../../cmd/install.go)) and `update otel` ([update.go](../../../cmd/update.go)). Unlike those two, host monitoring is not a separate subcommand; it is new behavior inside the existing `install otel` command, so the gate applies to config generation rather than command visibility:

- `generateOtelConfig` / `prepareCollectorPlan` ([collector.go](../../../pkg/installer/otel/collector.go)) SHALL check `featureflags.IsEnabled(featureflags.Experimental)`. When disabled (the default), the collector config, preview, and install flow are byte-for-byte the same as before this change: app-only pipelines, no `hostmetrics`/`journald`/`health_check` receivers or extensions, no privilege notice.
- When enabled, the full combined config from Decisions 1–4 is generated: host pipelines added and the privilege notice is shown.
- The install output makes the gate discoverable: when host monitoring is inactive because the flag is off, a single line notes it can be enabled with `--experimental` or `DTWIZ_EXPERIMENTAL=true`, instead of silently doing nothing.

Why: this change is substantial, untested-in-the-wild surface: cross-platform `hostmetrics` behavior (task 0.2/0.3 still open) and a template large enough to risk regressions in the existing app pipelines it must preserve unchanged. Shipping it gated avoids exposing every `install otel` user to that risk the moment the code merges, while still allowing it to be exercised end-to-end (including the manual per-platform verification in task 6.3) before promotion. This does not conflict with the "zero-config, on by default" goal; that is the target end state once the gate is lifted, not the initial rollout condition. Promotion (removing the gate so host monitoring is unconditionally on) happens once tasks 0–6 are complete and verified, not as part of the initial merge.

## Risks / Trade-offs

- **Privilege gaps, per platform:** full host metrics and logs need elevated privileges, but the mechanism differs by OS, not just Linux. On Linux, some `hostmetrics` scrapers (process, disk, filesystem) and `journald` need root or the `systemd-journal` group. On Windows, reading other processes' CPU, memory, and handle info generally needs Administrator or Debug privilege; without it, per-process metrics for services and other users' processes are silently skipped. On macOS, `system.processes.created` and `process.disk.io` are unavailable regardless of privilege level and will not appear in Dynatrace — macOS does not expose per-process disk I/O through standard APIs and `system.processes.created` is Linux-only (see Decision 3). Mitigation: document the privilege requirement in the install output, phrased per-OS and naming the specific metrics on macOS; do not silently claim full coverage; do not force elevated privileges in this change.
- **Health-check port collision:** the reference config binds the `health_check` extension to a fixed port (13133). If left fixed and another collector (dtwiz or third-party) already holds that port, the managed collector fails to start. Mitigation: give `health_check` a dynamically probed port, the same way `GRPCPort`, `HTTPPort`, and `MetricsPort` already are.
- **Template complexity:** the combined template is substantially larger, with ordered processors and platform conditionals. Mitigation: unit-test rendered output per platform (golden-file style), asserting receiver, processor, and pipeline keys and ordering; validate the config parses as YAML.
- **Extension format staleness:** the `transform` and `filter` rules must match what the Host Monitoring extension expects, and the reference config may evolve over time. Faithful mirroring is what produces the resource attributes the extension keys on for entity creation (`host.id`, `host.name`, `dt.metrics.source=opentelemetry`, and `process.executable.name` for process entities); drift here means data still arrives but no host or process entities are formed. Mitigation: mirror the pinned reference config in the template and note its source URL and version in a comment.
- **Cross-platform log coverage:** macOS and Windows get a `logs/host` pipeline but no `journald` receiver, so only OTLP-forwarded logs are collected on those platforms; journald-sourced host logs are Linux-only. Native platform log receivers (`windowseventlogreceiver`, `macosunifiedloggingreceiver`) are not in the Dynatrace distribution and cannot be added by dtwiz (see Decision 3 and task 0.3).
- **Rollback:** `uninstall otel` already removes the managed collector and config, so no additional rollback path is required.
