## Context

No OTel Node.js code exists in the project. Node.js auto-instrumentation uses `@opentelemetry/auto-instrumentations-node` — a package that, when required at startup via `--require @opentelemetry/auto-instrumentations-node/register`, automatically instruments supported libraries (Express, HTTP, gRPC, database clients, etc.) without code changes. OTEL_* environment variables configure the exporter endpoint and protocol.

The Python implementation provides the reference pattern: project detection → process detection → user selection → package installation → instrumented launch → Dynatrace verification.

## Goals / Non-Goals

**Goals:**
- Detect Node.js projects and running processes
- Install OTel Node packages into the selected project
- Launch with `--require` flag and OTEL_* env vars
- Implement uninstall
- Add pre-flight validation

**Non-Goals:**
- Supporting TypeScript compilation workflows
- Supporting Deno or Bun runtimes
- Multi-project instrumentation (single-app only)
- Framework-specific configuration (e.g., Next.js custom server setup)

## Decisions

**1. Project detection via `package.json`**
Scan working directory and common development directories for `package.json` files. Extract project name from `name` field, entrypoint from `main` or `scripts.start`. Also check for common filenames: `index.js`, `server.js`, `app.js`, `main.js`.

**2. Package installation via detected package manager**
Detect which package manager the project uses:
- `package-lock.json` → npm
- `yarn.lock` → yarn
- `pnpm-lock.yaml` → pnpm
- Fallback → npm

Install packages: `@opentelemetry/sdk-node`, `@opentelemetry/auto-instrumentations-node`, `@opentelemetry/exporter-trace-otlp-http`, `@opentelemetry/exporter-metrics-otlp-http`, `@opentelemetry/exporter-logs-otlp-http`.

Alternative: Always use npm — rejected because it would conflict with projects using yarn/pnpm lockfiles.

**3. Launch via `node --require` flag**
Run `node --require @opentelemetry/auto-instrumentations-node/register <entrypoint>` with OTEL_* env vars. This is the officially recommended zero-code approach.

Alternative: Programmatic SDK setup — rejected because it requires code changes, violating zero-config principle.

**4. Uninstall removes packages and stops processes**
Find running Node processes with `@opentelemetry/auto-instrumentations-node` in their command. Stop them. Run `npm uninstall` (or equivalent) for the OTel packages.

## Risks / Trade-offs

- [Package manager detection may fail for monorepos] → Use the `package.json` closest to the selected entrypoint.
- [Some Node frameworks (e.g., Next.js) don't support `--require`] → Document as a known limitation. These frameworks need programmatic setup.
- [`--require` flag may interact with other require hooks (ts-node, etc.)] → Accept this; the `--require` approach is the standard OTel recommendation.
