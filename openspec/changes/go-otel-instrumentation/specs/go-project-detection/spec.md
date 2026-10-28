# Spec Delta

## Purpose

Define how dtwiz discovers Go projects, associates them with running Go processes, and splits a selected project into the services that can be instrumented.

## ADDED Requirements

### Requirement: Go project discovery

The system SHALL detect Go projects by scanning the current directory and common development directories for `go.mod` files, using the same scan scope and ignore rules as other runtimes. Automatic Go instrumentation SHALL be available only when the `GoInstrumentation` feature flag is enabled (`--go-instrumentation` or `DTWIZ_GO_INSTRUMENTATION=true`), independently of `AllRuntimes`.

#### Scenario: Go project found

- **GIVEN** a directory under the scan scope contains a `go.mod` file
- **WHEN** `dtwiz install otel` or `dtwiz setup` scans for projects
- **THEN** the directory is listed as a Go project together with its module name

#### Scenario: Go toolchain not installed

- **GIVEN** a Go project exists but no Go toolchain is available on the system
- **WHEN** project detection runs
- **THEN** the Go project is still listed
- **AND** selecting it leads to the manual-steps outcome rather than an automatic build

#### Scenario: Go flag enabled

- **GIVEN** the `GoInstrumentation` feature flag is enabled and `AllRuntimes` is not
- **WHEN** a Go project exists in the scan scope
- **THEN** it is detected and offered for automatic instrumentation

#### Scenario: Go flag not enabled

- **GIVEN** the `GoInstrumentation` feature flag is not enabled
- **WHEN** a Go project exists in the scan scope
- **THEN** no Go build, launch or file creation takes place
- **AND** Go behaves as before this change

### Requirement: Go project selection by path

When the user passes `--project <path>` and the path contains a `go.mod`, the system SHALL treat it as a Go project without scanning.

#### Scenario: Explicit Go project path

- **GIVEN** `--project ./svc` where `./svc/go.mod` exists
- **WHEN** `dtwiz install otel --project ./svc` runs
- **THEN** the Go instrumentation flow starts for that project

### Requirement: Running Go process association

The system SHALL associate running Go processes with detected Go projects, including processes started with `go run` and processes running a compiled binary, and SHALL list the running processes next to the project in the selection list.

#### Scenario: Process started with go run

- **GIVEN** a `go run` process whose working directory is inside a detected Go project
- **WHEN** projects are listed
- **THEN** the project shows that process as running

#### Scenario: Compiled binary running from a project

- **GIVEN** a compiled Go binary running with its working directory inside a detected Go project
- **WHEN** projects are listed
- **THEN** the project shows that process as running

#### Scenario: Process scan fails

- **GIVEN** the process scan returns an error or no data
- **WHEN** projects are listed
- **THEN** projects are listed without running-process information and detection does not fail

### Requirement: Service discovery from main packages

For a selected Go project the system SHALL find every directory that contains a `package main` (excluding test files, `testdata` and ignored directories), searching up to 5 directory levels below the module root. Nested modules, which have their own `go.mod`, SHALL NOT be searched and are detected as separate projects. Each found directory is one service.

#### Scenario: Single main at module root

- **GIVEN** a module whose root directory contains a `package main`
- **WHEN** services are discovered
- **THEN** exactly one service is found for the module root

#### Scenario: Several commands under cmd/

- **GIVEN** a module with `package main` in `cmd/api` and `cmd/worker`
- **WHEN** services are discovered
- **THEN** two services are found, one per directory

#### Scenario: Depth limit

- **GIVEN** a `package main` located 6 levels below the module root
- **WHEN** services are discovered
- **THEN** that directory is not included

#### Scenario: Nested module

- **GIVEN** a nested directory with its own `go.mod`
- **WHEN** services are discovered for the parent project
- **THEN** the nested module's `package main` directories are not included

#### Scenario: No main package

- **GIVEN** a Go project that is a library with no `package main`
- **WHEN** it is selected for instrumentation
- **THEN** the system reports that no service was found and nothing is changed

### Requirement: Go workspaces are not auto-instrumented

The system SHALL recognize a Go project that belongs to a Go workspace, meaning a `go.work` file exists in the project directory or in any parent directory. Such a project SHALL still be listed, but selecting it SHALL NOT build or launch anything. The system SHALL explain that Go workspaces are not supported for automatic instrumentation, install the OTel Collector as for any other project, and print the manual instrumentation steps with a link to the Dynatrace Go OpenTelemetry walkthrough.

#### Scenario: Project with go.work at its root

- **GIVEN** a project directory that contains a `go.work` file
- **WHEN** the user selects it for instrumentation
- **THEN** the preview explains that Go workspaces are not auto-instrumented and shows that the OTel Collector will be installed
- **AND** after confirmation the OTel Collector is installed and manual steps are printed
- **AND** nothing is built or launched for the application

#### Scenario: Module inside a workspace

- **GIVEN** a module whose parent directory contains a `go.work` file
- **WHEN** the user selects the module for instrumentation
- **THEN** the same message and manual steps are shown

#### Scenario: Workspace project is still listed

- **GIVEN** a Go project that belongs to a workspace
- **WHEN** projects are listed
- **THEN** it appears in the list like any other Go project

#### Scenario: Explicit path to a workspace project

- **GIVEN** `--project` points at a project that belongs to a workspace
- **WHEN** `dtwiz install otel --project <path>` runs
- **THEN** the OTel Collector is installed, the manual steps are shown and the command exits successfully

### Requirement: Service naming

Each service SHALL be named after the name of its directory. The service at the project root SHALL be named after the project folder. When two services would receive the same name, each of them SHALL be named by its path relative to the project instead.

#### Scenario: Service named after its folder

- **GIVEN** a service in `cmd/api`
- **WHEN** the plan is shown
- **THEN** the service is named `api`

#### Scenario: Root service

- **GIVEN** a project folder named `shop` whose root contains a `package main`
- **WHEN** the plan is shown
- **THEN** that service is named `shop`

#### Scenario: Name collision

- **GIVEN** services in `services/a/cmd/server` and `services/b/cmd/server`
- **WHEN** the plan is shown
- **THEN** the two services have distinct names derived from their relative paths

### Requirement: Already instrumented services are skipped

The system SHALL skip a service whose module already depends directly on the OpenTelemetry SDK or an OTLP exporter, tell the user which services were skipped and why, and point to configuring the existing exporter to use the local collector. If every service is skipped, no build or launch SHALL take place.

#### Scenario: Module already sets up its own SDK

- **GIVEN** a module whose requirements directly include the OpenTelemetry SDK
- **WHEN** the plan is built
- **THEN** its services are skipped with an explanatory message
- **AND** the message states the local collector endpoints the application should export to

#### Scenario: Mixed services in one module

- **GIVEN** a module with two services, where the module already depends on the OpenTelemetry SDK
- **WHEN** the plan is built
- **THEN** both services are skipped, because the dependency is declared per module
