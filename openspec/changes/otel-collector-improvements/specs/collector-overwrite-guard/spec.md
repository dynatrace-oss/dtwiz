## ADDED Requirements

### Requirement: Clean install checks for existing collector
On the `dtwiz install otel` path, the installer SHALL check for an existing collector installation (running processes and installation directory) before presenting the installation plan.

#### Scenario: Existing collector found — user confirms overwrite
- **WHEN** the user runs `dtwiz install otel` and a collector is already running or the install directory exists
- **THEN** the installer SHALL inform the user and prompt with options: overwrite, switch to `dtwiz update otel`, or abort

#### Scenario: Existing collector found — user chooses update
- **WHEN** the user selects "switch to update" at the overwrite prompt
- **THEN** the installer SHALL invoke the `UpdateOtelConfig` flow instead of proceeding with a fresh install

#### Scenario: No existing collector
- **WHEN** no running collector processes are found and the install directory does not exist
- **THEN** the installer SHALL proceed with the normal installation plan without any additional prompts

#### Scenario: Dry-run mode with existing collector
- **WHEN** `--dry-run` is set and an existing collector is found
- **THEN** the installer SHALL report the existing installation in the dry-run output but SHALL NOT prompt for action
