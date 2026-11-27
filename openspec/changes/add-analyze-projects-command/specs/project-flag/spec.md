# Spec Delta

## MODIFIED Requirements

### Requirement: --project flag on install otel and install otel-python

`install otel` and `install otel-python` SHALL accept a `--project <path>` flag that skips interactive project scanning and selection, using the given path directly.

#### Scenario: --project provided to install otel

- **WHEN** `--project <path>` is passed to `install otel` without `--runtime`
- **THEN** the installer SHALL NOT scan for projects and SHALL NOT show the project selection prompt
- **AND** SHALL detect the runtime from the files at `<path>`, checking Java, Node.js, Python, Go, and (when `DotNetOtel` is on) .NET markers in that order
- **AND** SHALL build the instrumentation plan directly from that path

#### Scenario: --project provided to install otel-python

- **WHEN** `--project <path>` is passed to `install otel-python`
- **THEN** the Python installer SHALL use the provided path directly without scanning for projects

#### Scenario: --project path does not exist

- **WHEN** `--project <path>` is passed and `<path>` does not exist on disk
- **THEN** the installer SHALL exit with a clear error: `project path not found: <path>`

#### Scenario: --project not provided (default behavior unchanged)

- **WHEN** `--project` is not passed
- **THEN** the existing scan → list → select flow SHALL run unchanged

#### Scenario: --project and --yes together

- **WHEN** both `--project <path>` and `--yes` are passed
- **THEN** the project is pre-selected and no confirmation prompt is shown
- **AND** installation proceeds fully non-interactively

## ADDED Requirements

### Requirement: --runtime selects the runtime for --project on install otel

`install otel` SHALL accept `--runtime <id>` together with `--project <path>`. The value SHALL be exactly one identifier from the set accepted by `dtwiz analyze projects --runtime` (`python`, `java`, `nodejs`, `go`, and `dotnet` only while the `DotNetOtel` feature flag is on), matched exactly. The given runtime SHALL be used instead of inferring one from the files at `<path>`.

#### Scenario: Runtime chosen for a directory with two runtimes

- **GIVEN** `~/app` contains both `package.json` and `requirements.txt`
- **WHEN** the user runs `dtwiz install otel --project ~/app --runtime python`
- **THEN** the Python instrumentation plan is built for `~/app`, not the Node.js one

#### Scenario: Explicit runtime overrides marker precedence

- **GIVEN** the `DotNetOtel` feature flag is on and `~/app` contains both `package.json` and `App.csproj`
- **WHEN** the user runs `dtwiz install otel --project ~/app --runtime dotnet`
- **THEN** `~/app` is treated as a .NET project, although Node.js markers would take precedence without `--runtime`

#### Scenario: Value from analyze projects is accepted unchanged

- **GIVEN** `dtwiz analyze projects --json` lists an entry with an `install_command`
- **WHEN** the user runs that `install_command`
- **THEN** the install proceeds for exactly that project and runtime

#### Scenario: --project without --runtime

- **GIVEN** `~/app` contains only `pom.xml`
- **WHEN** the user runs `dtwiz install otel --project ~/app`
- **THEN** the runtime is inferred as before and the Java plan is built

---

### Requirement: --runtime value and pairing are validated

`install otel` SHALL reject, with exit code 1 and before any change is made: `--runtime` without `--project`, a value outside the identifier set (including wrong case), a value with more than one identifier, and a runtime for which `<path>` contains no project markers.

#### Scenario: --runtime without --project

- **GIVEN** any system
- **WHEN** the user runs `dtwiz install otel --runtime java`
- **THEN** the command fails with `--runtime requires --project`

#### Scenario: Unknown or wrongly cased value

- **GIVEN** `~/app` exists and the `DotNetOtel` feature flag is off
- **WHEN** the user runs `dtwiz install otel --project ~/app --runtime Python`
- **THEN** the command fails with `unknown runtime "Python" (valid: python, java, nodejs, go)`

#### Scenario: More than one value

- **GIVEN** `~/app` exists
- **WHEN** the user runs `dtwiz install otel --project ~/app --runtime java,go`
- **THEN** the command fails with `--runtime takes a single value`

#### Scenario: dotnet while the feature flag is off

- **GIVEN** the `DotNetOtel` feature flag is off and `~/app` contains a `.csproj` file
- **WHEN** the user runs `dtwiz install otel --project ~/app --runtime dotnet`
- **THEN** the command fails with `unknown runtime "dotnet" (valid: python, java, nodejs, go)`, without mentioning the feature flag

#### Scenario: No markers for the requested runtime

- **GIVEN** `~/app/web` contains `package.json` but no Python project markers
- **WHEN** the user runs `dtwiz install otel --project ~/app/web --runtime python`
- **THEN** the command fails with `no python project markers in ~/app/web`
- **AND** no OTel Collector or instrumentation change is made
