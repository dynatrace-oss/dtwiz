# Design

## Context

`pkg/installer/otel/golang.go` today detects Go projects (`go.mod` scan) and prints manual steps; it is gated by `AllRuntimes`. Python, Java and Node.js plans already follow one shape: detect → pick project → build a plan → preview/confirm → execute → launch → watch. Node keeps its artifacts in `<project>/.otel/` and uninstall finds them via the `RuntimeCleaner` registry in `uninstall_runtime.go` plus a directory scan (`findNodeOtelDirs`). See proposal.md for motivation.

Facts verified while exploring (otelc v1.0.1, macOS/arm64, Go 1.26):

- Install path is `go.opentelemetry.io/otelc/tool/cmd/otelc`; tags before v1.0.1 are not served by the Go proxy, so v1.0.1 is the first installable stable release. v1.0.1 declares `go 1.25.0`; v1.2.0 (latest) declares `go 1.26.0`, so `@latest` would need Go 1.26.
- `otelc --work-dir <dir> go build -o <bin> .` works. It printed "Bumped go version" yet `go.mod` was identical afterwards and no `go.sum` was created. A project with an existing `go.sum` and an interrupted build were not tested.
- A built binary emits a SERVER span for `net/http` from `OTEL_*` environment variables alone.
- Upstream CI runs on Linux, macOS and Windows; dtwiz has not run it on Linux or Windows yet.
- Workspace spike (`go.work` with modules a, b, c; a imports b only through the workspace, no `replace`; macOS, Go 1.26.2):
  - Workspace mode on, `go.work` and modules at `go 1.22`: **fails** for every module with `module . listed in go.work file requires go >= 1.25.0, but go.work lists go 1.22` (otelc raises the module's `go` directive, the workspace rejects it).
  - Workspace mode on, everything already at `go 1.25.0`: a module with no sibling dependency **builds**; a module importing a sibling through `go.work` only **fails** (otelc tries to download the sibling from the network).
  - `GOWORK=off`, module with no sibling dependency (even at `go 1.22`): **builds**; `go.mod` is bumped during the build and identical afterwards, no `go.sum` created.
  - `GOWORK=off`, module importing a sibling through `go.work` only: **fails**, and so does plain `go build` with `GOWORK=off` (missing go.sum entry). Adding `replace example.com/ws/b => ../b` makes it **build**.
  - One run of a module that builds in isolation failed with an empty error and passed on rerun; not reproduced, treated as a flake to watch for in the integration test.

## Goals / Non-Goals

**Goals:**

- One command from a Go project to telemetry in Dynatrace, with no source change and no lasting `go.mod`/`go.sum` change.
- Graceful, per-service degradation: otelc → client-only → manual steps.
- Complete, safe uninstall; identical behavior on Linux, macOS, Windows.

**Non-Goals:**

- Go workspaces (`go.work`) in this version: such projects get manual steps (decision 13).
- Persisting instrumentation into the user's own build or deploy pipeline (dtwiz launches its own binary; the preview prints the equivalent otelc command as a hint only).
- Instrumenting services that already configure their own OTel SDK.
- eBPF / OBI based instrumentation, and Go versions below 1.25 for compile-time instrumentation.
- Removing OTel modules downloaded into the shared Go module cache.

## Decisions

### 1. Per-service tier selection, not per-plan

`GoInstrumentationPlan` holds a list of services, each with a chosen method (`otelc`, `client-only`, `manual`). Tier 1 requires Go ≥ 1.25, a usable otelc, and and a project outside any Go workspace (decision 13). If otelc install fails, all services start at tier 2; if one `otelc go build` fails, only that service is rebuilt at tier 2; if tier 2 also fails, only that service gets manual steps.
*Alternative:* one method for the whole plan. Rejected: one cgo or odd service would downgrade or abort the rest.

### 1a. Manual steps are a plan, not an absence of one

The shared install flow treats a runtime plan that returns nothing as "this project can't be auto-instrumented" and asks to select another project. For Go, the manual outcome (workspace, no toolchain, both methods failed) is instead a real plan whose method is `manual`: the preview shows the collector section plus a Go section that says why nothing is automatic; after confirmation the collector is installed as usual, and `Execute()` only prints the manual steps (packages, env vars pointing at the local collector, walkthrough link). The existing stub's output is the basis. Exit status is success unless collector installation fails. A plan returns nothing only when there is nothing to say (no `package main`).
*Alternative:* return no plan and let the generic flow handle it. Rejected: it offers "Select another project?" and drops the instructions.

### 2. otelc pinned to v1.0.1, one constant

`otelcVersion = "v1.0.1"` is the only place the version appears; install path is `…/otelc/tool/cmd/otelc@<version>`. The Go gate is the pinned version's own minimum (1.25). Bumping is a deliberate change that re-runs the spike.
*Alternative:* `@latest` (currently v1.2.0, which needs Go 1.26). Rejected: it would exclude Go 1.25 users from compile-time instrumentation, it is untested by the spike, and we have no adoption data showing 1.26 is common enough. Revisit with the next version bump.

### 3. Isolated tool directory

dtwiz installs otelc itself when it is missing. If a usable otelc is already on PATH it is reused (it is the user's own and is never removed). Otherwise dtwiz runs `go install` with `GOBIN` set to `~/.opentelemetry/go/bin` (via `os.UserHomeDir()`), so nothing lands in the user's `GOPATH/bin` and an existing otelc is never overwritten. Only the copy dtwiz installed is removed on uninstall.

### 4. Artifacts live under `<module>/.otel/go/`

Node already owns `<dir>/.otel/` (with `package.json`). A Go module that also has a Node frontend in the same directory would collide if both used `.otel/` directly. Go uses the subdirectory `.otel/go/` containing `bin/<service>[.exe]`, tier-2 files (`overlay.json`, generated init, `go.mod`/`go.sum` copies), `modguard/` (only during/after an interrupted build) and a small marker file written last. Uninstall recognizes Go directories by that marker, removes only `.otel/go/`, then removes `.otel/` if it is empty.
*Alternative:* reuse `.otel/` with a marker. Rejected for the polyglot case; the cost of the subdirectory is zero.

### 5. `go.mod`/`go.sum` guard (option A)

otelc rejects `-modfile`, so tier 1 builds against the real module files. `modGuard` copies `go.mod`/`go.sum` to `.otel/go/modguard/` and restores them atomically afterwards, in a deferred function that also runs on SIGINT/SIGTERM (signal context). Safeguards:

- Uncommitted changes to `go.mod`/`go.sum` are not a special case: the backup is a copy of the files as they are, so restoring returns them to exactly that state. No git check is needed, and developers with a modified `go.mod` keep tier 1.
- A leftover backup that differs from the current files stops the run for that module with the backup path; an identical one is deleted as stale.
- Restore retries briefly (Windows can hold files open); failures are reported with the backup path and never swallowed.

Spike result: v1.0.1 left `go.mod` unchanged in a trivial case, so the guard may rarely act, but it is cheap insurance against kills and other otelc versions.
*Alternative B:* build in a temp copy of the module. Rejected: slow on large repos, breaks relative `replace` and vendored trees, and hides resolution differences.

### 6. Tier 2 (client-only) via build overlay and isolated modfile

Go's `-overlay` injects a generated `init()` file into each `package main` at build time, and `-modfile=.otel/go/go.mod` keeps the added requirements out of the project's `go.mod`. The init configures traces, metrics, logs from `OTEL_*` variables (per the Dynatrace Go walkthrough, HTTP OTLP, delta temporality) and wraps `http.DefaultTransport` for outgoing HTTP. It requires `GOWORK=off` because `-modfile` cannot be combined with workspace mode, and it is the only tier that adds modules (into the isolated copy). Incoming requests are not traced; the user is told so.
Only attempted when tier 1 is unavailable for that service, and not used at all if the module already depends on the OTel SDK (decision 8).

### 7. Service discovery and naming

A `package main` walker: depth 5, skip ignored dirs/`testdata`/nested modules, within the selected module only. Names: directory base name; project folder for the root; relative path (normalized) on collision. Own file `golang_entrypoints.go` with a table-driven test.

### 8. Skip modules that already set up OTel

If a module directly requires `go.opentelemetry.io/otel/sdk*` or `…/exporters/*`, its services are skipped with a message and the collector endpoints. Injecting a second pipeline would conflict, and tier 1 would not be applicable.

### 9. Process model: launch with absolute path, detect by executable location

Services are launched as `<module>/.otel/go/bin/<name>` by absolute path, with the module as working directory. Because the command line then starts with that path, uninstall (and re-run) can identify dtwiz-launched services by matching the executable path under a `.otel/go/bin/` segment — Unix via the process list, Windows via the executable path from WMI in a `_windows.go` file. `detectProcesses("go run", …)` alone is not enough: it misses compiled binaries, so project–process matching also considers processes whose working directory is inside the project and whose command is a Go binary. Registering `goCleaner` in `runtimeCleaners` plugs Go into the existing uninstall preview/stop flow.

### 10. Cross-platform rules

`filepath.Join` everywhere; `.exe` suffix on Windows; build-tagged files for process detection and signal handling (`syscall.SIGTERM` is not delivered on Windows, so the leftover-backup check carries that case); stop processes before rebuilding or deleting (a running `.exe` is locked) and use the existing `removeWithRetry`; overlay JSON written via `encoding/json` so Windows paths are escaped. Pure logic is unit-tested on all OSes; the build is verified on Linux, macOS and Windows in CI (see tasks).

### 11. Environment for launched services

Reuse `environment.GenerateBaseOtelEnvVars(collectorEndpoint, serviceName)` with the service name overridden per service. Delta metric temporality must be present in that set (verify; add if missing — Dynatrace requires it). No Dynatrace token reaches the application; the local collector forwards.

### 12. Code layout

Following the one-concern-per-file rule, under `pkg/installer/otel/`: `golang.go` (detection, plan, selection), `golang_entrypoints.go`, `golang_otelc.go` (tool install, version gate, tier-1 build), `golang_fallback.go` (overlay/init, tier-2 build) with the init source in its own file, `golang_modguard.go`, `golang_launch.go`, `uninstall_go.go` (+ `_unix`/`_windows` process detection), each with a matching `_test.go`. `otel.go` registers Go as an enabled runtime, infers Go from `go.mod`, and drops Go from the manual-language list.

### 13. Go workspaces (`go.work`) are out of scope for the first version

The workspace spike (see Context) showed that workspace mode cannot be used with otelc (the bumped `go` directive collides with `go.work`), and that building with `GOWORK=off` only works for modules that do not depend on workspace siblings through `go.work` alone. Supporting workspaces correctly would need dependency analysis or a pre-flight check per module, which is more than the first version needs. Instead, a project is treated as a workspace project when a `go.work` file exists in its directory or any parent directory (the lookup Go itself performs). Such a project is still listed, but selecting it yields the manual plan (decision 1a): the collector is installed, the explanation and manual steps are printed, and nothing is built. Builds for supported projects still set `GOWORK=off` so an inherited `GOWORK` environment variable cannot interfere.
*Alternatives:* (a) support workspace modules that build in isolation with a `GOWORK=off` pre-flight (`go list`) — deferred, and the spike results are the starting point; (b) edit `go.work` or add `replace` directives — rejected, it changes user files beyond a restorable backup.

### 14. Separate `GoInstrumentation` feature flag

A new flag is registered in `pkg/featureflags/` next to `AllRuntimes` and `Experimental`: CLI `--go-instrumentation`, env `DTWIZ_GO_INSTRUMENTATION`, default off. It gates the whole new Go flow: detection as an auto-instrumentable runtime, tier selection, build/launch, the notice entry, and the manual-languages change. With it off, today's behavior (including the existing `AllRuntimes` stub) is untouched. Uninstall cleanup is gated too: with the flag off, `dtwiz uninstall otel` does not scan for, list or remove Go artifacts and behaves as today.
*Alternative:* reuse `AllRuntimes`. Rejected: it enables unrelated runtimes and cannot be removed for Go alone.

## Risks / Trade-offs

- **otelc rewrites module files** → guard + leftover-backup check; documented limitation that an uncatchable kill (SIGKILL, power loss) relies on the leftover check at next run.
- **First build downloads many modules and is slow; needs network** → preview says so; failure messages classify connectivity errors; fallback or manual steps follow.
- **otelc behavior differs on Linux/Windows** → first task is a spike on all three OSes; the plan degrades per service if tier 1 fails.
- **Turning the flag off after an install leaves Go artifacts in place** → the user must re-enable `GoInstrumentation` to remove them with `dtwiz uninstall otel`.
- **Users with workspaces get manual steps only** → explicit message and walkthrough link; workspace support is a candidate follow-up with the spike results as input.
- **Instrumented binary only exists in `.otel/`** → users' own builds are unchanged; the preview prints the equivalent `otelc go build` command and uninstall explains what is removed.
- **Pinned version ages** → single constant; a periodic bump task, with the spike as the acceptance test.
- **Tier 2 wraps the default HTTP transport and slog default** → limited to the fallback, listed in the preview as "outgoing HTTP only".
- **Process detection by path is a heuristic on some platforms** → matches only executables under `.otel/go/bin/`, never plain `go run` processes, so false positives cannot kill the user's own processes.

## Migration Plan

Additive. Ship behind the new `GoInstrumentation` feature flag (off by default); with it off nothing changes for users. Removing the gate is a later, separate change once the Linux/Windows runs are done. Rollback: `dtwiz uninstall otel` removes every generated artifact; reverting the change restores the manual-steps output. No data migration.

## Open Questions

- Whether to print the equivalent `otelc go build` command for users' CI (wording only; does not change specs).
- Exact name and content of the marker file in `.otel/go/` (implementation detail; behavior is fixed by the uninstall spec).
