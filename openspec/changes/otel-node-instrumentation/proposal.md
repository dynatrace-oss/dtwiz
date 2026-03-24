## Why

Node.js is the next language on the OTel roadmap after Python and Java. No `otel-node` code exists yet. We need a full install/uninstall flow for Node.js auto-instrumentation, following the same patterns established by the Python implementation.

## What Changes

- Implement `dtwiz install otel-node` — detect Node.js projects, identify entrypoints, install OTel Node SDK packages, launch with `--require @opentelemetry/auto-instrumentations-node/register`, configure OTEL_* environment variables for Dynatrace export
- Implement `dtwiz uninstall otel-node` — stop instrumented processes, remove OTel Node packages
- Add pre-flight validation: Node.js in PATH, npm/yarn/pnpm availability
- Register CLI subcommands in `cmd/install.go` and `cmd/uninstall.go`
- Wire into collector app-type listing (depends on collector-improvements change)

## Capabilities

### New Capabilities
- `node-project-detection`: Detect Node.js projects by scanning for `package.json`, identify entrypoints from `main`/`scripts.start` fields or common filenames (`index.js`, `server.js`, `app.js`)
- `node-process-detection`: Detect running Node.js processes and correlate them to project directories
- `node-instrumentation`: Install OTel Node packages (`@opentelemetry/sdk-node`, `@opentelemetry/auto-instrumentations-node`, `@opentelemetry/exporter-trace-otlp-http`, etc.), launch with `--require` flag and OTEL_* env vars
- `node-uninstall`: Stop instrumented Node.js processes and remove OTel packages
- `node-install-validation`: Pre-flight checks — Node.js in PATH, package manager availability, OS compatibility

### Modified Capabilities

## Impact

- New file: `pkg/installer/otel_node.go` — full Node.js instrumentation implementation
- New file: `pkg/installer/otel_node_uninstall.go` — uninstall logic
- `cmd/install.go` — register `otel-node` subcommand
- `cmd/uninstall.go` — register `otel-node` subcommand
- `pkg/installer/otel_collector.go` — add Node to app-type detection (handled by collector-improvements change)
