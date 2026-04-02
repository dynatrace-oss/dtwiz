# Multi-Runtime Orchestration

## ADDED Requirements

### Requirement: Unified project listing

During OTel Collector installation, the system SHALL detect available runtimes, scan for projects across all GA runtimes, and present a single unified project list. Each entry shows the runtime name, project path, marker files, and any running PIDs. The list SHALL include a "Skip — collector only" option. The user selects exactly one project to instrument (or skips). There is no runtime selection menu — the user picks a project directly.

#### Scenario: Multiple runtimes and projects detected

- **GIVEN** an OTel Collector installation is in progress
- **WHEN** `java` and `node` are on PATH and Java/Node.js projects exist
- **THEN** the list shows e.g. `[1] Java     /home/user/projects/api  (pom.xml)`, `[2] Node.js  /home/user/projects/web  (package.json)`, `[3] Skip — collector only` and the user picks one

#### Scenario: No projects detected

- **GIVEN** an OTel Collector installation is in progress
- **WHEN** no project directories are found for any GA runtime
- **THEN** the system skips the project list and proceeds with collector-only installation

### Requirement: Runtime plan creation from selected project

After the user selects a project, the system SHALL create the appropriate instrumentation plan based on the project's runtime. For runtimes that require entrypoint detection (Python, Node.js), the system detects entrypoints automatically and prompts only if none are found.

#### Scenario: User selects a Java project

- **GIVEN** the unified project list is displayed
- **WHEN** the user picks a Java project
- **THEN** the system creates an instrumentation plan for that project and shows it in the confirmation preview

#### Scenario: User selects a Node.js project with detected entrypoint

- **GIVEN** the unified project list is displayed
- **WHEN** the user picks a Node.js project that has a `package.json` with a `main` field
- **THEN** the system creates an instrumentation plan with the detected entrypoint — no extra prompt

#### Scenario: User selects a Python project with no entrypoint

- **GIVEN** the unified project list is displayed
- **WHEN** the user picks a Python project and no entrypoint is auto-detected
- **THEN** the system prompts `No entrypoint detected. Enter the Python file to run (e.g. app.py):`

### Requirement: Confirmation preview

After the user selects a project (or skips), the system SHALL show a confirmation preview. Step 1 is always the OTel Collector; if a runtime plan was created, it appears as step 2. This preview is followed by a single `Proceed with installation?` prompt.

#### Scenario: User selected a Python project

- **GIVEN** the user picked a Python project from the list and an entrypoint was determined
- **WHEN** the confirmation preview is rendered
- **THEN** it shows `1) OTel Collector` with directory/binary details, `2) Python auto-instrumentation` with plan steps, then `Proceed with installation? [Y/n]`

#### Scenario: User skipped or no project found

- **GIVEN** the user chose "Skip" or no projects were detected
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

- **GIVEN** the user selected a Python project, confirmed, and the collector installed successfully
- **WHEN** the runtime plan execution phase begins
- **THEN** the Python instrumentation step runs under a `── Python auto-instrumentation ──` header

#### Scenario: Collector only

- **GIVEN** no project was selected or the plan was nil
- **WHEN** the collector installation completes
- **THEN** no instrumentation execution occurs

### Requirement: Header text mentions runtime presence

When a runtime plan is selected, the introductory message SHALL state that the collector and application instrumentation will be installed. When no runtime is selected, the message SHALL only mention the collector.

#### Scenario: Runtime selected

- **GIVEN** the user selected a project and a plan was created
- **WHEN** the intro message is printed
- **THEN** it reads "This will install the OTel Collector and auto-instrument your application."

#### Scenario: User skipped runtime selection

- **GIVEN** the user chose "Skip" from the unified project list
- **WHEN** the intro message is printed
- **THEN** the message is omitted

#### Scenario: No projects detected

- **GIVEN** no plan was created because no projects were found for any GA runtime
- **WHEN** the intro message is printed
- **THEN** the message is omitted
