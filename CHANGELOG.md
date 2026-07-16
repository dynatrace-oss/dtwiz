# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.30] - 2026-07-16

### Changed

- `install otel`: Python virtual environment stub files are now recognized during scanning, improving venv detection accuracy; runtime scan respects project boundaries more strictly to avoid false positives
- `install otel`: Java process detection improved — better handling of missing `javac` and multi-module project layouts
- `install gcp`: GCP environment detection refined for more accurate cloud identification
- Dependency bumps: `golang.org/x/sys` 0.46.0 → 0.47.0, `golang.org/x/term` 0.44.0 → 0.45.0

### Fixed

- `install azure` / `update azure`: older Azure CLI versions that return subscription data in a different format are now handled gracefully; a preflight check is added to detect and report the incompatibility clearly
- `install aws` / `install aws-lambda`: AWS endpoint URL parsing corrected to handle edge-case URL formats; hyperlinks in AWS output now render correctly on supported terminals
- `install gcp`: a preview notice is now shown when the GCP Service Account principal has not been provisioned yet, instead of silently proceeding

## [0.2.29] - 2026-07-09

### Added

- `install gcp`, `update gcp`, `uninstall gcp`: new GCP cloud integration — provisions a Workload Identity Federation pool and provider, activates the `da-gcp` extension, and creates monitoring configuration with project and feature-set selection; `update gcp` reconciles the monitoring config in place without touching auth, and `install gcp` delegates to update when a complete connection already exists
- `install gcp` / `install azure`: Azure and GCP resource types are now included in the ingest watch cloud monitoring section
- Integration tests for GCP, Azure, and Kubernetes install/update/uninstall lifecycles; e2e tests for GCP and Azure behind the `integration` build tag

### Changed

- OTel installer code reorganized into `pkg/installer/otel/` subdirectory for a cleaner package layout
- `setup`: improved wording and messaging for cloud installation flow — clearer progress indicators, better hyperlink detection, and more actionable recommendations
- Shared `ExtensionClient` abstraction extracted for Azure and GCP Dynatrace API interactions (extension activation, polling, status checks)
- Shared `retry` and `concurrent` utilities added to `pkg/installer/` for use across cloud installers
- Test helpers moved from `pkg/testutil/` to `test/helpers/`

## [0.2.28] - 2026-07-06

### Added

- `install azure`, `update azure`, `uninstall azure`: new Azure cloud integration — provisions the Service Principal / federated credential auth chain, activates the `da-azure` extension, and creates monitoring configuration with location and feature-set selection; `update azure` reconciles the monitoring config in place without touching auth, and `install azure` delegates to update when a complete connection already exists
- `install aws-lambda`: instrumented functions now get a runtime info container
- `uninstall kubernetes`: `--dry-run` support, shows the removal plan before confirmation

### Changed

- `install otel`: directory scanning now excludes the parent of the current working directory to avoid noisy/irrelevant project detections
- Kubernetes install and uninstall logic broken down into smaller, more maintainable pieces; distribution detection functions cleaned up
- Dependency bumps: `golang.org/x/net` 0.54.0 → 0.55.0, `actions/checkout` v6 → v7

## [0.2.27] - 2026-06-30

### Added

- `install kubernetes`: Helm installation on Windows now resolves the Helm binary path after installation via a PATH refresh, so the install no longer fails when Helm was just installed in the same session; Helm-related logic extracted into dedicated files (`helm_install.go`)

### Changed

- `install kubernetes` / `uninstall kubernetes`: DynaKube and EdgeConnect CRs are now uninstalled separately for more resilient cleanup
- Error handling: command usage is no longer printed to the user when an external (non-usage) error occurs; usage output is now suppressed on `install`, `update`, and `uninstall` commands

### Fixed

- `install otel`: OTel host monitoring informational banner moved from the install scripts (`install.sh`, `install.ps1`) into the OTel installer itself, so it is shown at the correct point in the flow regardless of how the tool is invoked

## [0.2.26] - 2026-06-29

### Added

- `install oneagent`: V2 installer — pre-flight OS/arch classification, existing-agent detection with update prompt, sudo availability check on Linux; dynamic endpoint resolution from the Dynatrace tenant API; streamed installer download; Linux CMS signature verification via `openssl cms -verify` (skipped on non-Linux); OS-specific command construction with sudo/UAC elevation
- `install oneagent`: new flags `--monitoring-mode`, `--no-verify-signature`, `--skip-connectivity-check`, `--connectivity-check-only`
- `uninstall oneagent`: V2 uninstall — checks agent is installed before proceeding, shows plan before confirmation, dry-run support; uses `display.PrintStatusLine` for output
- Integration tests for OneAgent V2 lifecycle (install → detect → uninstall → verify cleanup); real-tenant e2e tests behind the `integration` build tag for Linux (root) and Windows
- `install kubernetes`: DynaKube manifest is now rendered with distribution-aware settings — disables the CSI driver on GKE Autopilot and applies other per-distro adjustments
- `uninstall kubernetes`: prints the active kubectl context and detected K8s distribution before proceeding
- `setup`: "Show uninstall commands" option (`[u]`) added to the interactive menu; displays available `dtwiz uninstall` subcommands without leaving the flow
- `setup`: docs link for additional deployment options shown above the recommendation list
- `pkg/display`: `Hyperlink()` helper renders OSC 8 clickable terminal links; falls back to `text: url` on non-TTY outputs and Apple Terminal (which doesn't support right-click navigation)
- Install scripts (`install.sh`, `install.ps1`): show an informational banner about OTel host monitoring with a clickable link before prompting for confirmation

### Changed

- `install oneagent`: replaced legacy shell/exe wrapper with the V2 flow; `ONEAGENT_POC` feature flag removed, V2 is now unconditional
- GKE Autopilot detection now uses node name prefix (`gk3-`) instead of kube-system namespace annotation, which is the officially documented signal and more reliably available

## [0.2.25] - 2026-06-17

### Added

- `install oneagent`: pre-flight connectivity checks — TCP-probes all required Dynatrace endpoints in parallel before installation and prints latency or error per endpoint; failures are non-blocking (warning shown, install continues)
- `install oneagent`: automatically determine target OS and architecture from the Dynatrace API before downloading the installer

### Changed

- `update otel` is now hidden behind the `Experimental` feature flag; it remains fully functional but is only visible and accessible when `--experimental` or `DTWIZ_EXPERIMENTAL=true` is set
- `setup` flow excludes `update otel` from the actionable recommendation list when the `Experimental` flag is not enabled

## [0.2.24] - 2026-06-15

### Added

- `featureflags` package: new `Experimental` feature flag, enabled via `--experimental` flag or `DTWIZ_EXPERIMENTAL=true` env var

### Changed

- `install docker` and `install demo` are now hidden behind the `Experimental` feature flag; they remain fully functional but are only visible and accessible when the flag is enabled

### Fixed

- `uninstall oneagent`: remove the residual `/opt/dynatrace/oneagent` stub directory that the vendor uninstall script leaves behind on Linux; a warning is printed (and uninstall still succeeds) if the cleanup fails

## [0.2.23] - 2026-06-11

### Added

- Kubernetes distribution detection extended: detects RKE2, GKE Autopilot, EKS Bottlerocket, and `kind` sub-variants via kubectl probes

### Changed

- Access-token auth is now opt-in and flag-only: it activates only when `--access-token` is passed explicitly and is no longer read from the `DT_ACCESS_TOKEN` env var. This prevents a leftover `DT_ACCESS_TOKEN` from silently switching Classic API calls off the platform token. When `--access-token` is absent, the platform token is used for Classic API calls.

### Fixed

- `install aws`: accept HTTP 202 from the extension install endpoint (normal response when the extension is already installed)
- `install aws`: resolve latest installed `da-aws` extension version dynamically for monitoring config creation; avoids 404 caused by a hardcoded version mismatch
- `install aws`: use the latest CloudFormation template URL; fixes 403 returned by the previously pinned (retired) version
- `install aws`: enable monitoring config and credentials after CloudFormation deploy (aligns with `dtctl enable aws monitoring` behaviour)
- `watch`: add cloud-platform signal line (AWS metrics + `da-*` logs by `aws.resource.type`) scoped to the installed account; surfaces activity before Smartscape topology builds up
- `watch`: escape AWS account ID before interpolating into DQL to prevent injection via malformed account IDs

## [0.2.22] - 2026-06-09

### Fixed

- `install otel-python`: check that Python is installed before scanning for projects; avoids a confusing error when Python is not on the system

## [0.2.21] - 2026-06-08

### Added

- `dtwiz uninstall otel`: interactive picker when multiple Dynatrace OTel Collector instances are found (running or installed); Docker container support; "Uninstall all" option for multiple instances
- `dtwiz update otel`: interactive running-collector picker when `--config` is omitted; supports container-based collectors including config extract/patch/write-back for Docker-managed configs

### Changed

- Kubernetes DynaKube template: pin ActiveGate, EEC, CodeModules, and node agent images to specific build tags
- Kubernetes DynaKube template: right-size memory and CPU requests/limits to better match observed usage

## [0.2.20] - 2026-06-03

### Fixed

- Kubernetes operator: switch Helm chart registry from ECR to `ghcr.io/dynatrace/dynatrace-operator`, pin to nightly build (`0.0.0-nightly-chart`)

## [0.2.19] - 2026-06-01

### Added

- Windows install script (`install.ps1`) now adds Windows Security exclusion instructions and handles long path enablement to prevent the binary from being blocked
- Contributing docs for testing (`docs/contributing/testing.md`) and pull requests (`docs/contributing/pull-requests.md`)

### Changed

- OTel Collector is now installed to `~/opentelemetry/` (user home directory) instead of the current working directory to avoid permission issues
- Recommender doesn't exit early when OneAgent is installed
- Project detection is faster on large directory trees, no longer depth-limited, skips Windows system directories
- `install otel` and `install otel-node` now wire the `--project` flag through to the Node.js OTel installer

## [0.2.18] - 2026-05-21

### Changed

- Platform token (`--platform-token` / `DT_PLATFORM_TOKEN`) is now the primary required credential; access token (`--access-token` / `DT_ACCESS_TOKEN`) is optional and used only when explicitly configured

## [0.2.17] - 2026-05-19

### Added

- `dtwiz install otel-node` now auto-installs project dependencies (`npm install` / `yarn`) before instrumentation when `node_modules` is missing
- Node.js OTel service names are now derived from the project directory name instead of the full path
- E2E integration tests for Node.js and Java OTel instrumentation, with fixture apps (`test/fixtures/node-http`, `test/fixtures/java-maven`)
- OneAgent v2 installer scaffold (`pkg/installer/oneagent_v2.go`) with `InstallOptions` / `AgentConfig` types and a `--oneagent-poc` feature flag
- `main`-branch snapshot install channel: `install.sh` and `install.ps1` now support `DTWIZ_CHANNEL=main` for bleeding-edge builds
- Contributing guide: feature and bug request issue descriptions (`docs/contributing/issues.md`)
- Git hooks (`commit-msg`), `CODEOWNERS`, and PR-title enforcement workflow

### Changed

- OTel process wait timeout shortened to reduce time spent waiting for instrumented processes to start

### Fixed

- Ingest watch is now skipped when the user cancels an installation, instead of watching indefinitely after a no-op

## [0.2.16] - 2026-05-12

### Added

- `dtwiz install otel-java` — Java auto-instrumentation now fully implemented: detects Maven and Gradle projects (single-module and multi-module), builds the project before instrumentation, attaches the OTel Java agent, and detects the running process by port; `dtwiz uninstall otel` gracefully terminates instrumented JVM processes
- `dtwiz install otel-node` — Node.js auto-instrumentation: detects Next.js and Nuxt.js projects, injects OTel SDK via a register file, fails fast when project dependencies are not installed; `dtwiz uninstall otel` removes Node.js OTel artifacts
- `--extensions` switch to `dtwiz status` to test common HTTP client and token usage
- `pkg/extensions` — new package with Platform Extensions v2 API client wrappers (`InstallExtension`, `ListMonitoringConfigs`, `CreateMonitoringConfig`, `DeleteMonitoringConfig`) wired into the AWS installer
- Feature flags package (`pkg/featureflags`) for runtime feature toggling
- Centralized HTTP client (`pkg/client`) with `ClassicClient` (API token auth) and `PlatformClient` (Bearer/platform token auth)
- E2E test infrastructure with `install otel-python` test

### Changed

- AWS `install` now activates `com.dynatrace.extension.da-aws` via Platform Extensions v2 API before creating monitoring configurations
- OTel Collector update: minor UX improvements
- Updated QuickStart app link
- `refactor(cmd)`: commands now use the `pkg/display` package for consistent output formatting

### Fixed

- Extensions: `CreateMonitoringConfig` now POSTs a single `{scope,value}` object; Platform v2 endpoint rejects bulk arrays with HTTP 400
- AWS CloudFormation deploy: capture stderr so failures report the actual AWS CLI error instead of a bare exit status
- AWS: DT API calls are now gated behind dry-run guard and user confirmation
- Debug HTTP response body capped at 2048 bytes to prevent large/sensitive payloads appearing on stderr
- Improved instrumented process detection (cross-platform, including Windows)

## [0.2.15] - 2026-04-22

### Added

- `dtwiz install demo`: new command that downloads and extracts the schnitzel 4-service Python demo app, installs Python if missing (via brew/apt/dnf/winget), and wires it up to Dynatrace OTel monitoring end-to-end
- `dtwiz watch`: live polling for new data ingested into Dynatrace, with a `--from` flag for a custom DQL start timestamp
- Watch started in parallel with AWS CloudFormation deploy
- `--yes` / `-y` persistent flag on `install`, `update`, and `uninstall` command groups to skip all interactive confirmation prompts
- `--project <path>` flag on `install otel` and `install otel-python` to pre-select a project directory and skip interactive project scanning

### Changed

- Refactored `confirmProceed()` and shared `AutoConfirm` variable from `pkg/installer/kubernetes.go` into `pkg/installer/installer.go` where other shared utilities live
- Lambda instrumentation now runs before watch starts

### Fixed

- Quoted absolute timestamps in `watch --from` DQL queries
- Removed lambda-specific waiting message in favor of `watch`

## [0.2.14] - 2026-04-17

### Fixed

- OTel Python: fix detection of already-running processes

## [0.2.13] - 2026-04-16

### Fixed

- Post-install service polling: widen smartscape query window from 1m to 3m to reduce missed services

## [0.2.12] - 2026-04-16

### Fixed

- Fix test mocks to include Grail query `state` field required since v0.2.9

## [0.2.11] - 2026-04-16

### Fixed

- Windows: treat already-exited processes as successfully stopped instead of showing "Access is denied" warnings

## [0.2.10] - 2026-04-16

### Fixed

- Windows: fall back to `taskkill` when `TerminateProcess` cannot stop orphaned Python processes from a previous run

## [0.2.9] - 2026-04-16

### Fixed

- Post-install service polling: check Grail query state before using results to avoid intermittent missing services from partial responses

## [0.2.8] - 2026-04-16

### Fixed

- OTel Python: widen smartscape polling window from 1m to 3m so all instrumented services are detected

## [0.2.7] - 2026-04-16

### Fixed

- OTel install (Windows): kill running OTel Collector before downloading new binary
- Windows process detection: improve reliability of process lookups

## [0.2.6] - 2026-04-16

### Added

- Windows-specific service detection using `where.exe` and `Get-Process` for runtimes and daemons
- Preview/snapshot install support in `install.sh` and `install.ps1` via `DTWIZ_BRANCH` env variable
- GitHub Actions workflows for preview snapshot builds and cleanup

### Fixed

- OneAgent detection (Unix): check service is running via `systemctl is-active` instead of checking install directory
- OneAgent detection (Windows): verify service status is "Running", not just that the service exists
- OTel Collector detection (Windows): exclude shell processes and current process from fallback search to avoid false positives
- OTel uninstall: wait for process to fully exit before removing files; retry removal with backoff for Windows file lock issues

## [0.2.5] - 2026-04-14

### Changed

- OTel Python: refactor internals into dedicated modules for process management, venv handling, and package installation
- OTel Python: improve reliability: broken venv detection and recreation, process wait before re-instrumentation, and better install feedback

## [0.2.4] - 2026-04-14

### Changed

- OTel: reduce service detection and log ingest verification lookback window from 10 minutes to 1 minute

## [0.2.3] - 2026-04-13

### Added

- AWS Lambda: set `DT_ENABLE_ESM_LOADERS=true` automatically for Node.js runtimes
- AWS Lambda: poll Dynatrace after instrumentation and show a getting started link once each function appears as a service (uses substring match to handle the region suffix, e.g. "helloWorldNode2 in us-east-1")

## [0.2.2] - 2026-04-13

### Fixed

- OTel runtime scan: increase project search depth for more reliable detection
- OTel environment: fix service wait timeout and QuickStart app URL

### Changed

- CI: add coverage reporting to test workflow
- Makefile: add coverage targets

## [0.2.1] - 2026-04-08

### Added

- Azure and GCP cloud services now appear in recommendations when detected (shown as "coming soon", not selectable)
- `MethodAzure` and `MethodGCP` ingestion method constants
- `ComingSoon` field on `Recommendation` struct for items that are detected but not yet installable

### Changed

- Recommendation titles rewritten to focus on what gets monitored rather than method names (e.g. "This machine's services (via OneAgent)" instead of "Install Dynatrace OneAgent on this host")
- Recommendation header changed to "What do you want to monitor?"
- Removed `→ dtwiz install <method>` command hints from recommendation display

## [0.2.0] - 2026-04-07

### Added

- `dtwiz install aws-lambda` — instrument all Lambda functions in the current AWS region with the Dynatrace Lambda Layer (auto-detect runtime, fetch layer ARN from DT API, set connection env vars)
- `dtwiz uninstall aws-lambda` — remove Dynatrace Lambda Layer and DT\_\* env vars from all instrumented functions
- `dtwiz install aws` now runs Lambda instrumentation in parallel alongside CloudFormation deployment (non-fatal, skipped in dry-run)
- Skip Dynatrace-internal Lambda functions (`DynatraceApiClientFunction`) during install and uninstall
- Skip container image Lambda functions (layers not supported)
- `--verbose`/`-v` flag (count-based): enables verbose debug output
- `--debug`/`-vv` enables debug logging
- Active DT environment URL shown after banner in `dtwiz setup`
- Access token and platform token validation before every command
- CLI login hints when cloud/k8s tools are not detected during analysis
- OpenSpec workflow for planning changes (`openspec/` directory)
- GitHub Actions: run tests on PRs

## [0.1.4] - 2026-03-27

### Added

- GCP detection: detect project, account, and services (Compute VMs, GKE, Cloud Functions, Cloud Run, Cloud SQL, GCS Buckets) via `gcloud` CLI
- Docker variant detection: identify Docker Desktop, Rancher Desktop, OrbStack, and Colima

### Changed

- ASCII banner now rendered in purple (bold magenta)
- System analysis summary: `none` replaced with `<none>` for undetected components
- System analysis summary: muted text uses `color.Faint` style
- Simplified OTel Collector summary line (show binary path only, drop config path)
- Kubernetes summary: show distribution name directly instead of `dist=` prefix

## [0.1.3] - 2026-03-26

### Added

- ASCII banner displayed on `dtwiz setup`, `dtwiz` (no command), and `dtwiz --help`
- Banner includes version number and tagline "HASTA LA VISTA - BLIND SPOTS!"

## [0.1.2] - 2026-03-23

### Changed

- All `install` commands now use a consistent "Proceed with installation?" confirmation prompt
- Overhauled OTel install preview UI: purple title, separator-based config blocks, numbered sections (1) Collector, 2) Python), intro line for two-part installs
- Running OTel Collector processes are now detected before install and shown in the preview with their PID and binary path; stopped unconditionally without a separate prompt
- `install otel-python` standalone preview now matches the style of other installers (purple title, separator, purple "Steps:" header)
- Removed unofficial support disclaimer from README

## [0.1.1] - 2026-03-23

### Changed

- Renamed Go module path and all repository references from `dietermayrhofer/dtwiz` to `dynatrace-oss/dtwiz`

## [0.1.0] - 2026-03-23

### Added

- Initial release of **dtwiz** — zero-config Dynatrace observability setup CLI
- `dtwiz setup` — interactive analyze → recommend → pick → install workflow
- `dtwiz analyze` — detect platform, Docker, Kubernetes, OneAgent, OTel Collector, AWS, Azure, services (Linux, macOS, Windows)
- `dtwiz recommend` — priority-ranked ingestion recommendations
- `dtwiz status` — connection status and system analysis
- `dtwiz install oneagent` — full-stack OneAgent with optional `--host-group`, supports Linux/macOS/Windows
- `dtwiz install kubernetes` — Dynakube CR with `cloudNativeFullStack` mode via Helm
- `dtwiz install docker` — Docker monitoring via OneAgent container
- `dtwiz install otel-collector` — OpenTelemetry Collector with Dynatrace exporter, config auto-generated from template
- `dtwiz install otel-python` — Python auto-instrumentation with project detection, process management, and DQL log poll
- `dtwiz install otel-java` — Java auto-instrumentation (stub)
- `dtwiz install aws` — AWS CloudWatch / metric streams integration
- `dtwiz install azure` — Azure cloud integration (stub)
- `dtwiz install gcp` — GCP integration (stub)
- `dtwiz update otel` — patch an existing OTel Collector config in-place
- `dtwiz uninstall` — OneAgent, Kubernetes, OTel, AWS, self; all with `--dry-run`
- `dtwiz version` — build-time version injection via ldflags
- Bootstrap install scripts (`scripts/install.sh`, `scripts/install.ps1`)
- Embedded Go templates for Dynakube CR, OTel Collector config, and AWS config

[Unreleased]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.30...HEAD
[0.2.30]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.29...v0.2.30
[0.2.29]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.28...v0.2.29
[0.2.28]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.27...v0.2.28
[0.2.27]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.26...v0.2.27
[0.2.26]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.25...v0.2.26
[0.2.25]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.24...v0.2.25
[0.2.24]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.23...v0.2.24
[0.2.23]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.22...v0.2.23
[0.2.22]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.21...v0.2.22
[0.2.21]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.20...v0.2.21
[0.2.20]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.19...v0.2.20
[0.2.19]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.18...v0.2.19
[0.2.18]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.17...v0.2.18
[0.2.17]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.16...v0.2.17
[0.2.16]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.15...v0.2.16
[0.2.15]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.14...v0.2.15
[0.2.14]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.13...v0.2.14
[0.2.13]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.12...v0.2.13
[0.2.12]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.11...v0.2.12
[0.2.11]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.10...v0.2.11
[0.2.10]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.9...v0.2.10
[0.2.9]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.8...v0.2.9
[0.2.8]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.7...v0.2.8
[0.2.7]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.6...v0.2.7
[0.2.6]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.4...v0.2.0
[0.1.4]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/dynatrace-oss/dtwiz/releases/tag/v0.1.0
