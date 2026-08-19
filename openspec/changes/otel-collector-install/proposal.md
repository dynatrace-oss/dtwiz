# Proposal: Route App Telemetry Through Local OTel Collector on Install

## Why

When `dtwiz install otel` (or any of the standalone `install otel-java / otel-node / otel-python`) instruments an application, it sets `OTEL_EXPORTER_OTLP_ENDPOINT` to the Dynatrace API URL directly. This means instrumented apps bypass the local OTel Collector entirely — the collector runs but receives no traffic from the apps dtwiz just launched.

The intended design is that all instrumented apps route their telemetry through the locally running collector, which then forwards to Dynatrace. The collector is the single egress point: it holds the credentials, handles batching and retries, and is the only component that needs to know the Dynatrace URL.

Routing apps directly to Dynatrace also means the `OTEL_EXPORTER_OTLP_HEADERS` authorization token ends up in every instrumented process's environment — a credential that the app has no business holding when the collector is available to own it.

## What Changes

- All runtimes instrumented by `install otel` and by the standalone `install otel-java / otel-node / otel-python` commands send their telemetry to `http://localhost:<collectorHTTPPort>` instead of to the Dynatrace API URL
- `OTEL_EXPORTER_OTLP_HEADERS` (the Dynatrace authorization token) is no longer injected into app process environments
- The collector HTTP port is determined accurately in both flows:
  - **Bundled flow** (`install otel`): uses the port resolved during collector plan preparation — exact, no disk read required
  - **Standalone flow** (`install otel-java/node/python`): reads the port from the installed collector config file; falls back to 4318 if absent

## Capabilities

### Modified Capabilities

- `install otel` (bundled): instrumented app env vars target `http://localhost:<port>` (HTTP, no auth header)
- `install otel-java`: same
- `install otel-node`: same
- `install otel-python`: same
- Go instrumentation guidance (printed env vars): same — points to local collector

### Unchanged

- The OTel Collector config is unaffected — it continues to hold the Dynatrace endpoint and token
- `update otel` connected-service detection and retargeting is unaffected
- `OTEL_EXPORTER_OTLP_PROTOCOL` remains `http/protobuf`
- All other OTel env vars (`OTEL_SERVICE_NAME`, `OTEL_TRACES_EXPORTER`, etc.) are unchanged
