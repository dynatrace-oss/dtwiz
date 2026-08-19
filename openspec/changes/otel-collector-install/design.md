# Design: Route App Telemetry Through Local OTel Collector on Install

## Context

`generateBaseOtelEnvVars` in `pkg/installer/otel/env.go` builds the environment passed to every instrumented process. It currently takes `apiURL` and `token`, producing:

```text
OTEL_EXPORTER_OTLP_ENDPOINT = <apiURL>/api/v2/otlp
OTEL_EXPORTER_OTLP_HEADERS  = Authorization=Api-Token%20<token>
OTEL_EXPORTER_OTLP_PROTOCOL = http/protobuf
```

The collector is installed first in the bundled flow and is already listening by the time the runtime plan executes — but apps never send to it. The goal is to make apps route through `http://localhost:<httpPort>` instead, with no auth header (the collector holds credentials).

There are two entry points for runtime instrumentation that need to participate:

- **Bundled flow** (`InstallOtelCollectorWithProject` in `otel.go`): the collector plan is prepared before the runtime plan; `collectorPlan.httpPort` is already the resolved port.
- **Standalone flows** (`InstallOtelJava`, `InstallOtelNode`, `InstallOtelPython` in their respective files): called independently with no collector plan in scope; the collector may already be installed on disk.

## Goals / Non-Goals

**Goals:**

- Apps launched by `install otel` (all runtimes) send to `http://127.0.0.1:<port>` not to Dynatrace directly
- Port is accurate: bundled flow uses `cp.httpPort`; standalone flows read from installed config with 4318 fallback
- No auth token in app process environments

**Non-Goals:**

- Changing the collector config or its exporter setup — the collector side is unchanged
- Changing `update otel` — connected-service retargeting already works correctly
- Supporting gRPC (4317) — HTTP (4318) is already the configured protocol; no protocol change needed

## Decisions

### 1. Change `generateBaseOtelEnvVars` signature

The function changes from `(apiURL, token, serviceName string)` to `(collectorEndpoint, serviceName string)`, where `collectorEndpoint` is a full URL like `http://127.0.0.1:4318`.

`OTEL_EXPORTER_OTLP_HEADERS` is removed from the output. The auth token belongs in the collector config, not in the app environment.

All callers that previously passed `apiURL` and `token` now pass a pre-built collector endpoint string. This is a mechanical change with a narrow blast radius — only the `otel` package is affected.

### 2. Two port sources — one per flow type

**Bundled flow**: `prepareCollectorPlan` already resolves the HTTP port via `findFreePort(4318)` and stores it in `collectorPlan.httpPort`. This value is passed into `createRuntimePlan` as a new parameter, which builds the collector endpoint string `http://127.0.0.1:<httpPort>` before calling `generateBaseOtelEnvVars`.

Trying to read from the config file at plan time would require the file to already be written — but `cp.execute()` writes the config after the user confirms. Using `cp.httpPort` directly avoids this chicken-and-egg problem and keeps the preview consistent with execution.

**Standalone flows**: No collector plan is in scope. The installed config is read via the existing `otlpHTTPPortFromConfig(findExistingCollectorConfig())` helper (already present in `collector.go`), which returns 4318 on any read or parse failure. This is the same mechanism used by `update otel` for port-aware verification.

### 3. Use `127.0.0.1` instead of `localhost`

All collector endpoint strings are constructed as `http://127.0.0.1:<port>` rather than `http://localhost:<port>`. On dual-stack systems, `localhost` resolves to `::1` (IPv6) while the collector binds its OTLP receivers to `0.0.0.0` (IPv4 only). Using the explicit loopback address avoids the ambiguity. The same fix applies to the `collectorEndpoint` constructed in `update_dynatrace.go` for retargeting connected services.

### 4. Runtime-specific env var helpers are updated, not replaced

`generateOtelNodeEnvVars` and `generateOtelPythonEnvVars` extend `generateBaseOtelEnvVars` — their signatures change in lockstep. Java and Go use `generateBaseOtelEnvVars` directly. All four runtimes end up with the same base set plus any runtime-specific additions.

### 5. Preview and execution are consistent

Because `cp.httpPort` is resolved at `prepareCollectorPlan` time, the runtime plan preview (`PrintPlanSteps`) can display the correct collector endpoint before user confirmation. There is no deferred resolution.

## Risks / Trade-offs

- **[No collector running — warn, don't fail]** Today, `install otel-java` (and other standalone installs) export directly to Dynatrace and work without a local collector. After this change, they point to `127.0.0.1:<port>`, so telemetry is dropped if no collector is running on that port. Standalone installers perform a TCP dial to `127.0.0.1:<port>` after writing env vars; if nothing answers, a clear warning is printed ("collector not reachable on port X — telemetry will not reach Dynatrace until a collector is started") and the install exits cleanly. Hard failure is too aggressive: the user may be preparing env vars for a process that starts alongside the collector later.
- **[Port fallback mismatch]** If the collector was installed on a non-default port and the config read fails, the app gets 4318 but the collector listens on a different port. The fallback is the same one used by `update otel` today — consistent, even if imperfect.
