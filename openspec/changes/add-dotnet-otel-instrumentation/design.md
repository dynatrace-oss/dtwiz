# Design

## Context

See proposal.md for motivation. Today every auto-instrumented runtime (Python, Java, Node.js, Go) plugs into `pkg/installer/otel/` through the same pattern: a runtime entry in `detectAvailableRuntimes()`, a project scan on file markers via `scanProjectDirs`, process enrichment through `listProcesses`/`matchProcessesToProjects`, then a runtime plan (preview, confirm, execute) and `WatchIngest`. Go is the closest model for .NET: services are discovered inside a selected project, compiled binaries are told apart from tooling, and the project is listed even when the toolchain is missing.

Constraints that shape the approach:

- `scanProjectDirs` matches markers by exact file name; .NET project files have arbitrary names (`Api.csproj`), so matching needs a suffix mode. After a directory matches, the scanner skips its children, which already gives "solution directory is one project".
- `ignoredProjectDirNames` has `build`, `out`, `target` but not `bin` or `obj`.
- `.NET` is currently only an entry in `manualLanguages` (`dt-url.net/otel-dotnet`).
- Process detection is OS-specific (`ps`/`lsof` vs PowerShell CIM); .NET app hosts have no `dotnet` in their command line, so a filter-term approach (Java, Node.js) does not work; the Go approach (list all, then classify) does.
- Upstream facts (OpenTelemetry .NET automatic instrumentation v1.20.0, README and release assets): archives per `linux-glibc`/`linux-musl` x `x64`/`arm64`, `macos` (contains only `osx-arm64`), `windows` (win-x86 and win-x64); the official shell installer only picks the archive, optionally verifies it with `gh`, then unzips into `~/.otel-dotnet-auto`; activation is by `CORECLR_*`/`COR_*` profiler variables plus `DOTNET_STARTUP_HOOKS` and `OTEL_DOTNET_AUTO_HOME`; the Dynatrace page adds only the OTLP endpoint, header and delta temporality variables. The release is immutable and publishes `checksums.txt`.

## Goals / Non-Goals

**Goals:**

- Record the end-to-end plan and cross-step rules once (`dotnet-otel-instrumentation`) so follow-up changes only add detail.
- Deliver step 1 (`dotnet-project-detection`) completely and testably behind `DTWIZ_DOTNET_OTEL`, with no machine changes.
- Keep detection logic reusable by steps 2-5 (services carry name, target frameworks, .NET Framework and self-contained flags).

**Non-Goals:**

- Installing, verifying or launching anything (steps 2-5; their specs come later).
- Instrumenting .NET Framework, IIS, Windows services, self-contained or native-AOT applications.
- Evaluating MSBuild (imports, conditions, `Directory.Build.props`); project files are read statically.
- Removing the flag, or changing the manual-languages list other than hiding `.NET` while the flag is on.

## Decisions

### D1. Gate through the existing runtime registry, with a new feature flag

Register `DotNetOtel` (`--dotnet-otel`, `DTWIZ_DOTNET_OTEL`, default false) in `pkg/featureflags` and set the .NET runtime entry's `enabled` to `featureflags.IsEnabled(DotNetOtel)`. `--project` inference checks the flag too. *Alternatives:* reuse `Experimental` (rejected: the request was for a dedicated .NET flag, and `Experimental` has command-visibility semantics); reuse `AllRuntimes` (rejected: it is not wired into `detectAvailableRuntimes` and would conflate runtimes). The flag is removed in the change that completes step 5, as Go's gating was.

### D2. Project unit: solution directory, else project directory

A solution file (`.sln`/`.slnx`) makes its directory the project; project files below it are its services, not separate projects. Without a solution, each directory containing a project file is its own project. This reuses the scanner's "stop at first match" behavior and mirrors how Go treats `go.work`. Solution members outside the project directory are ignored; an unreadable solution falls back to a bounded search (5 levels, as for Go `package main`). *Alternative:* list every `.csproj` (rejected: noisy picker for multi-project solutions, and one confirmation per service is the established model).

### D3. Static project-file reading, SDK classification instead of MSBuild evaluation

Services are classified by reading the project XML: executable `OutputType`, or a web/worker SDK, which implies an executable. Libraries (default) and test projects (`IsTestProject` or a test SDK package reference) are skipped. Target frameworks, assembly name and self-contained are read when declared; when they come from shared build files, the framework is `unknown` rather than guessed. Legacy non-SDK project files are treated as .NET Framework. *Alternative:* shell out to `dotnet msbuild -getProperty` (rejected for step 1: requires the SDK and slows detection; may be offered in step 2 when the SDK is present, to resolve `unknown`).

### D4. Process detection by classification, not by filter term

List processes once and keep those that are `dotnet run|watch|<app>.dll` invocations or app-host executables (executable with a same-named managed library and `.runtimeconfig.json` beside it), then drop tooling (build servers, compilers, MSBuild nodes, restore, test hosts, IDE language services, the bare `dotnet` CLI) and dtwiz. Association uses the existing two-signal match (working directory or command line) plus the executable path, so processes running from `bin/<config>/<tfm>/` match. Unix and Windows share the classifier and keep separate build-tagged listing files, as Go does. OS differences the classifier must absorb: `dotnet.exe` and `<App>.exe` on Windows, case-insensitive paths with either separator, quoted command lines with spaces on Windows (Unix `ps` output is unquoted, so a path with spaces can only match through the working directory there), and working directories that may be missing: on Unix they come from a separate per-process lookup that can fail, and on Windows the existing listing selects a `WorkingDirectory` property that `Win32_Process` probably does not provide (to be confirmed on a real Windows machine), so Windows association must work from the command line and executable path alone. Linux and macOS share one Unix path. Case-insensitive, separator-insensitive path comparison is new behavior; the existing matcher only lowercases and does a substring check. IIS worker processes are deliberately not associated: mapping them to a site and project needs IIS configuration reading and belongs with IIS support.

### D5. Where step 1 stops

After selection the flow shows services with framework flags and ends in the existing manual-steps outcome. This keeps step 1 shippable and independent of install work, and mirrors Go's "toolchain not installed" outcome.

### D6. Cross-step rules fixed now (details in later specs)

- **Own download, one install path.** dtwiz downloads the archive itself and extracts it into its own directory in the user's home; the official shell script and PowerShell module are not run for plain application launch (the script only selects, verifies and unzips; the module needs elevation and a Program Files install). Env vars are computed in Go and passed through the launched process, so no token touches disk.
- **Latest release, not pinned.** Chosen by the maintainers. Consequence: no embedded digest, so integrity comes from the release's own checksum, plus the optional attestation check when `gh` is installed and authenticated. A failed check never falls back to a weaker one; a missing `gh` is silent. The README layout is validated after extraction so upstream layout changes fail loudly.
- **Platform gaps are upstream's:** macOS on Intel and Windows on arm64 have no archive; they get the manual outcome.

## Risks / Trade-offs

- **Unpinned latest** can break on an upstream release → post-extraction layout check, env-var tests pinned to the known layout, reuse of an existing installation when the release is unreachable, and a version override as a later option.
- **Unpinned verification is weaker than pinned** (checksum from the same origin proves integrity, not authenticity) → optional attestation layer; the spec states this plainly.
- **Static project reading misses settings from shared build files** → `unknown` framework instead of a guess; step 2 can resolve it with the SDK when present.
- **Process classification heuristics may misidentify** apps or tooling (e.g. unusual app-host layouts, new tooling names) → tooling list kept in one place with tests; matching only affects the picker annotation and the stop list, and stopping needs user confirmation (later steps).
- **Suffix-aware marker matching touches shared scanning code** → exact-name behavior stays the default, existing scan tests cover it; new tests for suffix mode.
- **Scan cost** of another runtime in the parallel scan → only when the flag is on.

## Migration Plan

Step 1 ships dark: flag off by default, no machine changes, shared-code changes limited to suffix matching and two more ignored directory names. Rollback is leaving the flag off or reverting the new files. Later steps ship behind the same flag; the flag is removed when step 5 is complete.

## Open Questions

- Where the version-override option for the installed release should live (flag, env var) and whether it is part of step 3 or a follow-up. Not needed for step 1.
- Whether the macOS archive's `additional-deps` generation scripts are required for newer runtimes; verify in step 3 before specifying the extraction.
- Whether `dotnet` found on PATH should be used to resolve unknown target frameworks in step 2.
