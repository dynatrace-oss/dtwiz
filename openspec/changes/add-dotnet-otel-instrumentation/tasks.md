# Tasks

Layout: .NET-specific logic lives in the self-contained subpackage `pkg/installer/otel/dotnet/` (no imports from `otel`); thin adapters (`dotnet_project.go`, `dotnet_process.go`, `dotnet_plan.go`) stay in `pkg/installer/otel/`.

Scope: step 1 only (flag and detection). Steps 2-5 (prerequisites and plan, installation, launch, uninstall/watch) get their own changes and specs. Tests follow `docs/contributing/testing.md` (Arrange / Act / Assert, `TestFunction_Scenario`, test file next to the production file).

## 1. Feature flag

- [x] 1.1 Add the `DotNetOtel` flag (`--dotnet-otel`, `DTWIZ_DOTNET_OTEL`, default false, description naming .NET OTel) to the registry in `pkg/featureflags/featureflags.go`; verify with new cases in `pkg/featureflags/featureflags_test.go`: default off, env `true`/`1`, CLI over env, `List()` name/env/source, listed after `RegisterFlags` (spec: feature-flag-registry)
- [x] 1.2 Verify `dtwiz status` shows the flag when enabled with its source, by extending the existing status flag test in `cmd/` (spec: dotnet-project-detection, "Flag visible in status")
- [x] 1.3 Add the flag to the feature-flag section of `docs/contributing/development.md` only if that section lists flags by name; add a `CHANGELOG.md` `[Unreleased]` "Added" entry; verify `make lint` passes on markdown if configured

## 2. Scanner support for suffix markers

- [x] 2.1 Extend marker matching in `pkg/installer/otel/runtime_scan.go` so a marker such as `*.csproj` matches by file-name suffix while plain names keep exact matching; verify in `pkg/installer/otel/runtime_scan_test.go` that exact markers behave as before and `*.csproj`/`*.sln` match, including matched marker names being the real file names
- [x] 2.2 Ignore `bin`, `obj`, `packages` for .NET scans only, by passing them as exclude names from `dotnet.IgnoredDirNames` (the shared `ignoredProjectDirNames` is unchanged, so Node.js monorepos under `packages/` keep working; `.vs` is already skipped as a dot directory); verify with a scan test that project files inside `bin/` and `obj/` are not reported and that existing runtimes' scan tests still pass (`go test ./pkg/installer/otel/...`)

## 3. Project discovery

- [x] 3.1 Create `pkg/installer/otel/dotnet/solution.go`: read member project paths from `.sln` and `.slnx`, normalize path separators, drop members outside the solution directory, return "unreadable" distinctly from "no members"; verify in `dotnet/solution_test.go` with fixtures for both formats, Windows-style separators, outside-directory members, malformed files
- [x] 3.2 Create `pkg/installer/otel/dotnet_project.go` (adapter in `otel`): `detectDotNetProjects(roots)` scanning solution and project markers, one project per directory (solution directory wins, members not listed separately, solution+project in one directory listed once); verify in `dotnet_project_test.go`: solution dir, standalone project, both in one dir, `bin`/`obj` ignored, symlink duplicate
- [x] 3.3 Cover `--project` inference: .NET markers recognised only when the flag is on and after the existing runtimes' precedence; verify in `otel_test.go` (flag off, flag on, `package.json` + `.csproj` stays Node.js)

## 4. Service discovery

- [x] 4.1 Create `pkg/installer/otel/dotnet/projectfile.go`: statically read a project file (XML) for output type, SDK, `TargetFramework(s)`, `AssemblyName`, `SelfContained`, test-project markers, and classify legacy non-SDK files as .NET Framework; verify in `dotnet/projectfile_test.go` using the scenarios from the spec (web, console, worker, library, test project, multi-target, `net48`, self-contained, framework missing → unknown, malformed XML)
- [x] 4.2 Create `pkg/installer/otel/dotnet/entrypoints.go`: discover services of a project (solution members, else bounded 5-level search, ignored directories skipped), name them (assembly name, else file name, project-relative path on collision); verify in `dotnet/entrypoints_test.go`: solution with library+test+two services, member outside the directory, unreadable solution fallback, name collision, library-only project reports no service
- [ ] 4.3 Add debug logging for discovered projects, services and skipped candidates (`logger.Debug`); verify with `--debug` against the fixture projects in the test data

## 5. Process detection

- [x] 5.1 Create `pkg/installer/otel/dotnet/process.go`: classify listed processes into `dotnet run|watch|<app>.dll` invocations and app-host executables (same-named managed library and `.runtimeconfig.json` beside the executable), drop tooling (build server, compiler server, MSBuild nodes, restore, test host, IDE language services, bare `dotnet` CLI) and dtwiz itself; verify in `dotnet/process_test.go` with stubbed process lists and a temp directory for the app-host check
- [x] 5.2 Wire association through the existing two-signal matcher plus the executable path (build output directories); add path normalization for case and separator differences, which the existing matcher (lowercase substring check) does not do; verify project annotation tests for: `dotnet run` from the project, dll in `bin/`, app-host (with and without `.exe`), `dotnet.exe`, quoted path with spaces on Windows, `dotnet run` in a path with spaces matched through the working directory on Unix, case and path-separator differences, unreadable working directory, process outside every project, scan failure yielding no PIDs, IIS worker not associated, Windows service inside a project associated
- [x] 5.3 Provide Unix and Windows process listing for the classifier by reusing `listProcesses`; add `dotnet_process_unix_test.go` (`//go:build !windows`) and `dotnet_process_windows_test.go` (`//go:build windows`) only for behavior that differs per OS (path separators, `.exe` suffix); verify with `GOOS=windows go vet ./pkg/installer/otel/...` and `go test` on the host OS

## 6. Wiring and step-1 outcome

- [x] 6.1 Register the .NET runtime in `detectAvailableRuntimes()` in `pkg/installer/otel/otel.go` enabled only by `DotNetOtel`, listed regardless of whether `dotnet` is on PATH, running project and process scans in parallel like the other runtimes; verify in `otel_test.go`: flag off → no .NET runtime and no .NET scan, flag on → .NET projects listed with PIDs
- [x] 6.2 Implement the step-1 selection outcome: show services with target framework and not-supported flags (.NET Framework, self-contained), then the manual-steps outcome with the Dynatrace .NET walkthrough, with no install, stop or start; verify with a test asserting no side-effect seams are called and the output contains the services and the link
- [x] 6.3 Hide the `.NET` entry of the manual-languages list while the flag is on and keep it while off; verify both in `dotnet_project_test.go`

## 7. Integration checks

- [x] 7.1 Run `make test` and `make lint`; both pass
- [ ] 7.2 On a real Windows machine, check whether the process listing returns a working directory for a running .NET app; record the result in design.md D4 and adjust 5.2 if it differs from the assumption that Windows association relies on the command line and executable path
- [ ] 7.3 Manual run on macOS (and Windows if available) with `DTWIZ_DOTNET_OTEL=true dtwiz install otel` and `--dry-run` from a directory with a multi-project solution and one running `dotnet run` app: project listed with PID, services and frameworks shown, nothing changed on disk, flag off lists nothing
- [x] 7.4 Run `openspec validate add-dotnet-otel-instrumentation --strict`; it passes

## Workflow follow-up

- Steps 2 and 3 (prerequisites, plan, installation) are specified together in `add-dotnet-otel-plan-and-install`.
- Write the detailed specs for steps 4 and 5 as separate changes after that.
- Archive this change after review, keeping the `dotnet-otel-instrumentation` capability as the umbrella spec that later changes extend.
