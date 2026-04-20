# Proposal

## Why

The existing Node.js OTel stub in `otel_nodejs.go` only prints manual instructions — it does not install packages, create configuration, or launch instrumented processes. Python has a full end-to-end flow (detect → install → launch → verify in Dynatrace), but Node.js users must copy-paste commands manually. This violates the project's "if we detect it, we enable it" philosophy. Additionally, monorepos, Next.js projects, and package manager detection are not supported.

## What Changes

- Rewrite `otel_nodejs.go` to perform actual installation: detect Node.js projects (including monorepos, Next.js, and Nuxt), auto-detect the package manager, create a `.otel/` sibling directory with its own `package.json` for OTel deps, run `npm install`, launch the instrumented app via `node --require`, and poll Dynatrace until the service appears.
- Add `dtwiz install otel-node` as a standalone Cobra subcommand with `--service-name` and `--dry-run` flags.
- Extend `dtwiz uninstall otel` to also detect and clean up `.otel/` directories and instrumented Node.js processes.
- For Next.js and Nuxt projects, generate framework-specific wrapper scripts in `.otel/` that require the auto-instrumentation module before delegating to the framework CLI — no changes to user code, no `NODE_OPTIONS`.
- Update the `createRuntimePlan()` Node.js branch in `otel.go` to pass `envURL` and `platformToken`.

## Capabilities

### New Capabilities

- `nodejs-project-detection`: Detect Node.js projects by scanning for `package.json` across CWD and common dev directories. Identify monorepos via `workspaces` field. Detect Next.js via `next.config.*` or `next` in dependencies. Detect Nuxt via `nuxt.config.*` or `nuxt` in dependencies. Auto-detect package manager from lockfiles. Detect running Node.js processes and correlate to project directories.
- `nodejs-otel-instrumentation`: Sets `process.env.OTEL_*` vars. Create a `.otel/` directory inside the project with its own `package.json` and OTel deps. Install packages via `npm install` in `.otel/`. For regular apps, run `node --require @opentelemetry/auto-instrumentations-node/register <entrypoint>` from `.otel/`. For Next.js, generate `.otel/next-otel-bootstrap.js` and run `node otel/next-otel-bootstrap.js start`. For Nuxt, generate `.otel/nuxt-otel-bootstrap.js` and run `node otel/nuxt-otel-bootstrap.js start`. Manage processes, detect ports, poll Dynatrace Smartscape.
- `nodejs-otel-uninstall`: Extend `dtwiz uninstall otel` to find `.otel/` directories containing OTel packages and running instrumented Node.js processes. Preview removal, confirm, kill processes, delete `.otel/` directories.

### Modified Capabilities

- `otel-combined-install` (existing `install otel`): The Node.js branch in `createRuntimePlan()` updated to pass `envURL` and `platformToken` for Smartscape polling.

## Impact

- **Code**: Heavy rewrite of `pkg/installer/otel_nodejs.go`; modified `pkg/installer/otel_uninstall.go`, `pkg/installer/otel.go`, `cmd/install.go`.
- **Dependencies**: No new Go module dependencies. Uses `node` and `npm` CLI (already detected by existing `exec.LookPath` checks).
- **UX**: `install otel-node` provides a full automated flow. `uninstall otel` now also cleans Node.js instrumentation. Both support `--dry-run`.
- **Non-regression**: Existing `install otel` combined flow unchanged (Node.js stays behind `DTWIZ_ALL_RUNTIMES` gate). Existing `uninstall otel` collector cleanup unchanged — Node.js cleanup is additive.
