# Spec Delta

## Purpose

Define how dtwiz discovers .NET projects, splits a selected project into the runnable services that can be instrumented, and associates running .NET processes with projects. This is step 1 of .NET OpenTelemetry instrumentation and applies only while the `DTWIZ_DOTNET_OTEL` feature flag is enabled.

## ADDED Requirements

### Requirement: Detection is gated by the .NET feature flag

The system SHALL detect .NET projects only while `DTWIZ_DOTNET_OTEL` (or `--dotnet-otel`) is enabled. While it is off, no .NET scan or process scan runs, .NET projects are not listed, and `--project` pointing at a .NET directory is not treated as a .NET project. While it is on, `.NET` is not offered in the list of languages without automatic detection, because .NET projects are detected; while it is off, that entry remains available.

#### Scenario: Flag off

- **GIVEN** the flag is not set
- **AND** the scan scope contains a directory with a `.csproj` file
- **WHEN** `dtwiz install otel` scans for projects
- **THEN** that directory is not listed as a .NET project

#### Scenario: Flag on through the environment

- **GIVEN** `DTWIZ_DOTNET_OTEL=true`
- **AND** the scan scope contains a directory with a `.csproj` file
- **WHEN** `dtwiz install otel` scans for projects
- **THEN** the directory is listed as a .NET project

#### Scenario: Flag on through the command line

- **GIVEN** `--dotnet-otel` is passed
- **WHEN** `dtwiz setup` scans for projects
- **THEN** .NET projects in the scan scope are listed

#### Scenario: Flag visible in status

- **GIVEN** the flag is enabled
- **WHEN** the user runs `dtwiz status`
- **THEN** the Feature Flags section lists `DTWIZ_DOTNET_OTEL` as enabled with its source

### Requirement: .NET project discovery

The system SHALL detect .NET projects by scanning the same scope and with the same ignore rules as other runtimes for solution files (`.sln`, `.slnx`) and project files (`.csproj`, `.fsproj`, `.vbproj`). Build output, package and IDE directories such as `bin`, `obj`, `packages` and `.vs` SHALL be ignored. A directory is listed once, with the solution or project files that identified it.

#### Scenario: Solution directory

- **GIVEN** a directory contains a `.sln` file and projects in subdirectories
- **WHEN** projects are scanned
- **THEN** the directory is listed as one .NET project
- **AND** its member project directories are not listed separately

#### Scenario: Standalone project without solution

- **GIVEN** a directory contains a `.csproj` and no solution exists above it within the scan scope
- **WHEN** projects are scanned
- **THEN** that directory is listed as a .NET project

#### Scenario: Solution and project in the same directory

- **GIVEN** a directory contains both a `.sln` and a `.csproj`
- **WHEN** projects are scanned
- **THEN** the directory is listed once

#### Scenario: Build output ignored

- **GIVEN** a `bin` or `obj` directory contains copies of project files
- **WHEN** projects are scanned
- **THEN** those directories are not listed

#### Scenario: .NET toolchain not installed

- **GIVEN** a .NET project exists and no `dotnet` command is available
- **WHEN** projects are scanned
- **THEN** the project is still listed

### Requirement: .NET project selection by path

When the user passes `--project <path>` and the directory contains a solution or project file, the system SHALL treat it as a .NET project without scanning, provided the flag is enabled and no other runtime's markers take precedence under the existing precedence rules.

#### Scenario: Explicit .NET project path

- **GIVEN** the flag is enabled
- **AND** `./api` contains `Api.csproj` and no other runtime's markers
- **WHEN** `dtwiz install otel --project ./api` runs
- **THEN** the .NET flow starts for that project

#### Scenario: Other runtime markers present

- **GIVEN** the flag is enabled
- **AND** `./web` contains both `package.json` and a `.csproj`
- **WHEN** `dtwiz install otel --project ./web` runs
- **THEN** the project is handled as before this change, as a Node.js project

### Requirement: Service discovery from project files

For a selected .NET project the system SHALL find every runnable service: a project whose output is an executable, a web application or a worker service. Projects listed in a solution file are used when the solution lists them and they lie inside the project directory; otherwise project files are searched up to 5 directory levels below the project directory. Libraries, test projects and projects in ignored directories SHALL NOT be services.

#### Scenario: Web application

- **GIVEN** a project file using the web SDK
- **WHEN** services are discovered
- **THEN** it is found as a service

#### Scenario: Console application

- **GIVEN** a project file declaring an executable output type
- **WHEN** services are discovered
- **THEN** it is found as a service

#### Scenario: Worker service

- **GIVEN** a project file using the worker SDK
- **WHEN** services are discovered
- **THEN** it is found as a service

#### Scenario: Library and test project

- **GIVEN** a solution with a class library and a test project
- **WHEN** services are discovered
- **THEN** neither is found as a service

#### Scenario: Solution file written on another operating system

- **GIVEN** a solution file that lists its projects with Windows-style path separators
- **WHEN** services are discovered on Linux or macOS
- **THEN** the listed projects are found

#### Scenario: Solution member outside the project directory

- **GIVEN** a solution file that lists a project outside the solution's directory
- **WHEN** services are discovered
- **THEN** that project is not included

#### Scenario: Solution without readable members

- **GIVEN** a solution file whose project list cannot be read
- **WHEN** services are discovered
- **THEN** project files below the directory are searched instead
- **AND** detection does not fail

#### Scenario: No runnable service

- **GIVEN** a .NET project consisting only of libraries
- **WHEN** it is selected for instrumentation
- **THEN** the system reports that no service was found and nothing is changed

### Requirement: Service details read from the project file

For each service the system SHALL determine its name, its target framework or frameworks, whether it targets .NET Framework, and whether it is published self-contained. A service's name SHALL be its assembly name, or the project file name when none is set, and SHALL be made unique within the project when two services would share a name.

#### Scenario: Target framework read

- **GIVEN** a project file declaring the target framework `net8.0`
- **WHEN** the service is discovered
- **THEN** its target framework is `net8.0`

#### Scenario: Several target frameworks

- **GIVEN** a project file targeting `net8.0` and `net9.0`
- **WHEN** the service is discovered
- **THEN** both frameworks are recorded

#### Scenario: Framework inherited from shared build files

- **GIVEN** the project file does not declare a target framework
- **WHEN** the service is discovered
- **THEN** the service is still listed
- **AND** its target framework is reported as unknown

#### Scenario: .NET Framework service

- **GIVEN** a project file targeting `net48`
- **WHEN** the service is discovered
- **THEN** it is flagged as .NET Framework

#### Scenario: Self-contained service

- **GIVEN** a project file that enables self-contained publishing
- **WHEN** the service is discovered
- **THEN** it is flagged as self-contained

#### Scenario: Name collision

- **GIVEN** two services in different directories with the same assembly name
- **WHEN** services are named
- **THEN** each name is made distinct using its project-relative path

### Requirement: Running .NET process recognition

The system SHALL recognize running .NET applications started through `dotnet run`, `dotnet watch`, `dotnet <application>.dll`, or as a native application-host executable, meaning an executable with a managed assembly and a runtime configuration file of the same name beside it.

#### Scenario: dotnet run

- **GIVEN** a `dotnet run` process whose working directory is inside a detected project
- **WHEN** projects are listed
- **THEN** the project shows that process as running

#### Scenario: Published or built application library

- **GIVEN** a process running `dotnet <path>/MyApp.dll` where the path lies inside a detected project's build output
- **WHEN** projects are listed
- **THEN** the project shows that process as running

#### Scenario: Application-host executable

- **GIVEN** a process running an executable from a detected project's build output, with `MyApp.dll` and `MyApp.runtimeconfig.json` beside it
- **WHEN** projects are listed
- **THEN** the project shows that process as running

#### Scenario: Application-host executable on Windows

- **GIVEN** the system is Windows
- **AND** a process runs `MyApp.exe` from a detected project's build output, with `MyApp.dll` and `MyApp.runtimeconfig.json` beside it
- **WHEN** projects are listed
- **THEN** the project shows that process as running

#### Scenario: dotnet host executable on Windows

- **GIVEN** the system is Windows
- **AND** a process runs `dotnet.exe` with an application library inside a detected project
- **WHEN** projects are listed
- **THEN** the project shows that process as running

#### Scenario: Path containing spaces on Windows

- **GIVEN** the system is Windows
- **AND** a project directory whose path contains spaces
- **AND** its application runs with the path quoted on the command line
- **WHEN** projects are listed
- **THEN** the process is recognized and associated with the project

#### Scenario: Path containing spaces on Linux or macOS

- **GIVEN** a project directory whose path contains spaces
- **AND** a `dotnet run` process has that directory as its working directory
- **WHEN** projects are listed
- **THEN** the process is recognized and associated with the project
- **AND** an application started by a path containing spaces, whose command line cannot be split reliably, is not required to be recognized

### Requirement: Association of .NET processes with projects

The system SHALL associate a recognized .NET process with a detected project when its working directory, its command line or its executable lies inside the project directory, including build output directories, and SHALL list the PIDs next to the project in the selection list. Matching SHALL ignore letter case.

#### Scenario: Matching ignores letter case

- **GIVEN** a process command line that differs from the project path only in letter case
- **WHEN** projects are listed
- **THEN** the process is still associated with the project

#### Scenario: Process outside every project

- **GIVEN** a running .NET application whose working directory, command line and executable are all outside every detected project
- **WHEN** projects are listed
- **THEN** it is not shown for any project

#### Scenario: Same behavior on Windows

- **GIVEN** the system is Windows and a .NET application runs from a detected project
- **WHEN** projects are listed
- **THEN** the project shows its PID in the same way as on Unix

#### Scenario: Windows path forms

- **GIVEN** the system is Windows
- **AND** a process command line uses a different letter case, or the other path separator, than the detected project path
- **WHEN** projects are listed
- **THEN** the process is still associated with the project

#### Scenario: Working directory cannot be read

- **GIVEN** a process whose working directory cannot be determined
- **AND** its command line or executable lies inside a detected project
- **WHEN** projects are listed
- **THEN** the process is still associated with the project

#### Scenario: Process scan fails

- **GIVEN** the process scan returns an error or no data
- **WHEN** projects are listed
- **THEN** projects are listed without running-process information and detection does not fail

### Requirement: .NET tooling and dtwiz are not reported as application processes

The system SHALL NOT report .NET tooling processes or dtwiz itself as running applications. This covers build servers, compiler servers, MSBuild nodes, package restore, test hosts, IDE language services and the `dotnet` command-line tool when it is not running an application.

#### Scenario: Build server running

- **GIVEN** a .NET build server or compiler server is running with a working directory inside a detected project
- **WHEN** projects are listed
- **THEN** it is not shown as a running process of the project

#### Scenario: dtwiz itself

- **GIVEN** the dtwiz process has the project directory as its working directory
- **WHEN** projects are listed
- **THEN** dtwiz is not shown as a running process

#### Scenario: Test host

- **GIVEN** a test host process is running tests of a detected project
- **WHEN** projects are listed
- **THEN** it is not shown as a running process of the project

### Requirement: IIS and Windows service hosts are not instrumented in step 1

IIS worker processes SHALL NOT be associated with projects by the process scan, and the system SHALL NOT register instrumentation with IIS or Windows services. A Windows service whose executable lies inside a detected project is associated like any other process.

#### Scenario: IIS worker process

- **GIVEN** an IIS worker process is running
- **WHEN** projects are listed
- **THEN** it is not shown as a running process of any project

#### Scenario: Windows service inside a project

- **GIVEN** a Windows service whose executable lies inside a detected project's build output
- **WHEN** projects are listed
- **THEN** it is shown as a running process of the project
- **AND** no instrumentation is registered for the service

### Requirement: Selection and outcome in step 1

When the flag is on, a .NET project SHALL be listed in the project selection together with other runtimes, labelled as .NET, with its path, the files that identified it and any running PIDs. Selecting it SHALL show the discovered services with their target framework and any not-supported flags, and SHALL end in the manual-steps outcome with the Dynatrace .NET walkthrough. Nothing SHALL be installed, stopped or started.

#### Scenario: Project listed with PIDs

- **GIVEN** the flag is enabled
- **AND** a detected .NET project has a running application
- **WHEN** the project selection is shown
- **THEN** the project appears with the label .NET and `← PIDs: <pid>`

#### Scenario: Selecting a project

- **GIVEN** the user selects a .NET project with two services, one targeting `net8.0` and one flagged .NET Framework
- **WHEN** the selection is confirmed
- **THEN** both services are shown with their frameworks
- **AND** the .NET Framework service is marked not supported yet
- **AND** the walkthrough link is shown

#### Scenario: No side effects

- **GIVEN** a .NET project with a running instance is selected
- **WHEN** the step-1 outcome is shown
- **THEN** the running process keeps running
- **AND** no file is created or changed
