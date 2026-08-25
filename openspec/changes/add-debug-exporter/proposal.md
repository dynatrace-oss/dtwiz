## Why

When debugging OTel Collector data flow issues, engineers must manually edit the collector config to add the `debug` exporter. Adding it automatically when `--debug` is passed to `dtwiz install otel` (or `dtwiz update otel`) eliminates that friction and makes the debug flag useful for both dtwiz diagnostics and collector pipeline visibility in a single pass.

## What Changes

- The OTel Collector config template (`pkg/installer/otel/otel.tmpl`) gains a conditional `debug` exporter with `verbosity: normal`, emitted when the template's `Debug` field is true.
- All five pipelines (traces, metrics/apps, metrics/host, metrics, logs) gain `debug` as a second exporter when `Debug` is true.
- `otelConfigData` gains a `Debug bool` field, set from `logger.IsDebug()` in `generateOtelConfig`.
- Both `install otel` and `update otel` pick up the flag naturally via the shared `generateOtelConfig` path.

## Capabilities

### New Capabilities

- `otel-debug-exporter`: Conditional inclusion of the OTel `debug` exporter (verbosity: normal) in the generated collector config, controlled by the `--debug` CLI flag.

### Modified Capabilities

<!-- none — no existing spec-level behavior changes -->

## Impact

- `pkg/installer/otel/otel.tmpl`: template additions for exporter and pipeline entries.
- `pkg/installer/otel/collector.go`: `otelConfigData` struct and `generateOtelConfig` function.
- `pkg/installer/otel/otel_test.go`: new test case covering `Debug: true` template rendering.
- No new dependencies, no breaking changes, no feature flag needed.
