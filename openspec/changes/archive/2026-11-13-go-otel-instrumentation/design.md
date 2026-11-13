# Design

## Context

`pkg/installer/otel/golang.go` today detects Go projects (`go.mod` scan) and prints manual steps; it is gated by `AllRuntimes`. Python, Java and Node.js plans already follow one shape: detect → pick project → build a plan → preview/confirm → execute → launch → watch. Node keeps its artifacts in `<project>/.otel/` and uninstall finds them via the `RuntimeCleaner` registry in `uninstall_runtime.go` plus a directory scan (`findNodeOtelDirs`). See proposal.md for motivation.

Facts verified while exploring (spike run on otelc v1.0.1, macOS/arm64, Go 1.26; dtwiz now installs `@latest` (v1.2.0 at the time of writing), so the spike results below must be re-verified on the current latest, task 1.5, and again by the scheduled smoke test, task 3.4):

- Install path is `go.opentelemetry.io/otelc/tool/cmd/otelc`; tags before v1.0.1 are not served by the Go proxy, so v1.0.1 is the first installable stable release. v1.0.1 declares `go 1.25.0`; v1.2.0 (12 Oct, latest at the time of writing) declares `go 1.26.0`. The minimum Go version therefore follows whatever `@latest` resolves to and is not a dtwiz constant.
- `otelc --work-dir <dir> go build -o <bin> .` works. It printed "Bumped go version" yet `go.mod` was identical afterwards and no `go.sum` was created. A project with an existing `go.sum` and an interrupted build were not tested.
- A built binary emits a SERVER span for `net/http` from `OTEL_*` environment variables alone.
- Upstream CI runs on Linux, macOS and Windows; dtwiz has not run it on Linux or Windows yet.
- Workspace spike, used by decision 13 (`go.work` with modules a, b, c; a imports b only through the workspace, no `replace`; macOS, Go 1.26.2):
  - Workspace mode on, `go.work` and modules at `go 1.22`: **fails** for every module with `module . listed in go.work file requires go >= 1.25.0, but go.work lists go 1.22` (otelc v1.0.1 raises the module's `go` directive, the workspace rejects it; with newer otelc versions the message and threshold will differ, re-verify).
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

- Workspace modules that depend on a workspace sibling only through `go.work`: they get manual steps with the reason (decision 13); making them build would need `replace` directives or editing `go.work`.
- Persisting instrumentation into the user's own build or deploy pipeline (dtwiz launches its own binary; the preview prints the equivalent otelc command as a hint only).
- Instrumenting services that already configure their own OTel SDK.
- eBPF / OBI based instrumentation, and Go versions too old for the latest otelc for compile-time instrumentation (such users are told which Go version to upgrade to and get the client-only fallback).
- Removing OTel modules downloaded into the shared Go module cache.

## Decisions

### 1. Per-service tier selection, not per-plan

`GoInstrumentationPlan` holds a list of services, each with a chosen method (`otelc`, `client-only`, `manual`). Tier 1 requires a Go toolchain new enough for the latest otelc (decision 2), a usable otelc, and, for workspace projects, a passing isolated pre-flight for that service (decision 13). If otelc install fails, all services start at tier 2; if one `otelc go build` fails, only that service is rebuilt at tier 2; if tier 2 also fails, only that service gets manual steps.
_Alternative:_ one method for the whole plan. Rejected: one cgo or odd service would downgrade or abort the rest.

### 1a. Manual steps are a plan, not an absence of one

The shared install flow treats a runtime plan that returns nothing as "this project can't be auto-instrumented" and asks to select another project. For Go, the manual outcome (workspace, no toolchain, both methods failed) is instead a real plan whose method is `manual`: the preview shows the collector section plus a Go section that says why nothing is automatic; after confirmation the collector is installed as usual, and `Execute()` only prints the manual steps (packages, env vars pointing at the local collector, walkthrough link). The existing stub's output is the basis. Exit status is success unless collector installation fails, or automatic instrumentation was tried (builds or launch) and no service ended up instrumented, which returns a typed error so the failed self-monitoring event carries its category. A project that is manual from the start (for example no toolchain) still exits successfully. A plan returns nothing only when there is nothing to say (no `package main`).
_Alternative:_ return no plan and let the generic flow handle it. Rejected: it offers "Select another project?" and drops the instructions.

### 2. otelc installed at `@latest`, no version constant

dtwiz installs `go.opentelemetry.io/otelc/tool/cmd/otelc@latest`, so users always get the current fixes without dtwiz needing a release or a bump. There is no version constant and no hard-coded Go gate: the minimum Go version is whatever the latest otelc declares (1.26 for v1.2.0). The install runs with `GOTOOLCHAIN=local` so Go never silently downloads a different toolchain. If `go install` fails because the local Go is too old (`requires go >= X`), dtwiz classifies the error, tells the user "upgrade Go to X or newer for compile-time instrumentation" using the version Go reported, and plans the client-only fallback (decision 1). Go supports only its two newest releases, so asking users to move to a supported Go is reasonable. After installing, dtwiz logs the resolved otelc version (`logger.Debug`) and shows it in the preview so a failure report names the exact version.

Because the version is no longer fixed, spike results can go stale when upstream releases. Mitigations: tier-1 failures degrade per service to tier 2 or manual steps (decision 1), and a scheduled CI smoke test builds a tiny module with the current `@latest` otelc and fails when it breaks (task 3.4), so regressions surface in CI instead of at users.

_Alternative:_ an exact pin in one constant (`otelcVersion`) with a bump check. Rejected by the team: it needs ongoing bump maintenance (Dependabot cannot see a constant in Go source, so it would need a custom CI job or Renovate rule) and keeps users on older tool versions between bumps. Trade-off accepted: builds are not reproducible across time, and a compromised or broken upstream release would be installed on the next run; the smoke test and the per-service fallback limit the impact.

### 3. Isolated tool directory

dtwiz installs otelc itself when it is missing. If a usable otelc is already on PATH it is reused (it is the user's own and is never removed). Otherwise dtwiz runs `go install` with `GOBIN` set to `~/.opentelemetry/go/bin` (via `os.UserHomeDir()`), so nothing lands in the user's `GOPATH/bin` and an existing otelc is never overwritten. Only the copy dtwiz installed is removed on uninstall.

### 4. Artifacts live under `<module>/.otel/go/`

Node already owns `<dir>/.otel/` (with `package.json`). A Go module that also has a Node frontend in the same directory would collide if both used `.otel/` directly. Go uses the subdirectory `.otel/go/` containing `bin/<service>[.exe]`, tier-2 files (`overlay.json`, generated init, `go.mod`/`go.sum` copies), `modguard/` (only during/after an interrupted build) and a small marker file written last. Uninstall recognizes Go directories by that marker, removes only `.otel/go/`, then removes `.otel/` if it is empty.
_Alternative:_ reuse `.otel/` with a marker. Rejected for the polyglot case; the cost of the subdirectory is zero.

### 5. `go.mod`/`go.sum` guard (option A)

otelc rejects `-modfile`, so tier 1 builds against the real module files. `modGuard` copies `go.mod`/`go.sum` to `.otel/go/modguard/` and restores them atomically afterwards, in a deferred function that also runs on SIGINT/SIGTERM (signal context). Safeguards:

- Uncommitted changes to `go.mod`/`go.sum` are not a special case: the backup is a copy of the files as they are, so restoring returns them to exactly that state. No git check is needed, and developers with a modified `go.mod` keep tier 1.
- A leftover backup that differs from the current files stops the run for that module with the backup path; an identical one is deleted as stale.
- Restore retries briefly (Windows can hold files open); failures are reported with the backup path and never swallowed.

Spike result: v1.0.1 left `go.mod` unchanged in a trivial case (to be re-checked on the current latest), so the guard may rarely act, but it is cheap insurance against kills and other otelc versions.
_Alternative B:_ build in a temp copy of the module. Rejected: slow on large repos, breaks relative `replace` and vendored trees, and hides resolution differences.

### 6. Tier 2 (client-only) via build overlay and isolated modfile

Go's `-overlay` injects a generated `init()` file into each `package main` at build time, and `-modfile=.otel/go/go.mod` keeps the added requirements out of the project's `go.mod`. The init configures traces, metrics, logs from `OTEL_*` variables (per the Dynatrace Go walkthrough, HTTP OTLP, delta temporality) and wraps `http.DefaultTransport` for outgoing HTTP. It requires `GOWORK=off` because `-modfile` cannot be combined with workspace mode, and it is the only tier that adds modules (into the isolated copy). Incoming requests are not traced; the user is told so.
Only attempted when tier 1 is unavailable for that service, and not used at all if the module already depends on the OTel SDK (decision 8).

### 7. Service discovery and naming

A `package main` walker: depth 5, skip ignored dirs/`testdata`/nested modules, within the selected module and, for a project with a `go.work`, within each workspace module listed in it that lives inside the project (decision 13). Each service records its module directory, used for the build, `.otel/go/` and the modguard backup. Names: directory base name; project folder for the root; relative path (normalized) on collision. Own file `golang_entrypoints.go` with a table-driven test.

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

### 13. Go workspaces (`go.work`): build each workspace module in isolation

The workspace spike (see Context) showed that workspace mode cannot be used with otelc (the bumped `go` directive collides with `go.work`), but that `GOWORK=off` builds a module that does not depend on workspace siblings through `go.work` alone. Many workspaces are exactly that shape: the motivating case, `go-pher-order-plain`, has a root module containing only `doc.go` and six independent service modules. A first version that sent every workspace to manual steps (and, in the implemented flow, reported "No Go service found" before reaching that path) left such projects uninstrumented although they build fine. The reference implementation is the `feat/go-instrumentation-poc` branch (`goWorkspaceModules`, `detectGoMainPackages`, `otelcBuildCmd`, `modGuard`).

- **Discovery:** the project's services are the `package main` directories of the root module plus those of every module listed in the root `go.work` `use` directives (single-line and block forms, comments ignored) that lives inside the project. `use` entries outside the project are ignored. Nested modules not listed in `go.work` stay separate projects (decision 7). The "no service found" check runs only after this expansion, so a workspace root without its own `package main` is not a dead end.
- **Build:** every service builds from its own module directory with `GOWORK=off`. Each module has its own `.otel/go/` (binaries, tier-2 files, marker) and its own modguard backup of its own `go.mod`/`go.sum`, since modules have different paths and pinned dependencies.
- **Pre-flight:** before building a service in a workspace project, `go list -deps ./<pkgdir>` runs in its module with `GOWORK=off`. When it fails (typically a sibling module required only through `go.work`, "missing go.sum entry" or an unresolvable module), that service is not built: it gets manual steps (decision 1a) with the reason stated, and the other services continue. Neither tier is attempted for it, because tier 2 shares the `GOWORK=off` constraint and would fail the same way.
- **Workspace membership:** a project is a workspace project when a `go.work` exists in its directory or any parent directory (the lookup Go itself performs). A selected module below a `go.work` is built alone under the same rules (its own `go.mod`, `GOWORK=off`, pre-flight); the parent's other modules are not touched. Builds for all projects set `GOWORK=off` so an inherited `GOWORK` cannot interfere.

_Alternatives:_ (a) workspace mode with otelc — rejected, the bumped `go` directive collides with `go.work` and sibling imports are fetched from the network; (b) edit `go.work` or add `replace` directives so siblings resolve — rejected, it changes user files beyond a restorable backup; (c) keep workspaces as manual-only — rejected, it blocks the common independent-modules layout.

### 14. Separate `GoInstrumentation` feature flag

A new flag is registered in `pkg/featureflags/` next to `AllRuntimes` and `Experimental`: CLI `--go-instrumentation`, env `DTWIZ_GO_INSTRUMENTATION`, default off. It gates the whole new Go flow: detection as an auto-instrumentable runtime, tier selection, build/launch, the notice entry, and the manual-languages change. With it off, today's behavior (including the existing `AllRuntimes` stub) is untouched. Uninstall cleanup is gated too: with the flag off, `dtwiz uninstall otel` does not scan for, list or remove Go artifacts and behaves as today.
_Alternative:_ reuse `AllRuntimes`. Rejected: it enables unrelated runtimes and cannot be removed for Go alone.

### 15. otelc work directory is shared per module for the length of a run

`otelc` keeps its analysis and generated files in `--work-dir`. A fresh directory per build makes every build start from scratch: measured on otelc v1.2.0 with a tiny service, about 30 s per service against about 5 s for a second service built with the same directory (a plain `go build` takes about 1 s). dtwiz therefore creates one temporary work directory per module on first use, reuses it for the module's other services, discards it after a failed build so leftovers cannot affect the next service, and removes all of them when the run ends. Nothing is kept between runs and nothing is written into the project, so the first build of every module stays slow and there is no stale-state risk across runs or otelc versions. The work directory reached about 290 MB for one tiny service, which is why it lives in the OS temp directory and is not kept.
_Alternatives:_ (a) a persistent directory in `<module>/.otel/go/` — rejected for now, it adds hundreds of MB per module to the customer's project and relies on otelc's reuse logic across runs and versions; (b) a persistent directory under `~/.opentelemetry/go/` — deferred, it is shared across projects and needs concurrency and cleanup rules; (c) a fresh directory per build — rejected, it is the slow baseline.

## Risks / Trade-offs

- **otelc rewrites module files** → guard + leftover-backup check; documented limitation that an uncatchable kill (SIGKILL, power loss) relies on the leftover check at next run.
- **First build downloads many modules and is slow; needs network** → preview says so; failure messages classify connectivity errors; fallback or manual steps follow.
- **otelc behavior differs on Linux/Windows** → first task is a spike on all three OSes; the plan degrades per service if tier 1 fails.
- **Turning the flag off after an install leaves Go artifacts in place** → the user must re-enable `GoInstrumentation` to remove them with `dtwiz uninstall otel`.
- **Workspace modules that import siblings only through `go.work` cannot build in isolation** → the per-module `GOWORK=off` pre-flight detects this before building, the service gets manual steps with the reason, and independent modules in the same workspace are still instrumented.
- **Instrumented binary only exists in `.otel/`** → users' own builds are unchanged; the preview prints the equivalent `otelc go build` command and uninstall explains what is removed.
- **`@latest` can change behavior, break, or ship a bad release without a dtwiz release** → scheduled CI smoke test against the current latest, per-service degradation to tier 2/manual, resolved version shown in the preview and logs.
- **The latest otelc may raise its Go minimum at any time** → install runs with `GOTOOLCHAIN=local`; users on older Go get an explicit "upgrade Go to X" message (X taken from Go's own error) and the client-only fallback.
- **Tier 2 wraps the default HTTP transport and slog default** → limited to the fallback, listed in the preview as "outgoing HTTP only".
- **Process detection by path is a heuristic on some platforms** → matches only executables under `.otel/go/bin/`, never plain `go run` processes, so false positives cannot kill the user's own processes.

## Migration Plan

Additive. Ship behind the new `GoInstrumentation` feature flag (off by default); with it off nothing changes for users. Removing the gate is a later, separate change once the Linux/Windows runs are done. Rollback: `dtwiz uninstall otel` removes every generated artifact; reverting the change restores the manual-steps output. No data migration.

## Open Questions

- Whether to print the equivalent `otelc go build` command for users' CI (wording only; does not change specs).
- Exact name and content of the marker file in `.otel/go/` (implementation detail; behavior is fixed by the uninstall spec).
