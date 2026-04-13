# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- `dtwiz uninstall aws-lambda` — remove Dynatrace Lambda Layer and DT_* env vars from all instrumented functions
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

[Unreleased]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.3...HEAD
[0.2.3]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/dynatrace-oss/dtwiz/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.4...v0.2.0
[0.1.4]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/dynatrace-oss/dtwiz/releases/tag/v0.1.0
