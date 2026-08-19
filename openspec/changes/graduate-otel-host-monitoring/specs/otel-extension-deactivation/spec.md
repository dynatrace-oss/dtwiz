## MODIFIED Requirements

### Requirement: OTel uninstall lets the user choose whether to remove the host monitoring extension

When `dtwiz uninstall otel` runs, the simple yes/no confirmation is replaced by a three-option prompt: **Delete all** (default), **Only collector**, and **Cancel**. The extension is removed only when the user selects Delete all. If the extension cannot be removed, the uninstaller SHALL warn the user and complete the uninstall normally.

#### Scenario: User selects Delete all and routes and extension are removed after local cleanup

- **GIVEN** the OpenTelemetry Host Monitoring extension is installed on the tenant
- **WHEN** the user selects `[1] Delete all` at the prompt (or `--yes` is set)
- **THEN** local processes are killed and directories removed first
- **THEN** the Grail OpenPipeline routing entries for metrics, logs, and spans are removed
- **THEN** a confirmation line is printed for each successfully removed route
- **THEN** the extension environment configuration is deactivated
- **THEN** the extension version is deleted from the tenant
- **THEN** a confirmation line is printed indicating the extension was removed

#### Scenario: User selects Only collector and extension and routes are kept on tenant

- **GIVEN** the user selects `[2] Only collector` at the prompt
- **WHEN** `dtwiz uninstall otel` proceeds
- **THEN** local processes are killed and directories removed
- **THEN** no route removal or extension API call is made
- **THEN** the extension and Grail routes remain on the tenant

#### Scenario: User selects Cancel

- **GIVEN** the user selects `[3] Cancel` at the prompt
- **WHEN** `dtwiz uninstall otel` handles the selection
- **THEN** no local cleanup is performed
- **THEN** no extension API call is made

#### Scenario: Extension not present on tenant

- **GIVEN** the OpenTelemetry Host Monitoring extension is not installed on the tenant
- **WHEN** the user selects `[1] Delete all`
- **THEN** local cleanup proceeds normally
- **THEN** the missing extension is treated as success

#### Scenario: Grail route absent treated as success

- **GIVEN** the OTel Host Monitoring extension pipeline or routing entry is not present on the tenant
- **WHEN** the user selects `[1] Delete all`
- **THEN** route removal is skipped for that signal without printing a warning
- **THEN** extension deactivation and version deletion proceed normally

#### Scenario: Grail route removal fails for one signal and other signals continue

- **GIVEN** the route removal API call fails for one signal but succeeds for others
- **WHEN** the user selects `[1] Delete all`
- **THEN** a per-signal advisory warning is printed for the failing signal
- **THEN** route removal continues for the remaining signals
- **THEN** extension deactivation and version deletion proceed normally

#### Scenario: Extension removal fails

- **GIVEN** the extension API call fails
- **WHEN** the user selects `[1] Delete all`
- **THEN** local cleanup completes normally
- **THEN** a warning is printed indicating extension removal failed
- **THEN** the command exits with code 0

#### Scenario: Dry run skips extension removal

- **GIVEN** the user runs `dtwiz uninstall otel --dry-run`
- **WHEN** the uninstall preview is rendered
- **THEN** the preview shows the extension that would be removed
- **THEN** no prompt is shown
- **THEN** no extension removal API call is made

## ADDED Requirements

### Requirement: Uninstall preview includes extension removal

When `dtwiz uninstall otel` runs, the uninstall preview SHALL include a line identifying the extension that will be removed from the tenant.

#### Scenario: Preview shows extension and route removal

- **GIVEN** `dtwiz uninstall otel` renders the preview
- **WHEN** removable local collector or instrumentation artifacts are found
- **THEN** the preview includes the name of the extension that would be deleted
- **THEN** the preview includes the Grail routes for metrics, logs, and spans that would be removed
- **THEN** the three-option prompt appears after the preview

## REMOVED Requirements

### Requirement: Uninstall preview includes extension removal when experimental is enabled

**Reason**: Extension and route cleanup is released and is now part of the default `dtwiz uninstall otel` flow.

**Migration**: Use `Uninstall preview includes extension removal` for the default preview behavior. `--experimental` and `DTWIZ_EXPERIMENTAL=true` are no longer required for the extension and route removal preview.
