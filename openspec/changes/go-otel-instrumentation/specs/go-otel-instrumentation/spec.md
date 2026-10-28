# Spec Delta

## Purpose

Define how dtwiz instruments a selected Go project with OpenTelemetry without source changes, builds and launches its services, and falls back when automatic instrumentation is not possible.

## ADDED Requirements

### Requirement: Compile-time instrumentation is the preferred method

For each service, the system SHALL first try compile-time instrumentation, which produces server and client spans for supported libraries, including third-party dependencies, without modifying project sources. It SHALL be used only when the installed Go toolchain is version 1.25 or newer. The instrumentation tool version SHALL be a fixed stable release, not "latest".

#### Scenario: Supported toolchain

- **GIVEN** Go 1.25 or newer is installed
- **WHEN** the plan is built for a Go project
- **THEN** each service is planned for compile-time instrumentation

#### Scenario: Older toolchain

- **GIVEN** Go older than 1.25 is installed
- **WHEN** the plan is built
- **THEN** each service is planned for the client-only fallback
- **AND** the plan states why compile-time instrumentation is not used

#### Scenario: Instrumentation tool not yet installed

- **GIVEN** the instrumentation tool is neither on the PATH nor in dtwiz's own tool directory
- **WHEN** the plan is confirmed
- **THEN** the pinned version is installed into dtwiz's own tool directory, not into the user's `GOBIN` or `GOPATH`

#### Scenario: Instrumentation tool already on the PATH

- **GIVEN** an instrumentation tool is already on the PATH
- **WHEN** a service is built
- **THEN** that tool is used and nothing is installed

### Requirement: Per-service fallback to client-only instrumentation

If compile-time instrumentation cannot be used for a service (tool installation fails, or the instrumented build fails), the system SHALL fall back to client-only instrumentation for that service only, tell the user what happened, and continue with the remaining services. Client-only instrumentation emits traces, metrics and logs to the configured endpoint and traces outgoing HTTP calls; incoming requests are not traced automatically.

#### Scenario: Tool installation fails

- **GIVEN** compile-time instrumentation is planned
- **WHEN** installing the tool fails
- **THEN** all services fall back to client-only instrumentation
- **AND** the user is told that only outgoing calls will be traced

#### Scenario: One service fails to build with the tool

- **GIVEN** two services, one of which cannot be built by the instrumentation tool
- **WHEN** the services are built
- **THEN** the failing service is rebuilt with client-only instrumentation
- **AND** the other service keeps compile-time instrumentation

#### Scenario: Fallback is visible in the final summary

- **GIVEN** at least one service used the fallback
- **WHEN** the run finishes
- **THEN** the summary names the services that have outgoing-call tracing only

### Requirement: Manual steps when automatic instrumentation is not possible

When a service cannot be built by either method, no Go toolchain is available, or the project is a Go workspace, the system SHALL NOT build or launch it. The plan SHALL present manual instrumentation as the chosen method, and a manual outcome SHALL NOT be an error: the command exits successfully.

#### Scenario: No Go toolchain

- **GIVEN** the selected project has no usable Go toolchain
- **WHEN** the user confirms the plan
- **THEN** the OTel Collector is installed
- **AND** manual steps are printed and nothing is built

#### Scenario: Both methods fail for a service

- **GIVEN** a service that fails to build with both methods
- **WHEN** the build step runs
- **THEN** the build error is shown
- **AND** manual steps are printed for that service
- **AND** other services continue

#### Scenario: Manual outcome is shown in the preview

- **GIVEN** a project that will be handled with manual steps
- **WHEN** the plan preview is shown
- **THEN** it states that the application is not instrumented automatically and why
- **AND** it shows that the OTel Collector will still be installed

#### Scenario: Manual outcome exits successfully

- **GIVEN** the user confirmed a plan whose outcome is manual steps
- **WHEN** the collector installed successfully and the manual steps were printed
- **THEN** the command exits with a success status

### Requirement: Manual steps content

Manual steps SHALL be printed after the collector is installed and SHALL list the packages to add, the environment variables to set (pointing at the local collector, with no Dynatrace token), and a link to the Dynatrace Go OpenTelemetry walkthrough.

#### Scenario: Manual steps point at the local collector

- **GIVEN** a manual outcome and a local collector on port 4318
- **WHEN** the manual steps are printed
- **THEN** the endpoint shown is `http://127.0.0.1:4318`
- **AND** no Dynatrace token or authorization header is shown
- **AND** the walkthrough link is included

### Requirement: Collector installation does not depend on the instrumentation outcome

For a selected Go project the system SHALL install the OTel Collector as part of the same confirmed run regardless of whether the project is instrumented automatically, partially or only manually, so that telemetry sent by the application reaches Dynatrace as soon as it is instrumented.

#### Scenario: Collector and manual steps together

- **GIVEN** a Go project that cannot be instrumented automatically
- **WHEN** the user confirms the plan
- **THEN** the OTel Collector is installed and running
- **AND** the manual steps reference the collector's endpoint

#### Scenario: Collector installation fails

- **GIVEN** a Go project that falls back to manual steps
- **WHEN** installing the collector fails
- **THEN** the failure is reported as for any other runtime
- **AND** manual steps are not presented as ready to use

### Requirement: Project sources and module files are unchanged

The system SHALL NOT modify Go source files in the project. All generated files SHALL live in a `.otel/` directory inside each module. If a build step modifies the module's `go.mod` or `go.sum`, they SHALL be restored to their original content afterwards, including when the build fails or the run is interrupted by the user.

#### Scenario: Build succeeds

- **GIVEN** a module with an existing `go.mod` and `go.sum`
- **WHEN** a service is built with compile-time instrumentation
- **THEN** after the build `go.mod` and `go.sum` are byte-identical to before
- **AND** no `go.sum` exists afterwards if none existed before

#### Scenario: Build is interrupted

- **GIVEN** a build with compile-time instrumentation is running
- **WHEN** the user presses Ctrl+C
- **THEN** `go.mod` and `go.sum` are restored before dtwiz exits

#### Scenario: Module files already have uncommitted changes

- **GIVEN** the module's `go.mod` or `go.sum` has uncommitted changes in version control
- **WHEN** a service would be built with compile-time instrumentation
- **THEN** that method is not used for the module
- **AND** the service uses the client-only fallback instead

#### Scenario: Leftover backup from an interrupted run

- **GIVEN** a backup of the module files from an earlier interrupted run exists and differs from the current files
- **WHEN** a new run starts for that module
- **THEN** the run stops for that module with a message naming the backup location
- **AND** nothing is overwritten

### Requirement: Plan preview and confirmation

Before building anything, the system SHALL show the project, the method per service, each service name with its directory, and the tool or packages that will be installed, then ask a single confirmation (default yes). With `--dry-run` the preview is shown and nothing is built or launched. With `--yes` the confirmation is skipped.

#### Scenario: Preview with compile-time instrumentation

- **GIVEN** two services planned for compile-time instrumentation
- **WHEN** the preview is shown
- **THEN** it lists both services with their directories
- **AND** it states that project sources are not modified
- **AND** it mentions that a network connection is needed for the first build

#### Scenario: Declined

- **GIVEN** the plan is shown
- **WHEN** the user answers no
- **THEN** nothing is built or launched and the command exits cleanly

#### Scenario: Dry run

- **GIVEN** `--dry-run`
- **WHEN** the command runs
- **THEN** the preview is shown and no file or process is created

### Requirement: Build and launch of each service

After confirmation the system SHALL stop the project's previously running Go processes that it is about to replace, build each service into the module's `.otel/` directory, and launch it with the environment described below. Launched services SHALL be reported with their names and process IDs, and a service that exits during start-up SHALL be reported as failed. The command SHALL then wait for the first telemetry to arrive, as for other runtimes.

#### Scenario: Previously running process is replaced

- **GIVEN** a Go process of the project is already running
- **WHEN** the plan is confirmed
- **THEN** that process is stopped before the instrumented service is launched

#### Scenario: Service fails at start-up

- **GIVEN** a built service that exits immediately
- **WHEN** the start-up check runs
- **THEN** it is reported as failed with a pointer to its output
- **AND** other services are not affected

#### Scenario: All services failed

- **GIVEN** every launched service failed to start
- **WHEN** the run finishes
- **THEN** the command exits with a non-success status

#### Scenario: Telemetry arrives

- **GIVEN** at least one service started
- **WHEN** it sends data to Dynatrace
- **THEN** the post-install ingest watch reports it

### Requirement: Dynatrace-ready environment for launched services

Each launched service SHALL receive its service name, the local collector as OTLP endpoint with HTTP protocol, and delta metric temporality. The service SHALL NOT receive a Dynatrace token; the collector holds credentials.

#### Scenario: Service environment

- **GIVEN** a service named `api` and a local collector on port 4318
- **WHEN** the service is launched
- **THEN** its service name is `api`
- **AND** its OTLP endpoint is `http://127.0.0.1:4318`
- **AND** no Dynatrace token is present in its environment

#### Scenario: Metric temporality

- **GIVEN** any launched service
- **WHEN** it exports metrics
- **THEN** delta temporality is requested

### Requirement: Cross-platform behavior

Go instrumentation SHALL behave the same on Linux, macOS and Windows: detection, tier selection, build, launch, interruption handling and uninstall. Executable names SHALL carry the platform's extension, and files of a running service SHALL be handled so that a rebuild or removal does not fail because the executable is in use.

#### Scenario: Windows executable

- **GIVEN** the platform is Windows
- **WHEN** a service named `api` is built
- **THEN** the output executable is `api.exe` under the module's `.otel/` directory

#### Scenario: Rebuild while an old executable is running on Windows

- **GIVEN** the platform is Windows and the previous executable of the service is still running
- **WHEN** the plan is confirmed
- **THEN** the old process is stopped before the executable is replaced

#### Scenario: Paths with spaces

- **GIVEN** a project path or toolchain path that contains spaces
- **WHEN** a service is built
- **THEN** the build succeeds

### Requirement: Failure messages carry context

Build and installation failures SHALL show the failing command's output and the platform (OS and architecture) and SHALL distinguish user-fixable problems (no network, missing module) from tool errors.

#### Scenario: Network unavailable

- **GIVEN** no network connection during the first build
- **WHEN** the build fails to download modules
- **THEN** the message says the failure is connectivity-related
