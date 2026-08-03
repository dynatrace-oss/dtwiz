# Spec: OTel Project Scan

## ADDED Requirements

### Requirement: Scan scope limited to working directory

The scanner SHALL search the working directory and its subdirectories for OTel project markers. It SHALL NOT traverse any ancestor directories of the working directory. When the working directory lies outside the home-directory tree, the scanner MAY additionally search the home directory and its subdirectories as a separate scan root, either because the user opted in at the interactive prompt or because the command is running non-interactively (see "Interactive home-directory scan choice"). Home is never an ancestor of the working directory in this case, so the no-ancestor-traversal guarantee is preserved.

#### Scenario: Project in working directory is found

- **GIVEN** the working directory contains a project marker (e.g. `requirements.txt`, `package.json`)
- **WHEN** the user runs a dtwiz OTel install command from that directory
- **THEN** the project is detected and instrumentation proceeds

#### Scenario: Project in subdirectory is found

- **GIVEN** the working directory has subdirectories that contain project markers
- **WHEN** the user runs a dtwiz OTel install command from that parent directory
- **THEN** all matching subdirectory projects are detected

#### Scenario: Parent directory is not scanned

- **GIVEN** the working directory is a subdirectory of a project root within the home tree (e.g. `~/my-project/src/`)
- **WHEN** the user runs a dtwiz OTel install command from there (no prompt is shown, since the working directory is within home)
- **THEN** the parent directory (`~/my-project/`) is NOT scanned and projects there are NOT detected

#### Scenario: Parent directory is not scanned during uninstall

- **GIVEN** the working directory is a subdirectory of a project root
- **WHEN** the user runs a dtwiz OTel uninstall command from that subdirectory
- **THEN** `.otel/` directories in parent directories are NOT found and NOT removed; only `.otel/` directories within the working directory tree are considered

---

### Requirement: Interactive home-directory scan choice

When `dtwiz install otel` runs WITHOUT an explicit `--project` path, and the working directory lies OUTSIDE the home-directory tree (the working directory and the home directory are in disjoint trees, with neither an ancestor of the other), the scanner SHALL prompt the user exactly once, before any project scanning begins, with a three-way choice:

- `Y` (default): scan the working directory AND the home directory
- `c`: scan the working directory only
- `n`: abort the entire `install otel` command

The scanner SHALL NOT prompt, and SHALL scan the working directory only, whenever the working directory is in the same lineage as the home directory:

- the working directory IS the home directory, or
- the home directory is a descendant of the working directory (the working-directory walk already covers home), or
- the working directory is a descendant of the home directory (the user is already working inside their home tree); the scanner SHALL NOT add the home directory as a second root in this case.

The prompt SHALL be presented before the per-runtime scans fan out, so that it is shown at most once per invocation regardless of how many runtimes are scanned.

The `~/.dtwiz/examples/` bundled-examples scan SHALL remain always-on regardless of the choice, including when the user selects `c`.

When an explicit `--project` path is provided, the scanner SHALL NOT prompt and SHALL NOT perform a directory scan.

#### Scenario: Prompt offered from a directory outside the home tree

- **GIVEN** the working directory is in a disjoint tree from the home directory (e.g. `/opt/app`, `/tmp/work`, or `D:\projects` on Windows)
- **WHEN** the user runs `dtwiz install otel` without `--project`
- **THEN** the scanner SHALL present the three-way `Y/c/n` prompt before scanning begins

#### Scenario: Choosing Y scans working directory and home

- **GIVEN** the three-way prompt is shown
- **WHEN** the user selects `Y` (or presses Enter)
- **THEN** the scanner SHALL search both the working directory tree and the home directory tree for project markers

#### Scenario: Choosing c scans working directory only

- **GIVEN** the three-way prompt is shown
- **WHEN** the user selects `c`
- **THEN** the scanner SHALL search only the working directory tree (plus the always-on bundled examples) and SHALL NOT traverse the home directory

#### Scenario: Choosing n aborts the install command

- **GIVEN** the three-way prompt is shown
- **WHEN** the user selects `n`
- **THEN** the entire `dtwiz install otel` command SHALL be cancelled and SHALL NOT proceed to scanning or installation

#### Scenario: Prompt skipped when working directory is within the home directory

- **GIVEN** the working directory is a descendant of the home directory (e.g. `~/projects/foo`)
- **WHEN** the user runs `dtwiz install otel` without `--project`
- **THEN** the scanner SHALL NOT prompt and SHALL scan the working directory tree only
- **AND** the scanner SHALL NOT add the home directory as a second root

#### Scenario: Prompt skipped when working directory is the home directory

- **GIVEN** the working directory is the home directory
- **WHEN** the user runs `dtwiz install otel` without `--project`
- **THEN** the scanner SHALL NOT prompt and SHALL scan the working directory (home) tree

#### Scenario: Prompt skipped when home is a descendant of the working directory

- **GIVEN** the working directory is an ancestor of the home directory (e.g. `/Users` on macOS, `/home` on Linux)
- **WHEN** the user runs `dtwiz install otel` without `--project`
- **THEN** the scanner SHALL NOT prompt and SHALL scan the working directory tree, which already includes the home directory

#### Scenario: Non-interactive default includes home

- **GIVEN** the command is run with `--yes`/`AutoConfirm`, or stdin is not a TTY
- **WHEN** the working directory lies outside the home tree (disjoint from home)
- **THEN** the scanner SHALL NOT block on a prompt and SHALL default to scanning the working directory AND the home directory

#### Scenario: Explicit project path skips prompt and scan

- **GIVEN** an explicit `--project <path>` is provided
- **WHEN** the user runs `dtwiz install otel --project <path>`
- **THEN** the scanner SHALL NOT present the prompt and SHALL NOT perform a directory scan

---

### Requirement: Bundled examples directory is always included in project scan

The scanner SHALL include `~/.dtwiz/examples/` as an additional scan root alongside the working directory. This ensures that the demo app is visible in the project list regardless of the directory the user runs `dtwiz` from.

The bundled examples path is:

- macOS and Linux: `$HOME/.dtwiz/examples/`
- Windows: `%USERPROFILE%\.dtwiz\examples\`

If the path does not exist, it is silently skipped.

#### Scenario: Demo app is detected from any working directory

- **WHEN** `~/.dtwiz/examples/schnitzel/` exists
- **AND** the user runs `dtwiz setup` or `dtwiz install otel` from any directory
- **THEN** the schnitzel project SHALL appear in the list of detected projects alongside any projects found in the working directory

#### Scenario: Bundled examples path does not exist

- **WHEN** `~/.dtwiz/examples/` does not exist on the user's machine
- **THEN** the scanner SHALL skip it silently and only scan the working directory

#### Scenario: Project in bundled examples does not duplicate a CWD project

- **WHEN** the working directory is `~/.dtwiz/examples/` or a subdirectory of it
- **THEN** the scanner SHALL NOT list the same project twice

#### Scenario: CWD projects and bundled examples projects both appear in results

- **WHEN** the working directory contains one or more projects
- **AND** `~/.dtwiz/examples/` also contains one or more projects
- **THEN** the scanner SHALL return all projects from both locations in a single combined list
- **AND** each project SHALL appear exactly once

---

### Requirement: Demo app remains visible in dtwiz setup after install demo completes

After `dtwiz install demo` has run, the schnitzel project SHALL appear in the project list when the user runs `dtwiz setup` from any directory, so the user can manage or update the instrumentation without needing to navigate to the bundled path.

#### Scenario: Schnitzel appears in dtwiz setup after demo install from a different directory

- **GIVEN** `dtwiz install demo` has completed and the OTel Collector is configured for schnitzel
- **WHEN** the user opens a new terminal in a different directory
- **AND** runs `dtwiz setup` or `dtwiz install otel`
- **THEN** the schnitzel project at `~/.dtwiz/examples/schnitzel/` SHALL appear in the detected project list
- **AND** the user SHALL be able to select it to update or re-instrument it

#### Scenario: Schnitzel appears alongside user projects in the same setup session

- **GIVEN** the user's working directory contains their own Python or Node.js project
- **AND** `~/.dtwiz/examples/schnitzel/` also exists
- **WHEN** the user runs `dtwiz setup`
- **THEN** both the user's project and schnitzel SHALL appear in the project list
- **AND** the user SHALL be able to select either one
