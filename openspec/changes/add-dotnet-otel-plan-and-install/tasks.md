# Tasks

Layout: install logic in `pkg/installer/otel/dotnet/` (no imports from `otel`), one file per concern, each with its `_test.go`; the plan adapter in `pkg/installer/otel/dotnet_plan.go`. Scope: steps 2 and 3 only; stop/launch and token/exporter environment are step 4, uninstall and watch step 5. Tests follow `docs/contributing/testing.md` (Arrange / Act / Assert, `TestFunction_Scenario`, test file next to the production file). Nothing in tests may use the network or the real home directory.

## 1. Preparation

- [x] 1.1 Check the current upstream release (asset names, checksum file, attestation commands, archive layout per platform) and record the findings in design.md; done against v1.20.0, see design Context. Re-check names when implementing task 3.1
- [x] 1.2 Refine the step wording in the sibling change `openspec/changes/add-dotnet-otel-instrumentation` (`proposal.md` follow-ups, `specs/dotnet-otel-instrumentation/spec.md` "Delivery in steps", `tasks.md` workflow follow-up) so steps 2-3 ship together and launch is step 4; verify with `openspec validate add-dotnet-otel-instrumentation --strict` and `openspec validate add-dotnet-otel-plan-and-install --strict`

## 2. Platform selection

- [x] 2.1 Create `pkg/installer/otel/dotnet/platform.go`: map OS, architecture and Linux C library to the archive name, reject unsupported combinations (macOS Intel, Windows arm64, unknown libc) with a message naming the supported ones; libc detection behind an injectable seam; verify in `dotnet/platform_test.go` with a table covering every spec scenario (linux glibc x64, linux musl arm64, macOS arm64, windows x64, each unsupported case)
- [x] 2.2 Add the Linux libc detection implementation and the `GOOS=windows` build check; verify with `GOOS=windows go vet ./pkg/installer/otel/...` and a unit test with stubbed loader output

## 3. Release lookup, download and verification

- [x] 3.1 Create `dotnet/release.go`: resolve the latest release version and the archive/checksum URLs through an injected HTTP client; classify unreachable and rate-limited responses; verify in `dotnet/release_test.go` using `httptest`: success, 404, rate limit, network failure
- [x] 3.2 Create `dotnet/download.go`: stream the archive to a temporary file in the install root with context cancellation and removal on failure; verify in `dotnet/download_test.go`: success, truncated response, cancelled context leaves no file
- [x] 3.3 Create `dotnet/verify.go`: SHA-256 against the published checksum entry, optional attestation via `gh` through the command-runner seam (silent when `gh` is missing or not signed in, abort when it fails, never fall back); verify in `dotnet/verify_test.go`: match, mismatch, missing entry, gh missing, gh not signed in, attestation fails after matching checksum

## 4. Extraction and installation

- [x] 4.1 Create `dotnet/extract.go`: zip extraction below a target directory, rejecting traversal, absolute and symlink entries, bounded total size (512 MB); verify in `dotnet/extract_test.go` with generated archives: normal, traversal entry, oversized
- [x] 4.2 Create `dotnet/layout.go`: per-platform expected files (`VERSION`, the startup hook, the platform's native library) and validation that names the first missing item; the installed version is read from `VERSION`; verify in `dotnet/layout_test.go`: valid layout, each platform's missing-file case
- [x] 4.3 Create `dotnet/install.go`: orchestration (resolve, reuse if current, download, verify, extract into a sibling temporary directory, validate, swap, record version), install root and clients injected, failure stages named, user-fixable causes distinguished (no network, not writable, disk full); verify in `dotnet/install_test.go` with a fake release server: fresh install, already current (no download), newer release, failure keeps the previous version, interrupted first install leaves nothing, release unreachable with and without an existing installation, not-writable root
- [ ] 4.4 Add `logger.Debug` lines for the selected archive, URLs, verification results and install paths; verify with `--debug` in the manual run (task 6.3)

## 5. Plan, prerequisites and wiring

- [x] 5.1 Create `pkg/installer/otel/dotnet_prereq.go`: locate `dotnet` through the existing lookup seam, parse `dotnet --list-runtimes`, compare by major version per service (multi-target passes on any satisfied framework, unknown passes with a note, parse failure treated as unknown); verify in `dotnet_prereq_test.go`: dotnet missing, runtime missing, newer major present, multi-target, unknown framework, unparseable output
- [x] 5.2 Create `pkg/installer/otel/dotnet_plan.go` plan type with `Runtime`, `PrintPlanSteps`, `Execute` and `buildDotnetInstrumentationPlan()`: per-service rows (instrumented, or reason), installed or to-download instrumentation with its location, no side effects while building or printing; verify in `dotnet_plan_test.go`: mixed supported and unsupported services, all unsupported, library-only, prerequisite failures, preview creates no files, no credential in the preview
- [x] 5.3 Implement `Execute`: install or reuse through the `dotnet` installer then the manual-steps outcome with the "automatic launch not available yet" line; Ctrl+C cancels and cleans up; no process is stopped or started; verify in `dotnet_plan_test.go` with stubbed installer and process seams: success shows install path and manual outcome, failure returns a stage-named error, interrupted cleans up, no stop/start seam called
- [x] 5.4 Wire into `createRuntimePlan` in `pkg/installer/otel/otel.go` and the selection flow: with the flag on, a selected .NET project goes through preview and confirmation (`--dry-run`, `--yes`, decline exits cleanly with `ErrInstallCancelled` handling) instead of `showDotNetStep1Outcome`; with the flag off nothing changes; verify in `otel_test.go`: flag off, flag on dry-run, decline, `--yes`, all-unsupported project goes straight to the manual outcome
- [x] 5.5 Keep `showDotNetStep1Outcome` only if still reachable (library-only and all-unsupported cases) or fold it into the plan; verify nothing unused remains with `make lint`

## 6. Integration checks and docs

- [x] 6.1 Run `make test` and `make lint`; both pass, and `GOOS=windows go vet ./...` and `GOOS=linux go vet ./...` are clean
- [x] 6.2 Add the `CHANGELOG.md` `[Unreleased]` "Added" entry for the .NET plan and instrumentation install (behind the flag); verify the entry renders under `[Unreleased]`
- [ ] 6.3 Manual run on macOS with `DTWIZ_DOTNET_OTEL=true dtwiz install otel --dry-run` and then a confirmed run against the example projects in `examples/dotnet-*`: preview correct, nothing changed on dry run, instrumentation installed under `~/.opentelemetry/dotnet/`, checksum verified (and attestation when `gh` is signed in), running process untouched, manual outcome shown; repeat on Linux (glibc and musl) and Windows where available and record gaps
- [x] 6.4 Run `openspec validate add-dotnet-otel-plan-and-install --strict`; it passes

## 7. Shared build properties (found in the first manual preview)

- [x] 7.1 Read values a project file does not declare from the applicable `Directory.Build.props` in `pkg/installer/otel/dotnet/projectfile.go`: nearest file upward, followed through an import of a parent `Directory.Build.props`, unconditional properties only, project values win, values that reference another property stay unknown; verify in `dotnet/projectfile_test.go`: inherited framework, project overrides shared, chained parent, shared self-contained, property reference stays unknown, no shared file; spec: dotnet-project-detection "Service details read from the project file"
- [x] 7.2 Make the preview note say where the framework was looked for when it is still unknown ("not declared in the project or Directory.Build.props"); verify in `dotnet_prereq_test.go` and `dotnet_plan_test.go`
- [x] 7.3 Verify against `examples/dotnet-coffee-shop-net11` (and the other coffee-shop examples) that the preview shows the real framework; verify with a test that copies the example's props layout into a temp dir

## 8. Preview in the style of the other runtimes

- [x] 8.1 Add `ActivationEnv` to `pkg/installer/otel/dotnet/` (new file `activation.go`): startup hook, auto-home, profiler enable/id and native library path for the platform (`CORECLR_PROFILER_PATH` on Linux and macOS, `_64` and `_32` variants on Windows); verify in `dotnet/activation_test.go` for glibc, musl, macOS and Windows against the values of the upstream `instrument.sh`
- [x] 8.2 Rewrite `PrintPlanSteps` in `dotnet_plan.go`: Project line, stop line with PIDs, one download/reuse line, one verify line, the activation environment once (secrets redacted by the shared formatter), one launch line per supported service with its service name, one skipped line per unsupported service, one line that stop and launch are not performed yet; the builder takes the collector endpoint so the exporter variables match the other runtimes; verify in `dotnet_plan_test.go`: two services with PIDs, mixed supported and unsupported, no PIDs, reused installation, manual outcome shows only reasons
- [x] 8.3 Run the preview against `examples/dotnet-coffee-shop-net11` and check it reads like the Python, Java and Go previews; verify by capturing the output in a test and by eye in task 6.3

## 9. Build step and order in the preview

- [x] 9.1 Add `pkg/installer/otel/dotnet/build.go`: assembly name on `Service`, discovery of an existing built library (configurations, frameworks, runtime configuration file beside it, most recent wins), the newest-source walk (ignored directories skipped, extension list, file limit that forces a build) and the default output path for a service that needs a build; verify in `dotnet/build_test.go`: library present and current, missing, sources newer, Debug and Release (newest wins), no runtime configuration file, multi-target, unknown framework, walk limit, ignored `bin`/`obj` sources do not count
- [x] 9.2 Reorder and extend `PrintPlanSteps` in `dotnet_plan.go`: download, verify, stop, build (only for services that need it, labelled "without instrumentation", with the "sources changed" note), environment, launch with `dotnet <library>`, "existing build" marker, closing line mentioning stopping, building and launching; verify in `dotnet_plan_test.go`: order of steps, existing build has no build line, no build yet, stale build, no running processes, unsupported service has no build or launch line
- [x] 9.3 Check the preview for the example projects, built and not built, and update the CHANGELOG wording if needed; verify by running the plan against `examples/dotnet-coffee-shop-net11` before and after `dotnet build` in a copy of it

## 10. Use an existing installation as it is

- [x] 10.1 Make `Install` in `pkg/installer/otel/dotnet/install.go` return a valid existing installation without release lookup, download or verification; only an invalid directory is reinstalled; update `dotnet/install_test.go`: installed and newer release exists (no request reaches the server), installed and server down, broken installation replaced, failed reinstall of a broken installation leaves its content, fresh install and unsupported platform unchanged
- [x] 10.2 Remove the reuse prompt for an unreachable release from `dotnet_plan.go` and its tests, report "already installed" for a reused installation; verify in `dotnet_plan_test.go`
- [x] 10.3 Preview: one "Use installed ... in <dir>" line and no verify line when installed, download and verify lines otherwise; verify in `dotnet_plan_test.go` and by running the plan against the real installation made in task 6.3's smoke test

## 11. Walkthrough link in the outcome

- [x] 11.1 Print the walkthrough as a link: the text "Instrument your .NET app" links to `https://dt-url.net/otel-dotnet` where the terminal supports links, otherwise the address follows below the text (the existing `termLink` helper), right after "Automatic launch of .NET services is not available yet; manual steps follow." and after the reason of a manual plan in `dotnet_plan.go`; stop returning the `dotnet` slug from `otel.go` so the ingest-watch footer does not repeat it; verify in `dotnet_plan_test.go` and `otel_test.go`: link printed once, directly after the statement, also for a manual plan; no slug returned

## Workflow follow-up

- Step 4 (stop and launch, token/exporter environment, process summary) gets its own change after this one lands.
- Step 5 (uninstall, ingest watch) follows step 4.
- Archive the umbrella change before or together with this one so the `dotnet-otel-instrumentation` capability exists in `openspec/specs/`.
