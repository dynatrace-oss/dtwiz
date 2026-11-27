# Spec Delta

## Purpose

Define step 2 of .NET OpenTelemetry auto-instrumentation: the prerequisite check, the plan shown for a selected .NET project, and the preview, confirmation and execution flow that follows. Detection is specified in `dotnet-project-detection`; installing the instrumentation is specified in `dotnet-instrumentation-install`.

## ADDED Requirements

### Requirement: Plan is offered for a selected .NET project

While the `DTWIZ_DOTNET_OTEL` flag is enabled, selecting a detected .NET project SHALL build an instrumentation plan for it and offer that plan through the same preview and confirmation flow as the other runtimes. With the flag off, nothing .NET-specific SHALL appear.

#### Scenario: Flag on, project selected

- **GIVEN** `DTWIZ_DOTNET_OTEL=true`
- **AND** a detected .NET project with one runnable service
- **WHEN** the user selects the project
- **THEN** a plan preview for that project is shown instead of the step-1 manual-steps outcome

#### Scenario: Flag off

- **GIVEN** the flag is not enabled
- **WHEN** `dtwiz install otel` runs in a directory containing a .NET project
- **THEN** no .NET plan is built and behavior is unchanged

### Requirement: Prerequisite check

Before showing the plan, the system SHALL check that the `dotnet` command is available and that, for each supported service, an installed .NET runtime of the service's target framework major version (or a newer major version, when roll-forward applies) is present. A failed check SHALL end in the manual-steps outcome with an actionable message and SHALL NOT download or change anything.

#### Scenario: dotnet missing

- **GIVEN** `dotnet` is not on the executable search path
- **WHEN** the plan is built
- **THEN** the user is told that .NET is required and where to get it
- **AND** the manual-steps outcome with the Dynatrace .NET walkthrough is offered
- **AND** nothing is downloaded

#### Scenario: Runtime for the target framework missing

- **GIVEN** a service targets `net8.0`
- **AND** no .NET 8 (or newer) runtime is installed
- **WHEN** the plan is built
- **THEN** the service is marked "requires .NET 8 runtime, not installed"
- **AND** it is not part of the actions of the plan

#### Scenario: Unknown target framework

- **GIVEN** a service whose target framework could not be read from its project file
- **WHEN** the plan is built
- **THEN** the service is planned with a note that its framework is unknown
- **AND** the runtime check does not block it

#### Scenario: Prerequisites met

- **GIVEN** `dotnet` is available and the runtime for each service's target framework is installed
- **WHEN** the plan is built
- **THEN** no prerequisite warning is shown

### Requirement: Plan lists the steps and how each service is instrumented

The preview SHALL list, in this order and as commands or short steps: download (or reuse) and verify the instrumentation, stop the project's running processes, build the services that need it, and launch each service that can be instrumented with its service name. The activation environment SHALL be shown once for all services. Services that cannot be instrumented SHALL be named with their reason and no launch step. Other explanations SHALL be left out.

#### Scenario: Two services with running processes

- **GIVEN** a project with services `Frontend` and `Order` and two running processes of it
- **WHEN** the plan preview is shown
- **THEN** it lists downloading and verifying the instrumentation with its install location
- **AND** then stopping those processes
- **AND** then the build steps, if any
- **AND** then one launch command per service together with the service name that service will report
- **AND** the activation environment appears once, not per service

#### Scenario: Stop comes after verification and before the build

- **GIVEN** a service that needs a build and has a running process
- **WHEN** the plan preview is shown
- **THEN** the stop step is listed after verification and before the build step

#### Scenario: Mixed supported and unsupported services

- **GIVEN** a project with a `net8.0` web service and a service targeting `net48`
- **WHEN** the plan preview is shown
- **THEN** the web service has a launch step
- **AND** the `net48` service is named as skipped with the reason and has no launch step

#### Scenario: No running processes

- **GIVEN** no process of the project is running
- **WHEN** the plan preview is shown
- **THEN** no stop step is listed

#### Scenario: Instrumentation already installed

- **GIVEN** a usable instrumentation installation exists in the dtwiz directory
- **WHEN** the plan preview is shown
- **THEN** a single step says the installed version in that directory is used
- **AND** no download or verification step is listed

#### Scenario: Instrumentation not installed

- **GIVEN** no installation exists
- **WHEN** the plan preview is shown
- **THEN** the download step names the archive for this platform and the directory it is installed to
- **AND** a verification step follows it

### Requirement: Services are built only when needed

A service SHALL be built only when it has no usable built library or when its sources are newer than that library. A built library is usable when it lies in the project's build output for one of the service's target frameworks and its runtime configuration file is beside it. The build SHALL run without the instrumentation environment. The launch step SHALL use the existing library when no build is needed.

#### Scenario: Existing build is reused

- **GIVEN** a service whose built library exists and no source of the project is newer
- **WHEN** the plan preview is shown
- **THEN** the service has no build step
- **AND** its launch step uses the existing library and says it is an existing build

#### Scenario: No build yet

- **GIVEN** a service without a built library
- **WHEN** the plan preview is shown
- **THEN** a build step is listed for it
- **AND** its launch step names the library the build produces

#### Scenario: Sources changed since the last build

- **GIVEN** a service whose built library exists but a source file of the project is newer
- **WHEN** the plan preview is shown
- **THEN** a build step is listed for it and says that sources changed since the last build

#### Scenario: Several build configurations

- **GIVEN** built libraries exist in more than one configuration
- **WHEN** the plan is built
- **THEN** the most recently built one is used

#### Scenario: Incomplete build output

- **GIVEN** a built library without its runtime configuration file
- **WHEN** the plan is built
- **THEN** the service is treated as not built

#### Scenario: Build without instrumentation

- **GIVEN** a build step is listed
- **WHEN** the plan preview is shown
- **THEN** the build is labelled as running without instrumentation

### Requirement: Nothing supported means manual steps

When no service of the selected project can be instrumented, the system SHALL NOT offer the install action and SHALL end in the manual-steps outcome, naming the reason per service.

#### Scenario: All services unsupported

- **GIVEN** every service of the project is self-contained or targets .NET Framework
- **WHEN** the project is selected
- **THEN** the services are listed with their reasons
- **AND** the manual-steps outcome is shown
- **AND** nothing is downloaded

#### Scenario: Library-only project

- **GIVEN** a project without any runnable service
- **WHEN** the project is selected
- **THEN** the user is told no runnable .NET service was found
- **AND** the manual-steps outcome is shown

### Requirement: Preview without side effects

Showing the plan SHALL NOT download, extract, write, stop or start anything. The preview SHALL be compact, and no credential SHALL appear in it in clear text.

#### Scenario: Preview only

- **GIVEN** a plan is built
- **WHEN** the preview is displayed
- **THEN** no file is created or changed and no process is stopped or started

#### Scenario: Dry run

- **GIVEN** `--dry-run` is set
- **WHEN** the plan preview has been shown
- **THEN** the command ends without the confirmation prompt, without downloading and without changing anything

### Requirement: Confirmation before execution

The system SHALL ask for confirmation after the preview, defaulting to yes. `--yes` SHALL skip the prompt. A declined confirmation SHALL exit cleanly without changes.

#### Scenario: Confirmed

- **GIVEN** the preview is shown
- **WHEN** the user confirms or presses Enter
- **THEN** the plan is executed

#### Scenario: Declined

- **GIVEN** the preview is shown
- **WHEN** the user declines
- **THEN** nothing is downloaded, extracted, stopped or started
- **AND** the command exits without an error

#### Scenario: Auto-confirm

- **GIVEN** `--yes` is set
- **WHEN** the preview has been shown
- **THEN** the plan runs without a prompt

### Requirement: Outcome until launching ships

Executing the plan SHALL install the instrumentation as specified in `dotnet-instrumentation-install` and SHALL then end in the manual-steps outcome. Until launching ships it SHALL NOT stop running processes, build or start any service, SHALL NOT modify the project, and the preview SHALL say once that stopping, building and launching are not performed yet.

#### Scenario: Install succeeded

- **GIVEN** the installation completed
- **WHEN** execution ends
- **THEN** the user sees where the instrumentation was installed
- **AND** the manual-steps outcome is shown
- **AND** running instances of the project keep running

#### Scenario: Preview states what is not performed yet

- **GIVEN** launching is not available yet
- **WHEN** the plan preview is shown
- **THEN** it states once that stopping, building and launching the services are not performed yet

#### Scenario: Install failed

- **GIVEN** installation fails
- **WHEN** execution ends
- **THEN** the command returns an error naming the failed stage
- **AND** the project and running processes are unchanged

#### Scenario: Interrupted

- **GIVEN** the user presses Ctrl+C during execution
- **WHEN** the interruption is handled
- **THEN** the operation stops promptly
- **AND** no partially installed instrumentation is left behind

### Requirement: Walkthrough link in the outcome

The manual-steps outcome SHALL show "Instrument your .NET app" as a link to the Dynatrace .NET walkthrough: right after the statement that automatic launch is not available yet, and after the reason of a manual plan. Where the terminal cannot show links, the address SHALL be shown below the text. The link SHALL be shown once in the flow.

#### Scenario: After a successful install

- **GIVEN** the instrumentation was installed or found installed
- **WHEN** execution ends
- **THEN** the statement that automatic launch is not available yet is followed directly by "Instrument your .NET app"
- **AND** that text is a link to the walkthrough

#### Scenario: Terminal without link support

- **GIVEN** the terminal cannot show links
- **WHEN** the outcome is shown
- **THEN** the walkthrough address is shown on the line below "Instrument your .NET app"

#### Scenario: After a manual plan

- **GIVEN** nothing can be instrumented automatically
- **WHEN** execution ends
- **THEN** the reason is followed by the same link

#### Scenario: Not repeated

- **GIVEN** the walkthrough link was shown
- **WHEN** the ingest watch ends its output
- **THEN** the link is not shown again
