# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/dynatrace-oss/dtwiz/releases/tag/v0.1.0
