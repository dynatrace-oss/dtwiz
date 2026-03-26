# Multi-Runtime Orchestration

## ADDED Requirements

### Requirement: Runtime selection menu

The `InstallOtelCollector` function in `pkg/installer/otel.go` SHALL detect available runtimes via `exec.LookPath` and present them as a numbered selection menu. The menu SHALL include a "Skip — collector only" option. The user selects exactly one runtime to instrument (or skips). Only one runtime plan is created per invocation.

#### Scenario: Multiple runtimes available

- **GIVEN** an OTel Collector installation is in progress via `InstallOtelCollector`
- **WHEN** `python3`, `java`, and `node` are on PATH
- **THEN** the menu shows `[1] Python`, `[2] Java`, `[3] Node.js`, `[4] Skip — collector only` and the user picks one

#### Scenario: Only one runtime available

- **GIVEN** an OTel Collector installation is in progress via `InstallOtelCollector`
- **WHEN** only `python3` is on PATH
- **THEN** the menu shows `[1] Python`, `[2] Skip — collector only`

#### Scenario: No runtimes available

- **GIVEN** an OTel Collector installation is in progress via `InstallOtelCollector`
- **WHEN** no runtime binaries are found on PATH
- **THEN** the system skips the menu and proceeds with collector-only installation

### Requirement: Project listing after runtime selection

After the user selects a runtime from the menu, the system SHALL call the corresponding `Detect<Lang>Plan` function, which lists discovered projects and prompts the user to select one (exactly as the existing Python flow does in `pkg/installer/otel_python.go`).

#### Scenario: User selects Python and projects exist

- **GIVEN** the runtime selection menu was shown
- **WHEN** the user picks Python from the menu and Python projects are found
- **THEN** the system lists Python projects with `Select a project to instrument [1-N] or press Enter to skip:`

#### Scenario: User selects a runtime but no projects found

- **GIVEN** the runtime selection menu was shown
- **WHEN** the user picks Java from the menu but no `pom.xml` or `build.gradle` files exist
- **THEN** `DetectJavaPlan` returns nil and the system proceeds with collector-only installation

### Requirement: Confirmation preview

After the user selects a project (or skips), the system SHALL show a confirmation preview. Step 1 is always the OTel Collector; if a runtime plan was created, it appears as step 2. This preview is followed by a single `Proceed with installation?` prompt.

#### Scenario: User selected a Python project

- **GIVEN** the user picked Python from the runtime menu and selected a project
- **WHEN** the confirmation preview is rendered
- **THEN** it shows `1) OTel Collector` with directory/binary details, `2) Python auto-instrumentation` with plan steps, then `Proceed with installation? [Y/n]`

#### Scenario: User skipped or no project found

- **GIVEN** the user chose "Skip" or the selected runtime had no projects
- **WHEN** the confirmation preview is rendered
- **THEN** it shows only `1) OTel Collector` and proceeds to confirmation

### Requirement: Single confirmation prompt

The system SHALL show a single `Proceed with installation?` prompt that covers the collector and the selected runtime plan (if any). There SHALL NOT be separate confirmation prompts for collector and runtime.

#### Scenario: User confirms

- **GIVEN** the confirmation preview is displayed with collector and optional runtime plan
- **WHEN** the user answers yes to the confirmation prompt
- **THEN** the collector is installed first, followed by execution of the selected runtime plan (if any)

#### Scenario: User cancels

- **GIVEN** the confirmation preview is displayed
- **WHEN** the user answers no
- **THEN** no collector or instrumentation is installed

### Requirement: Runtime plan execution

After the collector is installed successfully, the system SHALL execute the selected runtime plan (if non-nil). The plan's execution block SHALL be preceded by a separator header identifying the runtime.

#### Scenario: Plan executes after collector

- **GIVEN** the user selected Python, confirmed, and the collector installed successfully
- **WHEN** the runtime plan execution phase begins
- **THEN** `pythonPlan.Execute()` runs with header `── Python auto-instrumentation ──`

#### Scenario: Collector only

- **GIVEN** no runtime was selected or the plan was nil
- **WHEN** the collector installation completes
- **THEN** no instrumentation execution occurs

### Requirement: Header text mentions runtime presence

When a runtime plan is selected, the introductory message SHALL state that the collector and application instrumentation will be installed. When no runtime is selected, the message SHALL only mention the collector.

#### Scenario: Runtime selected

- **GIVEN** the user selected a runtime and a plan was created
- **WHEN** the intro message is printed
- **THEN** it reads "This will install the OTel Collector and auto-instrument your application."

#### Scenario: No runtime selected

- **GIVEN** the user chose "Skip" or no plan was created
- **WHEN** the intro message is printed
- **THEN** the message is omitted or states collector-only installation

### Requirement: Unimplemented runtimes shown as "coming soon"

Runtimes detected on PATH whose installer is not yet implemented SHALL appear in the selection menu with a "coming soon" label. They SHALL NOT be selectable.

#### Scenario: Runtime detected but not implemented

- **GIVEN** the runtime detection phase found `go` on PATH
- **WHEN** `GoInstrumentationPlan` execution is not yet implemented
- **THEN** the menu shows `[N] Go (coming soon)` and selecting it prints a message that it is not yet available

#### Scenario: All runtimes implemented

- **GIVEN** all detected runtimes have full installer implementations
- **WHEN** the runtime selection menu is rendered
- **THEN** no "coming soon" labels appear and all entries are selectable

### Requirement: Dry-run support for new flows

When `--dry-run` is set, the runtime selection menu and combined preview SHALL be printed but no collector or instrumentation SHALL be installed.

#### Scenario: Dry-run with runtimes detected

- **GIVEN** `--dry-run` is set on the `install otel` command
- **WHEN** runtimes are detected on PATH
- **THEN** the system prints the selection menu, shows the combined plan preview, and exits without installing anything

#### Scenario: Dry-run without runtimes

- **GIVEN** `--dry-run` is set on the `install otel` command
- **WHEN** no runtimes are detected
- **THEN** the system prints the collector-only dry-run plan as before

### Requirement: InstallOtelCollectorOnly non-regression

The existing `InstallOtelCollectorOnly()` function in `pkg/installer/otel.go` SHALL NOT be modified by this change. It SHALL continue to install the collector without runtime detection or selection.

#### Scenario: InstallOtelCollectorOnly unchanged

- **GIVEN** a user invokes the collector-only install path via `InstallOtelCollectorOnly()`
- **WHEN** the function executes
- **THEN** the behavior is identical to before this change — no runtime menu, no selection prompt, no instrumentation
