# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `--verbose`/`-v` flag (count-based, like `dtctl`): `-v` emits a compact `METHOD URL → STATUS (time)` line per HTTP request; `-vv` adds full headers and bodies matching dtctl's `===> REQUEST <===` / `===> RESPONSE <===` format
- `--debug` now promoted to full verbosity level 2 (equivalent to `-vv`) and installs the HTTP logging transport on startup; sensitive headers (`Authorization`, `x-api-key`, `cookie`) are always redacted as `[REDACTED]`
- `logger.Verbosity() int` helper; `logger.NewLoggingTransport()` HTTP round-tripper wrapping any base transport

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

[Unreleased]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/dynatrace-oss/dtwiz/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/dynatrace-oss/dtwiz/releases/tag/v0.1.0
