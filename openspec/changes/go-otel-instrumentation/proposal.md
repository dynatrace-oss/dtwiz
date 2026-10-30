# Proposal

## Why

`dtwiz install otel` auto-instruments Python, Java and Node.js projects, but a detected Go project only gets printed `go get` lines and a "initialize the SDK in main()" hint, behind the `AllRuntimes` experimental flag. Go users have to install packages and edit code themselves, which contradicts dtwiz's zero-config principle. OpenTelemetry now ships a stable compile-time instrumentation tool (`otelc`, v1.0+) that instruments a Go binary at build time with no source changes, so a fully automatic path is possible.

## What Changes

- `dtwiz install otel` (and `dtwiz setup`) detects Go projects, matches them to running Go processes, lets the user pick one, finds every `package main` directory (up to 5 levels deep inside the module), names each service after its folder, and after one confirmation builds and launches each service with Dynatrace-ready `OTEL_*` settings.
- Instrumentation is chosen **per service**, in this order:
  1. **Compile-time (otelc)** — server and client spans, library coverage, no source or `go.mod` change left behind. Requires Go ≥ 1.25; otelc is pinned to a stable release (v1.0.1).
  2. **Client-only fallback** — an SDK bootstrap injected at build time (traces/metrics/logs, outgoing HTTP spans only) when otelc is unavailable or its build fails for that service.
  3. **Manual steps** — the OTel Collector is still installed, and instructions are printed when neither can be applied (no Go toolchain, build failures in both tiers, module already uses its own OTel SDK).
- Everything dtwiz generates lives in a per-module `.otel/` directory; project sources are never modified, and `go.mod`/`go.sum` are backed up before an otelc build and restored afterwards.
- Go workspaces (`go.work`) are **not supported in this first version**: a project inside a workspace is detected, and dtwiz still installs the OTel Collector and prints manual Go instrumentation steps instead of building it.
- Linux, macOS and Windows are supported.
- `dtwiz uninstall otel` also detects and removes Go instrumentation added by dtwiz: stops its running services, deletes the generated `.otel/` directories and the isolated otelc copy dtwiz installed.
- Automatic Go instrumentation is gated behind a new, separate feature flag `GoInstrumentation` (`--go-instrumentation` / `DTWIZ_GO_INSTRUMENTATION`, off by default), independent of `AllRuntimes`. With the flag off, Go behaves exactly as today (including the existing `AllRuntimes` behavior and manual-languages menu). With it on, Go is detected, auto-instrumented, and removed from the "manual languages" menu. The Go part of `dtwiz uninstall otel` is behind the same flag: with it off, uninstall does not look for or remove Go artifacts. Removing the gate is a later, separate change.

Rollback: uninstall removes all generated artifacts; because sources and `go.mod`/`go.sum` are left unchanged, no revert of user code is needed. The existing manual-steps output remains as tier 3.

## Capabilities

### New Capabilities

- `go-project-detection`: how Go projects are discovered, matched to running processes, and split into services (`package main` directories, service naming, `go.work` workspaces recognized).
- `go-otel-instrumentation`: how a selected Go project is instrumented, built and launched — tier selection per service, plan preview, build isolation and `go.mod` protection, Dynatrace-ready environment, cross-platform behavior, failure fallbacks.
- `go-otel-uninstall`: how `dtwiz uninstall otel` finds and removes Go instrumentation artifacts and processes.

### Modified Capabilities

- `otel-collector-install`: the requirement "Go instrumentation guidance uses the local collector endpoint" changes — the local collector endpoint now applies to the environment of launched Go services and to the manual-steps fallback.
- `otel-project-scan`: the supported-runtimes notice must list Go among the auto-instrumented runtimes.

## Impact

- Code: `pkg/installer/otel/` (`golang.go` and new Go-specific files for entrypoints, otelc handling, fallback init, `go.mod` guard, uninstall cleaner), `pkg/installer/otel/otel.go` (runtime registration, project inference, manual-language list), `pkg/installer/otel/uninstall_runtime.go` (register a Go cleaner), platform-specific process detection files.
- Feature flags: new `GoInstrumentation` flag registered in `pkg/featureflags/` and applied via `ApplyCLIOverrides`; `AllRuntimes` no longer affects the new Go flow.
- Dependencies at runtime (not dtwiz's own `go.mod`): `otelc` v1.0.1 installed into `~/.opentelemetry/go/bin`; OTel Go modules fetched into the Go module cache during builds. Network access is needed during the first build.
- Docs: `CHANGELOG.md`, user-facing docs listing supported runtimes.
- Tests: unit tests per new file, plus cross-platform build verification on Linux, macOS and Windows.
