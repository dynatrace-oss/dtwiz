# Spec: OTel Project Scan

## MODIFIED Requirements

### Requirement: Scan scope limited to working directory

 The scanner SHALL search the working directory and its subdirectories for OTel project markers by default. It SHALL NOT traverse any ancestor directories of the working directory **unless** (a) the user selects an option that includes the home directory at the interactive prompt (see "Interactive home-directory scan choice"), or (b) the command is running non-interactively (e.g., `--yes`/`AutoConfirm`, or stdin is not a TTY), in which case scanning the home directory is the default. When home scanning is enabled, the home directory and its subdirectories are also searched.

#### Scenario: Project in working directory is found

- **GIVEN** the working directory contains a project marker (e.g. `requirements.txt`, `package.json`)
- **WHEN** the user runs a dtwiz OTel install command from that directory
- **THEN** the project is detected and instrumentation proceeds

#### Scenario: Project in subdirectory is found

- **GIVEN** the working directory has subdirectories that contain project markers
- **WHEN** the user runs a dtwiz OTel install command from that parent directory
- **THEN** all matching subdirectory projects are detected

#### Scenario: Parent directory is not scanned when user does not opt in

- **GIVEN** the user runs a dtwiz OTel install command from a subdirectory of their project root (e.g. `my-project/src/`)
- **WHEN** the user chooses to scan the working directory only
- **THEN** the parent directory (`my-project/`) is NOT scanned and projects there are NOT detected

#### Scenario: Parent directory is not scanned during uninstall

- **GIVEN** the working directory is a subdirectory of a project root
- **WHEN** the user runs a dtwiz OTel uninstall command from that subdirectory
- **THEN** `.otel/` directories in parent directories are NOT found and NOT removed; only `.otel/` directories within the working directory tree are considered

---

## ADDED Requirements

### Requirement: Interactive home-directory scan choice

When `dtwiz install otel` runs WITHOUT an explicit `--project` path, and the working directory does NOT already cover the home directory, the scanner SHALL prompt the user exactly once, before any project scanning begins, with a three-way choice:

- `Y` (default): scan the working directory AND the home directory
- `c`: scan the working directory only
- `n`: abort the entire `install otel` command

The working directory "already covers" the home directory when the working directory IS the home directory, or when the home directory is a descendant of the working directory. In those cases the scanner SHALL NOT prompt and SHALL scan the working directory only (the home directory is already within the working-directory walk).

The prompt SHALL be presented before the per-runtime scans fan out, so that it is shown at most once per invocation regardless of how many runtimes are scanned.

The `~/.dtwiz/examples/` bundled-examples scan SHALL remain always-on regardless of the choice, including when the user selects `c`.

When an explicit `--project` path is provided, the scanner SHALL NOT prompt and SHALL NOT perform a directory scan.

#### Scenario: Prompt offered from a directory that does not cover home

- **GIVEN** the working directory is neither the home directory nor an ancestor of it (e.g. `~/projects/foo` or `/tmp/work`)
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
- **WHEN** the working directory does not cover the home directory
- **THEN** the scanner SHALL NOT block on a prompt and SHALL default to scanning the working directory AND the home directory

#### Scenario: Explicit project path skips prompt and scan

- **GIVEN** an explicit `--project <path>` is provided
- **WHEN** the user runs `dtwiz install otel --project <path>`
- **THEN** the scanner SHALL NOT present the prompt and SHALL NOT perform a directory scan
