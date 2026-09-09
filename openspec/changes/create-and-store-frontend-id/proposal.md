# Proposal: Create and Store Frontend ID

## Why

Agentless RUM monitoring requires a "frontend" object on the Dynatrace tenant to route telemetry. Creating this object and persisting its ID locally is the prerequisite for all subsequent agentless RUM steps (fetching the JS snippet, injecting it into the project). Neither the frontend creation capability nor a project-local config file currently exist in dtwiz.

## What Changes

- New `pkg/config/` package for reading and writing a per-project YAML config file at `{project-dir}/.dtwiz/config.yaml`, keyed by environment URL inside the file.
- New `pkg/installer/otel/internal/rum/` package with an `EnsureFrontendApplication` function that creates a `WEB_AGENTLESS` frontend on the Dynatrace Platform API and saves the returned ID to the project config (idempotent: reuses an existing ID if one is already stored for the current environment). The `internal/` path restricts its use to the otel installer subtree.

## Capabilities

### New Capabilities

- `project-config`: Read/write persistent project-local configuration in `.dtwiz/config.yaml`, keyed by environment URL. General-purpose — not RUM-specific.
- `rum-frontend`: Create a `WEB_AGENTLESS` frontend on the Dynatrace RUM API and cache its ID in the project config. Exposes `EnsureFrontendApplication` via `pkg/installer/otel/internal/rum/`, accessible only within the otel installer subtree.

### Modified Capabilities

## Impact

- New package `pkg/config/` — no existing code touched.
- New file(s) in `pkg/installer/otel/` for RUM frontend logic — no other existing code touched.
- Dependency already present: `gopkg.in/yaml.v3` (used by the otel installer).
- No new CLI commands or flags.
- No changes to existing installers in this change (wire-up into `install otel` / `update otel` is a follow-on change).
