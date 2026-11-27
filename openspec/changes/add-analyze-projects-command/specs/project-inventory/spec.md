# Spec Delta

## Purpose

`dtwiz analyze projects` lists the application projects on this machine that dtwiz can instrument with OpenTelemetry, as a read-only, non-interactive command whose output users and agents can turn directly into an install command.

## ADDED Requirements

### Requirement: Project listing is read-only and non-interactive

`dtwiz analyze projects` SHALL list detected projects without requiring Dynatrace credentials, without contacting the Dynatrace tenant, without prompting, and without changing anything on the system.

#### Scenario: Listing without credentials

- **GIVEN** neither `DT_ENVIRONMENT` nor `DT_PLATFORM_TOKEN` is set and no credential flags are passed
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** the detected projects are listed and the command exits with code 0

#### Scenario: No prompt from a directory outside the home tree

- **GIVEN** the working directory lies outside the home-directory tree and stdin is a TTY
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** no prompt is shown and the command completes without reading stdin

---

### Requirement: Bare analyze is unchanged

Running `dtwiz analyze` without a subcommand SHALL behave as before this change, including its `--json` output. The System Analysis block SHALL NOT be printed by `dtwiz analyze projects`.

#### Scenario: Bare analyze

- **GIVEN** dtwiz with the `analyze projects` subcommand
- **WHEN** the user runs `dtwiz analyze` or `dtwiz analyze --json`
- **THEN** the output is the System Analysis block or its JSON form, as before

#### Scenario: Subcommand does not print the System Analysis block

- **GIVEN** any system
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** the System Analysis block is not printed

---

### Requirement: Scan root defaults to the working directory

`dtwiz analyze projects` SHALL scan exactly one root: the directory given by `--dir <path>`, or the working directory when `--dir` is absent. It SHALL NOT add the home directory or the bundled examples directory as further roots. A relative `--dir` SHALL be resolved against the working directory.

#### Scenario: Default root

- **GIVEN** the working directory is `~/work/shop`
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** only `~/work/shop` and its subdirectories are scanned, and the output names `~/work/shop` as the scanned root

#### Scenario: Explicit root

- **GIVEN** the working directory is `/tmp`
- **WHEN** the user runs `dtwiz analyze projects --dir ~/work/shop`
- **THEN** only `~/work/shop` and its subdirectories are scanned

#### Scenario: Home directory not added outside the home tree

- **GIVEN** the working directory is `/opt/app`, outside the home-directory tree, and the home directory contains projects
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** projects in the home directory are not listed

#### Scenario: Bundled examples not added

- **GIVEN** `~/.dtwiz/examples/schnitzel/` exists and the scan root does not contain it
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** the schnitzel project is not listed

#### Scenario: Bundled examples listed when they are the scan root

- **GIVEN** `~/.dtwiz/examples/schnitzel/` exists
- **WHEN** the user runs `dtwiz analyze projects --dir ~/.dtwiz/examples`
- **THEN** the schnitzel project is listed

#### Scenario: Directory does not exist

- **GIVEN** `/nope` does not exist
- **WHEN** the user runs `dtwiz analyze projects --dir /nope`
- **THEN** the command prints `directory not found: /nope` and exits with code 1

---

### Requirement: Runtime identifiers form a fixed set

Runtimes SHALL be identified by exactly these values: `python`, `java`, `nodejs`, `go`, and, only while the `DotNetOtel` feature flag is on, `dotnet`. Matching SHALL be exact and case-sensitive; no aliases SHALL be accepted. The command help SHALL document the set, and any error about an invalid value SHALL list it.

#### Scenario: Unknown identifier

- **GIVEN** the `DotNetOtel` feature flag is off
- **WHEN** the user runs `dtwiz analyze projects --runtime golang`
- **THEN** the command prints `unknown runtime "golang" (valid: python, java, nodejs, go)` and exits with code 1

#### Scenario: Wrong case is rejected

- **GIVEN** any system
- **WHEN** the user runs `dtwiz analyze projects --runtime JAVA`
- **THEN** the command reports `JAVA` as an unknown runtime, lists the valid identifiers, and exits with code 1

#### Scenario: Help documents the set

- **GIVEN** the `DotNetOtel` feature flag is off
- **WHEN** the user runs `dtwiz analyze projects --help`
- **THEN** the help text lists `python, java, nodejs, go` as the accepted `--runtime` values

#### Scenario: Help documents the set with the feature flag on

- **GIVEN** the `DotNetOtel` feature flag is on, set either by `DTWIZ_DOTNET_OTEL=true` or by passing `--dotnet-otel` together with `--help`
- **WHEN** the user runs `dtwiz analyze projects --help`
- **THEN** the help text lists `python, java, nodejs, go, dotnet` as the accepted `--runtime` values

---

### Requirement: Runtime filter

`--runtime` SHALL accept a comma-separated list of runtime identifiers. When given, only projects of the listed runtimes SHALL be listed, and only the listed runtimes SHALL be reported in the runtime availability information. When absent, all runtimes SHALL be included. Duplicate identifiers SHALL be treated as one.

#### Scenario: Filter to two runtimes

- **GIVEN** the scan root contains Java, Go, and Node.js projects
- **WHEN** the user runs `dtwiz analyze projects --runtime java,go`
- **THEN** only the Java and Go projects are listed

#### Scenario: Empty element in the list

- **GIVEN** any system
- **WHEN** the user runs `dtwiz analyze projects --runtime java,`
- **THEN** the command reports the empty value as an unknown runtime, lists the valid identifiers, and exits with code 1

---

### Requirement: .NET exists only while the DotNetOtel feature flag is on

While the `DotNetOtel` feature flag (`--dotnet-otel` / `DTWIZ_DOTNET_OTEL`) is off, the command SHALL behave as if .NET support did not exist: no .NET projects are listed, `dotnet` is not reported in the runtime availability information, not shown in help or error messages, and rejected as an unknown runtime like any other unsupported value. While the flag is on, .NET projects SHALL be listed.

#### Scenario: Flag off, no filter

- **GIVEN** the `DotNetOtel` feature flag is off and the scan root contains a .NET project
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** the .NET project is not listed and `dotnet` is not reported in the runtime availability information

#### Scenario: Flag off, dotnet requested

- **GIVEN** the `DotNetOtel` feature flag is off
- **WHEN** the user runs `dtwiz analyze projects --runtime dotnet`
- **THEN** the command prints `unknown runtime "dotnet" (valid: python, java, nodejs, go)` and exits with code 1
- **AND** the output does not mention the feature flag

#### Scenario: Flag on

- **GIVEN** the `DotNetOtel` feature flag is on and the scan root contains a .NET project
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** the .NET project is listed and marked as ending in manual steps

---

### Requirement: Project status describes the install outcome

Each listed project SHALL carry a status that describes what installing it with `dtwiz install otel` does, not how mature a runtime's support is: `instrumentable` when the install instruments the application, `manual_steps` when it ends in the manual-steps outcome, and `runtime_unavailable` when the runtime's toolchain is not usable on this machine.

#### Scenario: Python project with a usable interpreter

- **GIVEN** a usable `python3` is on PATH and the scan root contains a Python project
- **WHEN** the user runs `dtwiz analyze projects --json`
- **THEN** the project's `status` is `instrumentable`

#### Scenario: .NET project

- **GIVEN** the `DotNetOtel` feature flag is on and the scan root contains a .NET project
- **WHEN** the user runs `dtwiz analyze projects --json`
- **THEN** the project's `status` is `manual_steps`, because `dtwiz install otel` does not yet launch instrumented .NET services and ends in the manual-steps outcome

---

### Requirement: Projects with an unavailable runtime are listed

A project whose runtime toolchain is not usable on this machine SHALL still be listed, marked as runtime unavailable together with the reason (for example, the binary is not on PATH). Runtime availability SHALL be judged the same way `dtwiz install otel` judges it. Runtimes excluded by `--runtime` SHALL NOT be reported.

#### Scenario: Missing Python interpreter

- **GIVEN** no usable `python3` is on PATH and the scan root contains a Python project
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** the Python project is listed with the note `python3 not found on PATH`

#### Scenario: Unavailable runtime outside the filter

- **GIVEN** no `node` binary is on PATH and the scan root contains Node.js and Java projects
- **WHEN** the user runs `dtwiz analyze projects --runtime java`
- **THEN** no Node.js project and no note about `node` appear in the output

---

### Requirement: Human-readable output

Without `--json`, the command SHALL print the scanned root, the active runtime filter when one is given, and a table with the columns PATH, RUNTIME, MARKERS, NOTES. Rows SHALL be sorted by path, then by runtime identifier alphabetically. Paths under the home directory SHALL be shown with a `~` prefix. A single footer line SHALL show the install command pattern.

#### Scenario: Two runtimes in one directory

- **GIVEN** `~/work/shop/api` contains both `package.json` and `requirements.txt`
- **WHEN** the user runs `dtwiz analyze projects` from `~/work/shop`
- **THEN** two adjacent rows for `~/work/shop/api` are printed, `nodejs` before `python`

#### Scenario: Notes column

- **GIVEN** the scan root contains a Java project with running process 5310, a Go project with module `github.com/acme/gateway`, and a .NET project while `DotNetOtel` is on
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** the notes read `PID 5310`, `module github.com/acme/gateway`, and `manual steps after install` respectively

#### Scenario: Footer

- **GIVEN** at least one project is listed
- **WHEN** the human-readable output is printed
- **THEN** it ends with `Instrument one:  dtwiz install otel --project <path> --runtime <runtime>`

#### Scenario: Nothing found

- **GIVEN** the scan root contains no project markers
- **WHEN** the user runs `dtwiz analyze projects`
- **THEN** the output names the scanned root, prints `No projects found.`, and the command exits with code 0

---

### Requirement: JSON output

With `--json`, stdout SHALL contain exactly one JSON object and nothing else, with keys `roots` (absolute scanned paths), `runtimes` (each reported runtime with `id`, `available`, and `reason` when unavailable), and `projects`. Each project SHALL carry `runtime`, absolute `path`, `markers`, `status` (`instrumentable`, `manual_steps`, or `runtime_unavailable`), and, when present, `pids` and `module`. Projects SHALL use the same order as the human-readable output.

#### Scenario: JSON document

- **GIVEN** the scan root `~/work/shop` contains a Node.js project at `api` with running processes 4121 and 4122
- **WHEN** the user runs `dtwiz analyze projects --json`
- **THEN** stdout is a single JSON object whose `projects` contain an entry with `runtime` `nodejs`, the absolute path of `api`, `markers` `["package.json"]`, `pids` `[4121, 4122]`, and `status` `instrumentable`

#### Scenario: Availability details in JSON

- **GIVEN** no usable `python3` is on PATH
- **WHEN** the user runs `dtwiz analyze projects --json`
- **THEN** `runtimes` contains an entry with `id` `python`, `available` `false`, and a `reason`, and Python projects have `status` `runtime_unavailable`

#### Scenario: No other text on stdout

- **GIVEN** a runtime is unavailable
- **WHEN** the user runs `dtwiz analyze projects --json`
- **THEN** stdout parses as JSON as a whole; no "Skipping" or other informational lines are printed to it

#### Scenario: Empty result

- **GIVEN** the scan root contains no project markers
- **WHEN** the user runs `dtwiz analyze projects --json`
- **THEN** stdout is a JSON object with an empty `projects` array and the command exits with code 0

---

### Requirement: Listed projects carry their install command

In JSON output, every project whose status is `instrumentable` or `manual_steps` SHALL carry an `install_command` of the form `dtwiz install otel --project <absolute path> --runtime <id>` that selects exactly that project and runtime. Projects with status `runtime_unavailable` SHALL NOT carry an `install_command`.

#### Scenario: Install command matches the entry

- **GIVEN** `/Users/me/work/shop/api` is listed as an instrumentable `nodejs` project
- **WHEN** the user runs `dtwiz analyze projects --json`
- **THEN** that entry's `install_command` is `dtwiz install otel --project /Users/me/work/shop/api --runtime nodejs`

#### Scenario: Path with spaces

- **GIVEN** `/Users/me/My Apps/web` is listed as an instrumentable `nodejs` project
- **WHEN** the user runs `dtwiz analyze projects --json`
- **THEN** that entry's `install_command` is `dtwiz install otel --project "/Users/me/My Apps/web" --runtime nodejs`

#### Scenario: No install command for an unavailable runtime

- **GIVEN** a Python project is listed with status `runtime_unavailable`
- **WHEN** the user runs `dtwiz analyze projects --json`
- **THEN** that entry has no `install_command`
