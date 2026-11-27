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

### Requirement: Plan contents per service

The plan SHALL list every discovered service of the selected project with its name, target framework and whether it will be instrumented. Services that cannot be instrumented, namely .NET Framework, self-contained, native-AOT, or missing a required runtime, SHALL be listed with the reason and SHALL NOT be acted on. The plan SHALL include the instrumentation version and install location it will use and whether it will be downloaded or reused.

#### Scenario: Mixed supported and unsupported services

- **GIVEN** a project with a `net8.0` web service and a service targeting `net48`
- **WHEN** the plan preview is shown
- **THEN** the web service is listed as instrumented
- **AND** the `net48` service is listed as not supported yet with the reason

#### Scenario: Instrumentation already installed

- **GIVEN** a usable instrumentation installation exists in the dtwiz directory
- **WHEN** the plan preview is shown
- **THEN** it shows the installed version and that it will be reused, or that it will be updated when a newer release is available

#### Scenario: Instrumentation not installed

- **GIVEN** no installation exists
- **WHEN** the plan preview is shown
- **THEN** it shows that the latest release will be downloaded and the directory it will be installed to

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

Executing the plan SHALL install the instrumentation as specified in `dotnet-instrumentation-install` and SHALL then end in the manual-steps outcome, stating that launching the instrumented service automatically is not available yet. It SHALL NOT stop running processes or start any service until the launch step ships, and SHALL NOT modify the project.

#### Scenario: Install succeeded

- **GIVEN** the installation completed
- **WHEN** execution ends
- **THEN** the user sees where the instrumentation was installed
- **AND** the manual-steps outcome is shown
- **AND** running instances of the project keep running

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
