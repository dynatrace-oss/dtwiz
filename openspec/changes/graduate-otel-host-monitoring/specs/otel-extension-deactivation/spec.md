# OTel Extension Deactivation

## MODIFIED Requirements

### Requirement: OTel uninstall lets the user choose whether to remove the host monitoring extension

When `dtwiz uninstall otel` runs, the simple yes/no confirmation is replaced by a three-option prompt: **Delete all** (default), **Only collector**, and **Cancel**. The extension is removed only when the user selects Delete all. If the extension cannot be removed, the uninstaller SHALL warn the user and complete the uninstall normally.

#### Scenario: User selects Delete all

- **GIVEN** the OpenTelemetry Host Monitoring extension is installed on the tenant
- **WHEN** the user selects `[1] Delete all` at the prompt (or `--yes` is set)
- **THEN** local processes are killed and directories removed first
- **THEN** tenant-side host monitoring cleanup is attempted
- **THEN** the extension environment configuration is deactivated
- **THEN** the extension version is deleted from the tenant
- **THEN** a confirmation line is printed indicating the extension was removed

#### Scenario: User selects Only collector

- **GIVEN** the user selects `[2] Only collector` at the prompt
- **WHEN** `dtwiz uninstall otel` proceeds
- **THEN** local processes are killed and directories removed
- **THEN** no tenant-side host monitoring cleanup API call is made
- **THEN** the extension and Grail routes remain on the tenant

#### Scenario: User selects Cancel

- **GIVEN** the user selects `[3] Cancel` at the prompt
- **WHEN** `dtwiz uninstall otel` handles the selection
- **THEN** no local cleanup is performed
- **THEN** no tenant-side host monitoring cleanup API call is made

#### Scenario: Extension removal fails

- **GIVEN** the extension API call fails
- **WHEN** the user selects `[1] Delete all`
- **THEN** local cleanup completes normally
- **THEN** a warning is printed indicating extension removal failed
- **THEN** the command exits with code 0

## ADDED Requirements

### Requirement: Uninstall preview includes extension removal

When `dtwiz uninstall otel` runs, the uninstall preview SHALL include a line identifying the extension that will be removed from the tenant.

#### Scenario: Preview shows extension removal before the uninstall choice

- **GIVEN** `dtwiz uninstall otel` renders the preview
- **WHEN** removable local collector or instrumentation artifacts are found
- **THEN** the preview includes the name of the extension that would be deleted
- **THEN** the three-option prompt appears after the preview

#### Scenario: Dry run skips the uninstall choice and extension removal

- **GIVEN** the user runs `dtwiz uninstall otel --dry-run`
- **WHEN** the uninstall preview is rendered
- **THEN** the preview shows the extension that would be removed
- **THEN** no prompt is shown
- **THEN** no extension removal API call is made

## REMOVED Requirements

### Requirement: Uninstall preview includes extension removal when experimental is enabled

**Reason**: Extension and route cleanup is released and is now part of the regular `dtwiz uninstall otel` flow.

**Migration**: Use `Uninstall preview includes extension removal` for the regular preview behavior. `--experimental` and `DTWIZ_EXPERIMENTAL=true` are no longer required for the extension and route removal preview.
