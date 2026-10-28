# Spec Delta

## Purpose

Define how `dtwiz uninstall otel` finds and removes Go instrumentation that dtwiz added to projects, including running services and the isolated instrumentation tool.

## ADDED Requirements

### Requirement: Uninstall detects Go instrumentation artifacts

`dtwiz uninstall otel` SHALL scan the current directory and its subdirectories (not parents, using the same scope rules as other runtimes) for Go instrumentation directories created by dtwiz (kept in a Go-specific location inside a module's `.otel/` folder), and SHALL recognize them without relying on a directory name alone, so that a Node.js `.otel/` directory or an unrelated `.otel/` directory is never treated as Go instrumentation.

#### Scenario: Go .otel directory found

- **GIVEN** a module contains a Go instrumentation directory created by dtwiz
- **WHEN** `dtwiz uninstall otel` scans
- **THEN** the directory is included in the removal preview

#### Scenario: Node.js .otel directory in the same folder

- **GIVEN** a folder has a Node.js `.otel/` created by dtwiz
- **AND** Go instrumentation was also added in that folder
- **WHEN** Go uninstall removes its files
- **THEN** the Node.js files are left in place

#### Scenario: Node.js .otel directory without Go instrumentation

- **GIVEN** a folder has a Node.js `.otel/` created by dtwiz
- **AND** no Go instrumentation was ever added there
- **WHEN** `dtwiz uninstall otel` scans for Go artifacts
- **THEN** that directory is not reported as Go instrumentation

#### Scenario: Unrelated .otel directory

- **GIVEN** a `.otel/` directory not created by dtwiz
- **WHEN** `dtwiz uninstall otel` scans
- **THEN** it is not listed and is never deleted

#### Scenario: Several modules in one tree

- **GIVEN** a directory tree where two independent modules each have a Go instrumentation directory
- **WHEN** `dtwiz uninstall otel` scans
- **THEN** both directories are listed

### Requirement: Uninstall stops instrumented Go services

`dtwiz uninstall otel` SHALL detect running services launched by dtwiz from a Go instrumentation directory and SHALL list them in the preview and stop them on confirmation. Detection SHALL identify these services by the location of their executable, so that other Go processes of the same project are not stopped.

#### Scenario: Instrumented service running

- **GIVEN** a service launched by dtwiz from its Go instrumentation directory is running
- **WHEN** the preview is shown
- **THEN** the service is listed under the instrumented Go processes that will be stopped

#### Scenario: Unrelated Go process of the same project

- **GIVEN** the user runs the same project with plain `go run`, not from the instrumentation directory
- **WHEN** `dtwiz uninstall otel` runs
- **THEN** that process is not stopped

#### Scenario: Process scan fails

- **GIVEN** the process scan returns no data
- **WHEN** uninstall runs
- **THEN** the process section is omitted and directory cleanup still proceeds

### Requirement: Uninstall removes generated files

On confirmation, `dtwiz uninstall otel` SHALL delete the listed Go instrumentation directories (and the module's `.otel/` folder if it is left empty) after stopping their services, and SHALL leave project sources, `go.mod` and `go.sum` untouched. Deletion SHALL cope with executables that are still locked shortly after their process was stopped.

#### Scenario: Directories removed

- **GIVEN** the preview lists two Go instrumentation directories
- **WHEN** the user confirms
- **THEN** both directories are deleted and the project sources are unchanged

#### Scenario: Executable briefly locked

- **GIVEN** the platform is Windows and a stopped service's executable is still locked
- **WHEN** deletion runs
- **THEN** it is retried and succeeds once the lock is released

#### Scenario: Deletion fails

- **GIVEN** a directory cannot be deleted
- **WHEN** uninstall runs
- **THEN** the error names the directory and the remaining directories are still processed

### Requirement: Interrupted-build backup is preserved

If a Go instrumentation directory contains a backup of `go.mod`/`go.sum` left by an interrupted build, uninstall SHALL NOT delete it silently. It SHALL warn the user with the backup location and leave the backup in place.

#### Scenario: Backup found during uninstall

- **GIVEN** a Go instrumentation directory holds a backup of the module files
- **WHEN** `dtwiz uninstall otel` runs
- **THEN** a warning names the backup location
- **AND** the backup is not deleted

### Requirement: Isolated instrumentation tool is removed

`dtwiz uninstall otel` SHALL remove the instrumentation tool that dtwiz installed into its own tool directory, and SHALL NOT remove a tool the user installed themselves elsewhere (for example on the PATH or in `GOBIN`). The shared Go module cache SHALL NOT be modified.

#### Scenario: Tool installed by dtwiz

- **GIVEN** the tool exists in dtwiz's own tool directory
- **WHEN** the user confirms uninstall
- **THEN** the tool is deleted

#### Scenario: User's own tool

- **GIVEN** the user has the tool installed in `GOBIN`
- **WHEN** uninstall runs
- **THEN** that copy is not listed and not deleted

### Requirement: Preview, dry run and nothing-to-remove

The Go section SHALL appear in the same uninstall preview and confirmation as other runtimes. With `--dry-run` it SHALL be shown without stopping or deleting anything. When no Go artifact, process or tool is found, the Go section SHALL be omitted, and when nothing at all is found overall the existing "Nothing to remove" behavior applies.

#### Scenario: Go section in the preview

- **GIVEN** Go directories and processes exist
- **WHEN** the uninstall preview is shown
- **THEN** it has a Go section listing directories, services, and the tool

#### Scenario: Dry run

- **GIVEN** `--dry-run`
- **WHEN** `dtwiz uninstall otel` runs
- **THEN** the Go section is shown and nothing is stopped or deleted

#### Scenario: Only Go artifacts exist

- **GIVEN** no collector and no other runtime artifacts, only Go artifacts
- **WHEN** `dtwiz uninstall otel` runs
- **THEN** the Go section is shown and confirmation proceeds with cleanup

#### Scenario: Nothing to remove

- **GIVEN** no artifacts of any runtime exist
- **WHEN** `dtwiz uninstall otel` runs
- **THEN** it prints "Nothing to remove" and exits
