# Design: opentelemetry-host-group

## Context

The OTel Collector config is generated from `otel.tmpl` (an embedded Go template) by `generateOtelConfig()` in `pkg/installer/otel/collector.go`. The template data model is `otelConfigData`, a plain Go struct. Port numbers, endpoint, and auth header are already resolved in Go and passed into the template as fields.

The OTel Collector supports named processor instances via the `type/name` convention (e.g., `resource/add-host-group-id`). The `resource` processor type operates on resource attributes — the outer metadata envelope shared across all signals from a given source — which is where `dt.host_group.id` belongs per the OTel semantic conventions.

## Goals / Non-Goals

**Goals:**

- All telemetry flowing through the dtwiz-installed collector carries `dt.host_group.id` set to the machine hostname.
- The change is minimal: one new field on `otelConfigData`, one new processor block in the template, one additional entry in each pipeline's processor list.
- Works for both standard (app-only) and experimental (host monitoring) installs.

**Non-Goals:**

- Making `dt.host_group.id` configurable by the user — dtwiz targets people trying things out, not power users.
- Resolving hostname dynamically at collector runtime — static embedding at install time is sufficient.
- Migrating users who already have a collector installed — `dtwiz update otel` is the path for that.

## Decisions

### Resolve hostname at config generation time (not at collector runtime)

**Decision:** Call `os.Hostname()` in `generateOtelConfig()` and embed the result as a literal string in the config file.

**Rationale:** The config is already a static file generated once at install time. All other dynamic values (endpoint, auth header, ports) follow the same pattern. Using the OTel Collector's `${env:HOSTNAME}` substitution syntax would require an extra env-var to be set before the collector starts, adding complexity for no practical benefit — hostnames rarely change after install.

**Alternative considered:** `${env:HOSTNAME}` or `${env:HOST}` in the config. Rejected because it requires a startup-time env var and adds a failure mode (collector refuses to start if the variable is absent).

### Use `upsert` action (not `insert`)

**Decision:** Use `action: upsert` so the processor always sets `dt.host_group.id` to the machine hostname, overwriting any value the application may have sent.

**Rationale:** dtwiz's goal is that all data from a machine ends up in the same Dynatrace group. Using `insert` would allow application-set values to silently bypass the grouping, creating split groups without the user realizing it. Applications instrumented by dtwiz (Node.js, Python, Java auto-instrumentation) do not set `dt.host_group.id` by default, so in practice upsert and insert are equivalent today — but upsert is the more robust default for the future.

**Alternative considered:** `insert` to respect app-set values. Rejected because dtwiz's target audience (people trying things out) is unlikely to set `dt.host_group.id` intentionally, and silent split-grouping is worse than a predictable override.

### Name the processor `resource/add-host-group-id`

**Decision:** Use the `type/name` convention with name `add-host-group-id`.

**Rationale:** Consistent with the existing `filter/delete-metrics` naming style in the template (verb-noun, kebab-case). The name mirrors the attribute being set, making the config self-documenting.

### Apply to all pipelines unconditionally

**Decision:** Wire `resource/add-host-group-id` into every pipeline regardless of the `HostMonitoring` flag.

**Rationale:** Machine-level grouping is valuable for service telemetry (traces, metrics, logs from apps) independently of whether host metrics are enabled. The processor is cheap and its presence in non-host-monitoring configs does not affect those configs' behavior in any other way.

## Risks / Trade-offs

- **Hostname at install time is static.** If the machine is renamed after install, `dt.host_group.id` will be stale until the user reinstalls. Acceptable: hostnames rarely change, and the user can re-run `dtwiz install otel` to regenerate the config.
- **Upsert silently overrides app-set values.** An application that deliberately sets `dt.host_group.id` will have it overwritten. Acceptable given the target audience and the goal of guaranteed machine-level grouping.
