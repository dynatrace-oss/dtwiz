# Tasks

Layout: install logic in `pkg/installer/otel/dotnet/` (no imports from `otel`), one file per concern, each with its `_test.go`; the plan adapter in `pkg/installer/otel/dotnet_plan.go`. Scope: steps 2 and 3 only; stop/launch and token/exporter environment are step 4, uninstall and watch step 5. Tests follow `docs/contributing/testing.md` (Arrange / Act / Assert, `TestFunction_Scenario`, test file next to the production file). Nothing in tests may use the network or the real home directory.

## 1. Preparation

- [x] 1.1 Check the current upstream release (asset names, checksum file, attestation commands, archive layout per platform) and record the findings in design.md; done against v1.20.0, see design Context. Re-check names when implementing task 3.1
- [ ] 1.2 Refine the step wording in the sibling change `openspec/changes/add-dotnet-otel-instrumentation` (`proposal.md` follow-ups, `specs/dotnet-otel-instrumentation/spec.md` "Delivery in steps", `tasks.md` workflow follow-up) so steps 2-3 ship together and launch is step 4; verify with `openspec validate add-dotnet-otel-instrumentation --strict` and `openspec validate add-dotnet-otel-plan-and-install --strict`

## 2. Platform selection

- [ ] 2.1 Create `pkg/installer/otel/dotnet/platform.go`: map OS, architecture and Linux C library to the archive name, reject unsupported combinations (macOS Intel, Windows arm64, unknown libc) with a message naming the supported ones; libc detection behind an injectable seam; verify in `dotnet/platform_test.go` with a table covering every spec scenario (linux glibc x64, linux musl arm64, macOS arm64, windows x64, each unsupported case)
- [ ] 2.2 Add the Linux libc detection implementation and the `GOOS=windows` build check; verify with `GOOS=windows go vet ./pkg/installer/otel/...` and a unit test with stubbed loader output

## 3. Release lookup, download and verification

- [ ] 3.1 Create `dotnet/release.go`: resolve the latest release version and the archive/checksum URLs through an injected HTTP client; classify unreachable and rate-limited responses; verify in `dotnet/release_test.go` using `httptest`: success, 404, rate limit, network failure
- [ ] 3.2 Create `dotnet/download.go`: stream the archive to a temporary file in the install root with context cancellation and removal on failure; verify in `dotnet/download_test.go`: success, truncated response, cancelled context leaves no file
- [ ] 3.3 Create `dotnet/verify.go`: SHA-256 against the published checksum entry, optional attestation via `gh` through the command-runner seam (silent when `gh` is missing or not signed in, abort when it fails, never fall back); verify in `dotnet/verify_test.go`: match, mismatch, missing entry, gh missing, gh not signed in, attestation fails after matching checksum

## 4. Extraction and installation

- [ ] 4.1 Create `dotnet/extract.go`: zip extraction below a target directory, rejecting traversal, absolute and symlink entries, bounded total size (512 MB); verify in `dotnet/extract_test.go` with generated archives: normal, traversal entry, oversized
- [ ] 4.2 Create `dotnet/layout.go`: per-platform expected files (`VERSION`, the startup hook, the platform's native library) and validation that names the first missing item; the installed version is read from `VERSION`; verify in `dotnet/layout_test.go`: valid layout, each platform's missing-file case
- [ ] 4.3 Create `dotnet/install.go`: orchestration (resolve, reuse if current, download, verify, extract into a sibling temporary directory, validate, swap, record version), install root and clients injected, failure stages named, user-fixable causes distinguished (no network, not writable, disk full); verify in `dotnet/install_test.go` with a fake release server: fresh install, already current (no download), newer release, failure keeps the previous version, interrupted first install leaves nothing, release unreachable with and without an existing installation, not-writable root
- [ ] 4.4 Add `logger.Debug` lines for the selected archive, URLs, verification results and install paths; verify with `--debug` in the manual run (task 6.3)

## 5. Plan, prerequisites and wiring

- [ ] 5.1 Create `pkg/installer/otel/dotnet_prereq.go`: locate `dotnet` through the existing lookup seam, parse `dotnet --list-runtimes`, compare by major version per service (multi-target passes on any satisfied framework, unknown passes with a note, parse failure treated as unknown); verify in `dotnet_prereq_test.go`: dotnet missing, runtime missing, newer major present, multi-target, unknown framework, unparseable output
- [ ] 5.2 Create `pkg/installer/otel/dotnet_plan.go` plan type with `Runtime`, `PrintPlanSteps`, `Execute` and `buildDotnetInstrumentationPlan()`: per-service rows (instrumented, or reason), installed or to-download instrumentation with its location, no side effects while building or printing; verify in `dotnet_plan_test.go`: mixed supported and unsupported services, all unsupported, library-only, prerequisite failures, preview creates no files, no credential in the preview
- [ ] 5.3 Implement `Execute`: install or reuse through the `dotnet` installer then the manual-steps outcome with the "automatic launch not available yet" line; Ctrl+C cancels and cleans up; no process is stopped or started; verify in `dotnet_plan_test.go` with stubbed installer and process seams: success shows install path and manual outcome, failure returns a stage-named error, interrupted cleans up, no stop/start seam called
- [ ] 5.4 Wire into `createRuntimePlan` in `pkg/installer/otel/otel.go` and the selection flow: with the flag on, a selected .NET project goes through preview and confirmation (`--dry-run`, `--yes`, decline exits cleanly with `ErrInstallCancelled` handling) instead of `showDotNetStep1Outcome`; with the flag off nothing changes; verify in `otel_test.go`: flag off, flag on dry-run, decline, `--yes`, all-unsupported project goes straight to the manual outcome
- [ ] 5.5 Keep `showDotNetStep1Outcome` only if still reachable (library-only and all-unsupported cases) or fold it into the plan; verify nothing unused remains with `make lint`

## 6. Integration checks and docs

- [ ] 6.1 Run `make test` and `make lint`; both pass, and `GOOS=windows go vet ./...` and `GOOS=linux go vet ./...` are clean
- [ ] 6.2 Add the `CHANGELOG.md` `[Unreleased]` "Added" entry for the .NET plan and instrumentation install (behind the flag); verify the entry renders under `[Unreleased]`
- [ ] 6.3 Manual run on macOS with `DTWIZ_DOTNET_OTEL=true dtwiz install otel --dry-run` and then a confirmed run against the example projects in `examples/dotnet-*`: preview correct, nothing changed on dry run, instrumentation installed under `~/.opentelemetry/dotnet/`, checksum verified (and attestation when `gh` is signed in), running process untouched, manual outcome shown; repeat on Linux (glibc and musl) and Windows where available and record gaps
- [ ] 6.4 Run `openspec validate add-dotnet-otel-plan-and-install --strict`; it passes

## Workflow follow-up

- Step 4 (stop and launch, token/exporter environment, process summary) gets its own change after this one lands.
- Step 5 (uninstall, ingest watch) follows step 4.
- Archive the umbrella change before or together with this one so the `dotnet-otel-instrumentation` capability exists in `openspec/specs/`.
