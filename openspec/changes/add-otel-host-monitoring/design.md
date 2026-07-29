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
- Any write access to a third-party collector's config from `install otel`. This change only ever reads a foreign config (to check for a same-tenant hostmetrics conflict) and only ever writes to dtwiz's own managed config. Merging into a foreign config remains the job of the existing `update otel` command, not something `install otel` attempts on the user's behalf.
- Configurable scrape intervals, per-scraper toggles, or a `--host-monitoring` opt-out flag in this change. Host monitoring is on by default, matching the zero-config principle.
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

### Decision 2: Never write to a foreign collector; detect conflicts read-only and let the user choose

`install otel` manages `~/opentelemetry/config.yaml`, and that config is always regenerated with host pipelines included, regardless of what else is running on the host. `install otel` never writes to any other collector's config, under any circumstance. Attempting an automatic merge into a third-party config was considered and rejected: it is the same class of operation `update otel` already exists for, and building a second, automatic version of it inside `install otel` (extending the merge from a single exporter key to five processors and multiple named pipelines, then getting it right against an arbitrary third-party file with no visibility into its structure) adds meaningful risk to what is supposed to be the safe path. When it is not possible to be certain an action is safe, the safer default is to not take it.

What `install otel` does instead is a read-only check, purely to inform the user, never to act on their behalf:

1. Detect other running OTel Collector processes via [findDynatraceOtelCollectors and findAllRunningOtelCollectors](../../../pkg/installer/otel/collector_select.go).
2. For each one with a resolvable config path (the same path resolution `update otel` already uses), read the file (never write it) and check whether it already defines a `hostmetrics` receiver.
3. If it does, look for an exporter whose endpoint resembles a Dynatrace ingest URL and extract its tenant ID with the existing [ExtractTenantID](../../../pkg/installer/installer.go) helper, then compare it against the tenant ID of the environment `install otel` is configuring.

This produces three outcomes:

- **No hostmetrics receiver found, or a different tenant:** no conflict. Proceed exactly as today: dtwiz deploys or updates its own managed collector, coexisting via dynamically probed ports (including the new `health_check` port, see Decision 1).
- **Same tenant and a hostmetrics receiver already present:** a real duplication risk. Warn the user clearly and let them choose: skip host monitoring for this run (avoids the duplicate, recommended), or proceed anyway with dtwiz's own separate collector, fully informed. If the user would rather consolidate onto their existing collector, the output points them at `dtwiz update otel` as the tool for that; dtwiz does not attempt it automatically.
- **Tenant cannot be determined:** this is expected to be common, because the reference config's own exporter endpoint is `${env:DT_ENDPOINT}`, an environment-variable placeholder that dtwiz cannot resolve from another process's static config file. Since nothing is written to the foreign config in any of these outcomes, the residual risk here is duplicate *data* in Dynatrace, not a destructive or corrupting action against a file dtwiz does not own. Surface a note that the check was inconclusive, and proceed by default rather than blocking, since blocking would penalize the common case (foreign config uses an env var) for a risk that is recoverable, not destructive.

Why over alternatives: this keeps the entire change additive and reversible from the foreign collector's point of view. It removes the fragile-merge risk from this change's scope entirely, at the cost of not fully automating the "consolidate onto one collector" outcome. That is an acceptable trade, since `update otel` already exists for a user who wants that outcome deliberately.

### Decision 3: Platform-aware templating

`journald` is Linux-only, and no equivalent host-log receiver (such as Windows Event Log or macOS unified logging) is added in this change, so macOS and Windows simply get no host-log pipeline rather than a substitute. Reuse dtwiz's OS-suffix and build-tag pattern: the template (or a small config-data struct field like `IncludeJournald bool`) conditionally includes the `journald` receiver and `logs/host` pipeline. On Linux, include host logs. On macOS and Windows, emit host metrics only. `hostmetrics` scrapers are gated per-platform where a scraper is unsupported.

The `journald` receiver and its `logs/host` pipeline reference must be gated together, in both the `receivers:` block and the `service.pipelines` list, from the same struct field. If the two guards ever drift apart, the collector either fails to start (a pipeline references a receiver that was never defined) or carries a dead, unused receiver definition. Tests must render and parse the macOS/Windows config, not just check that the string `journald` is absent, so a mismatched guard is caught rather than passing on a surface-level string check.

Separately, some per-process `hostmetrics` metrics are unavailable on specific platforms independent of any conditional gating, for example per-process disk IO is not implemented on macOS at all. The reference config already sets `mute_process_all_errors: true` on the `process` scraper so these gaps degrade to missing data points rather than startup failures; this behavior is carried through unchanged.

Why over alternatives: a single template with conditionals keeps one source of truth. Fully separate per-OS templates would duplicate the large shared metrics section. dtwiz already renders OS differences via template data elsewhere.

### Decision 4: Preview and verification unchanged

The combined config is printed inline in the existing preview ([printConfigPreview](../../../pkg/installer/otel/collector.go)) with the token masked, under the single `Apply? [Y/n]` prompt. The existing OTLP round-trip verification still exercises the shared `otlp` receiver and `otlp_http` exporter, so it validates the collector regardless of the added host pipelines. Host metrics arrival is not separately verified in this change; the DQL log round-trip remains the signal that the collector is up and exporting.

## Risks / Trade-offs

- **Privilege gaps, per platform:** full host metrics and logs need elevated privileges, but the mechanism differs by OS, not just Linux. On Linux, some `hostmetrics` scrapers (process, disk, filesystem) and `journald` need root or the `systemd-journal` group. On Windows, reading other processes' CPU, memory, and handle info generally needs Administrator or Debug privilege; without it, per-process metrics for services and other users' processes are silently skipped. On macOS, some per-process metrics are unavailable regardless of privilege level (see Decision 3). Mitigation: document the privilege requirement in the install output and info box, phrased per-OS rather than as a single Linux-centric note; do not silently claim full coverage; do not force elevated privileges in this change.
- **Health-check port collision:** the reference config binds the `health_check` extension to a fixed port (13133). If left fixed, a second dtwiz collector deployed under the foreign-collector fallback (Decision 2) would fail to bind that port and crash at startup, breaking the fallback it was meant to support. Mitigation: give `health_check` a dynamically probed port, the same way `GRPCPort`, `HTTPPort`, and `MetricsPort` already are.
- **Duplicate host metrics:** if a foreign collector already scrapes hostmetrics for the same tenant and dtwiz also deploys host pipelines, host metrics could double-count. Mitigation: read-only detection of an existing `hostmetrics` receiver plus a same-tenant check (Decision 2) surfaces a warning and an explicit skip-or-proceed choice instead of silently deploying. This mitigation is incomplete when the foreign config's exporter endpoint is templated via an environment variable, as the reference config itself is; tenant comparison is then inconclusive, and the design proceeds by default with a note rather than blocking, since the residual risk is duplicate data, not a destructive write.
- **Template complexity:** the combined template is substantially larger, with ordered processors and platform conditionals. Mitigation: unit-test rendered output per platform (golden-file style), asserting receiver, processor, and pipeline keys and ordering; validate the config parses as YAML.
- **Extension format drift:** the `transform` and `filter` rules must match what the Host Monitoring extension expects, and the reference config may evolve over time. Mitigation: mirror the pinned reference config in the template and note its source URL and version in a comment.
- **Cross-platform coverage:** macOS and Windows have no host-log pipeline at all in this change (not a reduced or substitute one), since `journald` has no equivalent enabled here. Mitigation: metrics-only is explicit and documented; it is not a regression, since nothing is collected host-side today.
- **Rollback:** `uninstall otel` already removes the managed collector and config, so no additional rollback path is required.

## Open Questions

- Should the retained app-logs pipeline and the host `logs/host` pipeline be merged into one `logs` pipeline (otlp and journald to resource_detection) or kept separate? Leaning toward merged on Linux to mirror the reference config, but this changes app-log processing (it adds resource_detection). Confirm this is acceptable.
- Should there be an escape hatch (flag or feature flag) to skip host monitoring for users who only want app instrumentation? Current decision: no, on by default; revisit if it proves disruptive.
