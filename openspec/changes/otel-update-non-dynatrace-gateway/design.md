# OTel Update: Non-Dynatrace Collector Gateway Design

## Context

`dtwiz update otel` ([update.go](../../../pkg/installer/otel/update.go)) lets a user pick a running OTel Collector and adds the Dynatrace exporter to its config via `mergeDynatraceExporter`, which unmarshals the config into a `map[string]interface{}`, mutates it, and re-marshals with `yaml.Marshal` (comments, key order, and flow style are not preserved by this approach). After writing the patched config, it restarts the collector by killing the detected PID(s) (`killCollectorProcesses`) and relaunching the binary directly (`startOtelCollector(binaryPath, configPath)`), or by calling `docker/podman restart` for containers.

This is correct and safe when the collector is dtwiz's own managed Dynatrace distro: dtwiz knows exactly how it was started, owns its lifecycle, and the distro guarantees every component the merge might need. It is not safe applied unmodified to a **foreign** (non-Dynatrace) collector:

- **Process ownership:** a foreign collector is typically supervised by systemd, docker-compose, or a Kubernetes controller. Killing its PID directly fights that supervisor — it may spawn a replacement (port conflicts, config drift) or leave an orphaned, unsupervised relaunch.
- **Component availability:** host monitoring needs `hostmetrics`, `filter`, `transform`, and `resource_detection` components. These are bundled in `otelcol-contrib`, the Datadog Agent OTel distro, and Grafana Alloy, but not in a minimal core-only `otelcol` build. dtwiz cannot assume they exist.
- **Config ownership and secret hygiene:** a foreign config file is likely owned by configuration management (Helm, Ansible, Puppet) or version control, and inlining a Dynatrace token into it is a hygiene problem dtwiz shouldn't introduce.
- **Lossy relaunch:** `otelInfoFromPID` ([detect_otel.go](../../../pkg/analyzer/detect_otel.go)) captures only the binary path and a single `--config` value from `ps` output, which is too lossy to faithfully relaunch a process that may have multiple `--config` flags, `--feature-gates`, `--set` overrides, or environment-driven configuration.

This change adds a dedicated flow for the non-Dynatrace branch of `update otel`, following the UX in the flow diagram supplied for this change. It depends on the host-monitoring-capable config generation already implemented in `add-otel-host-monitoring` (`generateOtelConfig`, gated behind `featureflags.Experimental`), reused here for the new gateway collector's own config.

## Goals / Non-Goals

**Goals:**

- Let a non-Dynatrace collector's existing telemetry also reach Dynatrace, without altering its existing destination, receivers, or processors.
- Add host monitoring safely regardless of what OTel distro or components the existing collector has.
- Never restart a supervised foreign process by killing its PID directly; use the supervisor's own restart mechanism, or hand off to the user when that isn't safely possible.
- Never write a Dynatrace token or Dynatrace-specific processors into a config file dtwiz doesn't own.
- Make every filesystem change reversible: back up before writing, and always tell the user where the backup and new config are.

**Non-Goals:**

- Kubernetes pod/ConfigMap-sourced collectors. Detecting and patching these needs a fundamentally different mechanism (cluster-targeting via `kubectl`, not host process scanning) and a different consent model (mutating shared cluster state). Out of scope this iteration — always routed to the docs-link branch.
- Reconciling a previously-deployed gateway collector's host-monitoring block against a newer reference config on subsequent runs. The gateway is freshly deployed each time this flow completes for a given host in this iteration.
- Any change to the existing Dynatrace-collector `update otel` path (`mergeDynatraceExporter`) — untouched.
- Isolating the forwarded stream from further processing via a `forward` connector. The gateway receives exactly what the foreign collector's existing pipelines already carry and does all Dynatrace-specific processing itself; no processor injection into the foreign config is needed or added.
- Modifying, removing, or reordering anything in the foreign collector's existing receivers, processors, pipelines, or other exporters.

## Decisions

### Decision 1: Deploy a dedicated Gateway Collector rather than merging Dynatrace/host-monitoring logic directly into the foreign config

The foreign collector receives exactly one small, generic addition (Decision 2). All Dynatrace-specific work — authentication, retries, and host monitoring — runs on a new, dtwiz-owned **Dynatrace Gateway Collector**, deployed the same way `install otel` deploys its managed collector, listening on `localhost:4319`.

Why over in-place merging: merging host monitoring and Dynatrace export logic directly into the foreign config would require a component-availability check as a fallback for distros missing `hostmetrics`/`filter`/`transform`/`resource_detection` — meaning the fallback path would still be needed in exactly the cases that matter most, so in-place merging doesn't actually eliminate complexity, it relocates it. The gateway pattern needs almost nothing from the foreign distro: just a generic `otlp` exporter, present in effectively every OTel distribution including minimal core-only builds. Host monitoring is then unconditionally safe, since it never depends on the foreign distro's component set.

### Decision 2: The foreign-config patch is exactly one additive exporter, appended to existing pipelines

Reuses the same unmarshal-mutate-remarshal technique `mergeDynatraceExporter` already uses (`map[string]interface{}` via `yaml.Unmarshal`/`yaml.Marshal`, not a node-level edit — the existing code does not preserve comments, key order, or flow style, and this change does not change that). The exporter definition differs from the Dynatrace-collector case:

```yaml
otlp/dt-gateway:
  endpoint: localhost:4319
  tls:
    insecure: true
  sending_queue:
    block_on_overflow: false
```

- No `Authorization` header — no secret ever appears in the foreign config.
- `block_on_overflow: false` ensures that if the gateway collector is slow or down, its queue drops rather than backpressures the foreign collector's original exporter(s).
- The exporter name is appended to each existing pipeline's `exporters:` list; no existing exporter, receiver, or processor is modified.

### Decision 3: Config-source validation gates the entire flow — on failure, nothing is deployed

Before any backup, supervisor detection, or gateway deployment, dtwiz resolves the effective config source from the running process and confirms it is a **single, writable, local YAML file**. This check fails when:

- Multiple `--config` flags are present (merged configs).
- The config is provided via an `env:`/`yaml:` inline provider (no file at all).
- The config is baked into a container image with no durable write-back path (a `docker cp` write would be silently discarded on the next image pull/redeploy).
- The resolved path is not writable (permission denied).
- The collector is Kubernetes pod/ConfigMap-sourced (Non-Goal; always treated as a validation failure in this iteration).

**On failure, dtwiz makes no changes of any kind** — no backup, no gateway collector, no config write — and shows a docs link explaining how to add the Dynatrace exporter and host monitoring manually. This matches the flow diagram exactly: the "config source is not a single writable file" branch bypasses backup, supervisor detection, and gateway deployment entirely.

### Decision 4: Supervisor detection determines whether dtwiz attempts an automatic restart

- **Systemd unit** (Linux, detected via `/proc/<pid>/cgroup` containing a `*.service` path, distinct from a generic `user.slice`/session scope) or **docker/docker-compose container** (existing detection): dtwiz can restart automatically, using `systemctl restart <unit>` or `docker restart <container>` — never a raw `kill`.
- **Bare/manual process:** dtwiz can restart automatically only if it successfully captures the full original launch context (argv, env, cwd — Decision 6). If capture is incomplete or ambiguous, treat as cannot-restart.
- **Kubernetes pod, or supervisor undetectable/ambiguous:** cannot-restart in this iteration; always routed to the manual-restart branch.

Why: killing a supervised process directly either causes the supervisor to spawn a duplicate (port conflicts, config drift) or leaves an unsupervised orphan. dtwiz only takes an automatic action it can be confident is both correct (the collector actually comes back with the new config) and safe (it isn't fighting something else that manages this process).

### Decision 5: Restart outcome determines the final step — WatchIngest vs. manual instructions + Services/Tracing links

- **Automatic restart attempted and succeeds:** run the existing `WatchIngestWithStatus` flow ([ingest_watch.go](../../../pkg/installer/ingest_watch.go)) so the user sees ingest confirmed live, consistent with other install/update flows.
- **Automatic restart attempted and fails, OR dtwiz determined it cannot restart automatically (Decision 4):** print manual restart instructions — the new config path and the backup path from Decision 3 — with brief guidance tailored to the detected supervisor when known. Then show links to the Dynatrace Services and Distributed Tracing apps instead of running `WatchIngest`, since dtwiz cannot know if or when the user completes a manual restart, and polling indefinitely would be misleading.

Both the "restart failed" and "cannot restart automatically" branches converge on this same manual-instructions + Services/Tracing-links outcome, matching the flow diagram.

### Decision 6: Extend process detection to capture full launch context

`otelInfoFromPID` today parses `ps -o args=` output for the binary path and a single `--config=`/`--config` value — sufficient for dtwiz's own managed collector (which it started itself with exactly that invocation) but too lossy to faithfully relaunch an arbitrary bare/manual foreign process. This change adds capture of:

- **Linux:** `/proc/<pid>/cmdline` (NUL-separated, exact), `/proc/<pid>/environ`, `/proc/<pid>/cwd`.
- **macOS:** `sysctl kern.procargs2` via `golang.org/x/sys/unix`, falling back to the existing `ps`-based capture if unavailable.
- **Windows:** WMI `CommandLine` (existing pattern used elsewhere in the codebase for process inspection).

This full context is used only for the bare/manual restart path (Decision 4); the systemd/docker paths restart via the supervisor's own command and don't need it.

### Decision 7: Ship behind `--experimental`, same convention as today

No change to the existing gating. `update otel` remains hidden and inert unless `--experimental` or `DTWIZ_EXPERIMENTAL=true` is set, consistent with `install docker` and the pre-existing `update otel` command.

## Risks / Trade-offs

- **Second process to run and monitor:** the gateway collector is an additional long-running process on the host, with its own memory footprint and its own potential failure mode independent of the foreign collector.
- **Restart still causes a brief telemetry gap:** the foreign collector must actually restart to pick up the new exporter — there is no universal OTel hot-reload. This is the same trade-off `update otel` already accepts today; it is not new to this change.
- **Kubernetes scope gap:** users whose foreign collector runs as a k8s pod get only the docs-link path in this iteration, not an automated ConfigMap patch + rollout restart. Flagged explicitly as future work, not silently unsupported.
- **Supervisor misdetection:** if dtwiz incorrectly classifies a process as safely bare-restartable when it is in fact loosely supervised (e.g., a shell wrapper that respawns it), an automatic restart could produce a duplicate or orphaned process. Mitigation: only attempt an automatic bare-process restart when the full launch context (Decision 6) was captured with high confidence; when in doubt, fall back to the manual path.
- **Rollback:** because the foreign-config patch is a single additive block, restoring the pre-patch config from the printed backup path trivially reverses the change. The gateway collector can be independently stopped and removed via the same uninstall mechanics as `install otel`'s managed collector.
