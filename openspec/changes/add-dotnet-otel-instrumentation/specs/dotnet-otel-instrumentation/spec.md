# Spec Delta

## Purpose

Define the overall behavior of automatic OpenTelemetry instrumentation for .NET applications in dtwiz: how it is gated, delivered in steps, installed, activated and which targets it covers. Project detection (step 1) is specified in `dotnet-project-detection`; later steps get their own detailed specs.

## ADDED Requirements

### Requirement: .NET instrumentation is gated by a feature flag

All .NET OpenTelemetry behavior SHALL be available only while the `DTWIZ_DOTNET_OTEL` feature flag (CLI flag `--dotnet-otel`) is enabled. It is off by default. While it is off, dtwiz behaves exactly as it did before .NET support was added.

#### Scenario: Flag off

- **GIVEN** neither `DTWIZ_DOTNET_OTEL` nor `--dotnet-otel` is set
- **WHEN** `dtwiz install otel` or `dtwiz setup` runs in a directory containing a .NET project
- **THEN** the project is not detected as a .NET project and no .NET-specific output appears

#### Scenario: Flag on

- **GIVEN** `DTWIZ_DOTNET_OTEL=true` is set
- **WHEN** `dtwiz install otel` or `dtwiz setup` runs in a directory containing a .NET project
- **THEN** the project is offered according to the `dotnet-project-detection` capability

### Requirement: Delivery in steps behind the flag

.NET instrumentation SHALL be delivered in steps: (1) project detection, (2) prerequisites and plan preview, (3) instrumentation installation, (4) launch of the instrumented services, (5) uninstall and ingest watch. A step that has not shipped SHALL end in the manual-steps outcome for .NET and SHALL NOT change the machine.

#### Scenario: Only detection has shipped

- **GIVEN** the flag is enabled and only step 1 has shipped
- **WHEN** the user selects a detected .NET project
- **THEN** the discovered services are shown
- **AND** the user is pointed to the Dynatrace .NET walkthrough
- **AND** nothing is downloaded, installed, stopped or started

### Requirement: No changes to the user's project

Instrumenting a .NET project SHALL NOT modify its source files, project files, solution files, configuration files or package references. The instrumentation SHALL be activated for the launched process only. Build output produced by building or running the project is not a modification.

#### Scenario: Project left untouched

- **GIVEN** a .NET project was instrumented
- **WHEN** the working tree is compared with its state before instrumentation, ignoring build output directories
- **THEN** no file inside the project differs

### Requirement: Isolated installation

The instrumentation files SHALL be installed into a directory owned by dtwiz under the user's home directory, not into system-wide locations, the global .NET installation, the global assembly cache, or the project. Installing SHALL NOT require elevated privileges. An override location MAY be supplied.

#### Scenario: Default location

- **GIVEN** no override location is configured
- **WHEN** the instrumentation is installed
- **THEN** its files are placed in a dtwiz-owned directory in the user's home directory
- **AND** no elevation prompt is shown

#### Scenario: Windows

- **GIVEN** the system is Windows
- **WHEN** the instrumentation is installed
- **THEN** its files are placed in a dtwiz-owned directory in the user's profile, not in Program Files
- **AND** no administrator rights are required

### Requirement: Latest upstream release, per platform

The system SHALL install the latest release of the upstream OpenTelemetry .NET automatic instrumentation, choosing the archive that matches the operating system, C library (glibc or musl on Linux) and processor architecture. It SHALL reject unsupported combinations before downloading, naming the supported ones.

#### Scenario: Matching archive chosen

- **GIVEN** a Linux system with the musl C library on arm64
- **WHEN** the instrumentation is installed
- **THEN** the archive for Linux musl on arm64 is downloaded

#### Scenario: Unsupported platform

- **GIVEN** a Windows system on arm64 or a macOS system on an Intel processor
- **WHEN** installation is planned
- **THEN** the user is told that the platform is not supported by the upstream release
- **AND** the manual-steps outcome is offered
- **AND** nothing is downloaded

#### Scenario: Release cannot be reached

- **GIVEN** the upstream release cannot be reached
- **AND** an earlier installation exists in the dtwiz directory
- **WHEN** installation is planned
- **THEN** the user is offered to reuse the existing installation, showing its version
- **AND** otherwise the user gets an actionable error

### Requirement: Integrity verification of the download

The system SHALL verify the downloaded archive against the checksum published with the same release before extracting it, and SHALL abort on a mismatch or a missing checksum. When the GitHub CLI is installed and usable, the system SHALL additionally verify the release attestation and SHALL abort when it fails. A failed stronger check SHALL NOT fall back to a weaker one.

#### Scenario: Checksum mismatch

- **GIVEN** the downloaded archive does not match the published checksum
- **WHEN** verification runs
- **THEN** installation is aborted with an error naming the archive
- **AND** nothing is extracted

#### Scenario: GitHub CLI missing or not signed in

- **GIVEN** the GitHub CLI is not installed or is not authenticated
- **WHEN** verification runs
- **THEN** only the checksum is verified
- **AND** the missing attestation check does not produce an error

#### Scenario: Attestation fails

- **GIVEN** the GitHub CLI is usable
- **AND** attestation verification fails
- **WHEN** verification runs
- **THEN** installation is aborted even though the checksum matched

### Requirement: Activation through the launched process's environment only

The system SHALL activate the instrumentation by setting environment variables on the launched application process. The exporter settings (endpoint, protocol and delta metrics temporality) SHALL be the same ones other runtimes use, which send telemetry to the OpenTelemetry Collector that dtwiz runs locally. No credential SHALL be written to disk or shown in the preview in clear text.

#### Scenario: Same exporter settings as other runtimes

- **GIVEN** a .NET service is planned for launch
- **WHEN** the plan preview is shown
- **THEN** its exporter endpoint, protocol and metrics temporality equal those used for the other runtimes

#### Scenario: Token handling

- **GIVEN** a .NET service is launched with instrumentation
- **WHEN** the plan preview is shown and files are written
- **THEN** the token appears neither in clear text in the preview nor in any generated file

#### Scenario: Service name per service

- **GIVEN** a project with two runnable services
- **WHEN** they are launched
- **THEN** each service receives its own distinct service name

### Requirement: Supported application targets

The system SHALL instrument framework-dependent applications on modern .NET. Self-contained and native-AOT applications SHALL be reported as not supported by this method. .NET Framework applications, IIS-hosted applications and Windows services SHALL be reported as not supported initially.

#### Scenario: Self-contained application

- **GIVEN** a service published as self-contained
- **WHEN** the plan is built
- **THEN** the service is marked as not supported with the reason
- **AND** the manual-steps outcome is offered for it

#### Scenario: .NET Framework application

- **GIVEN** a service targeting .NET Framework
- **WHEN** the plan is built
- **THEN** the service is marked as not supported yet with the reason

### Requirement: Same plan, confirmation and watch flow as other runtimes

.NET instrumentation SHALL show a plan preview, ask for confirmation before changing anything, stop already running instances of the selected services only after confirmation, and finish with the ingest watch, as the other runtimes do. `--dry-run` and `--yes` SHALL behave as they do for other runtimes.

#### Scenario: Declined confirmation

- **GIVEN** the plan preview is shown
- **WHEN** the user declines
- **THEN** nothing is installed, stopped or started
- **AND** the command exits cleanly
