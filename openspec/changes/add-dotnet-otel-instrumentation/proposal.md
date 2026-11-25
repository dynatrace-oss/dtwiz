# Proposal

## Why

dtwiz auto-instruments Java, Python, Node.js and Go projects with OpenTelemetry, but a .NET project can only end in the manual-steps outcome ("see the Dynatrace walkthrough"). Dynatrace documents an "after deployment" auto-instrumentation path for .NET that needs no source or `.csproj` changes, which fits dtwiz's isolated, zero-config model. The work is large (detection, prerequisites, provisioning, launch, uninstall), so it is split across several changes. This change records the overall plan and specifies the first step in detail: finding .NET projects, their runnable services and their running processes, behind a feature flag.

## What Changes

- Add a general capability spec describing how .NET auto-instrumentation will work end to end, its five steps, and the decisions already made (isolated install in a dtwiz-owned directory, latest upstream release, env vars passed to the launched process only, nothing secret on disk). Steps 2-5 are planned there and get their own detailed specs in follow-up changes.
- Specify **step 1: .NET project detection**:
  - project discovery from `.sln`/`.slnx` and SDK-style project files under the existing scan scope;
  - service discovery: which projects are runnable (exe, web, worker) and which are not (libraries, test projects), with target framework and self-contained information read from the project file;
  - association of running .NET processes with detected projects, covering `dotnet run`, `dotnet <app>.dll` and native app-host executables, on Unix and Windows.
- Gate everything behind a new feature flag `DTWIZ_DOTNET_OTEL` (`--dotnet-otel`, default off). With the flag off, behavior is unchanged: .NET stays in the manual-languages list and `--project` pointing at a .NET directory is not inferred as a runtime.
- With the flag on, step 1 lists .NET projects (with running PIDs) in the project picker. Selecting one ends in the existing manual-steps outcome until the later steps ship; nothing is installed or changed on the machine.
- Detect, but do not yet instrument, .NET Framework projects, IIS (`w3wp.exe`) and Windows services; they are reported as not yet supported.
- **No breaking changes.**

## Capabilities

### New Capabilities

- `dotnet-otel-instrumentation`: The overall plan and cross-step rules for instrumenting .NET applications with OpenTelemetry auto-instrumentation: flag gating, the five steps and their status, isolated installation, latest-release download with verification, launch with environment variables only, and out-of-scope targets.
- `dotnet-project-detection`: Step 1. How dtwiz discovers .NET projects, splits a project into runnable services, associates running .NET processes with projects, and presents them in the project selection, including the manual-steps outcome until later steps exist.

### Modified Capabilities

- `feature-flag-registry`: Registers the `DotNetOtel` flag (`--dotnet-otel`, `DTWIZ_DOTNET_OTEL`, default off) alongside the existing flags.

## Impact

- **Flag**: `DTWIZ_DOTNET_OTEL` / `--dotnet-otel`, defined in `pkg/featureflags/featureflags.go`; shows up automatically in `dtwiz status` and `--help`.
- **Code (step 1, in follow-up implementation)**: a new self-contained subpackage `pkg/installer/otel/dotnet/` holding the .NET-specific logic (project file parsing, entrypoint discovery, process classification) with its own `Project`/`Service` types and plain-parameter inputs, importing nothing from `pkg/installer/otel/` (dependency points one way: `otel` imports `dotnet`); the parts that must talk to existing code stay in `pkg/installer/otel/`: process listing (Unix/Windows split stays there) and adapting `dotnet` results into `ScannedProject`; a small extension of project scanning in `pkg/installer/otel/runtime_scan.go` so markers can match by file suffix (project files have arbitrary names); `bin`/`obj` added to ignored directories; runtime registration and `--project` inference in `pkg/installer/otel/otel.go`; removal of `.NET` from the manual-languages list only when the flag is on and the flow ships end to end (later change).
- **Dependencies**: none new for step 1. Later steps add an HTTPS download of the upstream release archive and hash/attestation checks.
- **Docs**: flag description and, in later changes, user documentation for .NET.
- **Rollback**: the flag is off by default and step 1 changes nothing on the machine, so rollback is leaving the flag off or reverting the new files; the only shared-code changes are the suffix-aware marker matching and the extra ignored directories, both covered by existing scan tests.
- **Follow-ups (own specs later)**: prerequisites and plan (step 2), instrumentation provisioning (step 3), execute/launch (step 4), uninstall and watch (step 5); IIS, Windows services and .NET Framework instrumentation are a possible later change.
