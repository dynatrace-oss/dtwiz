# add-debug-exporter: Design

## Context

The OTel Collector config is generated from `pkg/installer/otel/otel.tmpl` via `renderOtelTemplate(otelConfigData)`. The `otelConfigData` struct carries all template variables. The `--debug` flag sets `logger.IsDebug()` to true; this is already used inside the otel package (e.g., `printConfigPreview`) to react to debug mode without needing to thread the flag explicitly through function signatures.

Both `install otel` and `update otel` reach `generateOtelConfig(apiURL, token)` to produce the rendered config, so adding `Debug` to `otelConfigData` and setting it from `logger.IsDebug()` inside that function covers both commands automatically.

## Goals / Non-Goals

**Goals:**

- Emit a `debug` exporter (verbosity: normal) in the collector config when `--debug` is active.
- Apply to all pipelines: traces, metrics/apps, metrics/host, metrics (non-host-monitoring path), logs.
- Work for both `install otel` and `update otel` without additional wiring.

**Non-Goals:**

- A separate `--debug-exporter` flag.
- Runtime toggling of the debug exporter without reinstalling/updating.
- Per-pipeline selection of whether to include the debug exporter.

## Decisions

**Use `logger.IsDebug()` rather than threading the flag.**
`logger.IsDebug()` is already the established pattern inside the otel package for reacting to `--debug`. Adding a `debug bool` parameter to `generateOtelConfig` would require changes to callers and adds no value over reading the logger state that is already set before any installer code runs.

**`normal` verbosity over `basic` or `detailed`.**
`basic` only counts items. `detailed` dumps full data point values and is too noisy for general use. `normal` surfaces attribute names and values, which is the right level for checking whether expected attributes (e.g., `host.name`, `service.name`) are actually present in the pipeline.

**All pipelines, unconditionally.**
Selective pipeline inclusion would require more template complexity and more configuration surface. When debugging, you want visibility across the board. If the signal is too noisy, the user drops `--debug`.

## Risks / Trade-offs

- **Persistent debug config**: If the user installs with `--debug`, the written config file retains the debug exporter permanently. Subsequent runs without `--debug` (e.g., `update otel`) will remove it, since the config is regenerated. This is acceptable and consistent with how all other config fields work.
- **Log volume**: `normal` verbosity can be chatty on a busy host (e.g., with host metrics enabled). This is intentional: the user opted in via `--debug`.
