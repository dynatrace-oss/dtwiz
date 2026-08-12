# Spec: OTel Extension Deactivation

## Requirements

### Requirement: OTel uninstall lets the user choose whether to remove the host monitoring extension

When `dtwiz uninstall otel` runs with the experimental flag enabled, the simple yes/no confirmation is replaced by a three-option prompt: **Delete all** (default), **Only collector**, and **Cancel**. The extension is removed only when the user selects Delete all. If the extension cannot be removed, the uninstaller SHALL warn the user and complete the uninstall normally.

#### Scenario: User selects Delete all — routes and extension removed after local cleanup

- **GIVEN** the experimental flag is enabled
- **AND** the OpenTelemetry Host Monitoring extension is installed on the tenant
- **WHEN** the user selects `[1] Delete all` at the prompt (or `--yes` is set)
- **THEN** local processes are killed and directories removed first
- **THEN** the Grail OpenPipeline routing entries for metrics, logs, and spans are removed
- **THEN** a confirmation line is printed for each successfully removed route
- **THEN** the extension environment configuration is deactivated
- **THEN** the extension version is deleted from the tenant
- **THEN** a confirmation line is printed indicating the extension was removed

#### Scenario: User selects Only collector — extension and routes kept on tenant

- **GIVEN** the experimental flag is enabled
- **WHEN** the user selects `[2] Only collector` at the prompt
- **THEN** local processes are killed and directories removed
- **THEN** no route removal or extension API call is made
- **THEN** the extension and Grail routes remain on the tenant

#### Scenario: User selects Cancel

- **GIVEN** the experimental flag is enabled
- **WHEN** the user selects `[3] Cancel` at the prompt
- **THEN** no local cleanup is performed
- **THEN** no extension API call is made

#### Scenario: Extension not present on tenant

- **GIVEN** the experimental flag is enabled
- **AND** the OpenTelemetry Host Monitoring extension is not installed on the tenant
- **WHEN** the user selects `[1] Delete all`
- **THEN** local cleanup proceeds normally
- **THEN** the missing extension is treated as success (nothing to remove)

#### Scenario: Grail route absent — treated as success

- **GIVEN** the experimental flag is enabled
- **AND** the OTel Host Monitoring extension pipeline or routing entry is not present on the tenant
- **WHEN** the user selects `[1] Delete all`
- **THEN** route removal is skipped for that signal without printing a warning
- **THEN** extension deactivation and version deletion proceed normally

#### Scenario: Grail route removal fails for one signal — advisory, other signals continue

- **GIVEN** the experimental flag is enabled
- **AND** the route removal API call fails for one signal (e.g. metrics) but succeeds for others
- **WHEN** the user selects `[1] Delete all`
- **THEN** a per-signal advisory warning is printed for the failing signal
- **THEN** route removal continues for the remaining signals
- **THEN** extension deactivation and version deletion proceed normally

#### Scenario: Extension removal fails

- **GIVEN** the experimental flag is enabled
- **AND** the extension API call fails (network error, token lacks permissions, etc.)
- **WHEN** the user selects `[1] Delete all`
- **THEN** local cleanup completes normally
- **THEN** a warning is printed indicating extension removal failed
- **THEN** the command exits with code 0 (failure is advisory, not fatal)

#### Scenario: Experimental flag disabled

- **GIVEN** the experimental flag is not enabled
- **WHEN** the user runs `dtwiz uninstall otel`
- **THEN** the original yes/no confirmation is shown
- **THEN** no extension removal is attempted
- **THEN** no credentials are required

#### Scenario: Dry run skips extension removal

- **GIVEN** the experimental flag is enabled
- **WHEN** the user runs `dtwiz uninstall otel --dry-run`
- **THEN** the preview shows the extension that would be removed
- **THEN** no prompt is shown
- **THEN** no extension removal API call is made

### Requirement: Uninstall preview includes extension removal when experimental is enabled

When `dtwiz uninstall otel` runs with the experimental flag enabled, the uninstall preview SHALL include a line identifying the extension that will be removed from the tenant.

#### Scenario: Preview shows extension and route removal when experimental is enabled

- **GIVEN** the experimental flag is enabled
- **WHEN** `dtwiz uninstall otel` renders the preview
- **THEN** the preview includes the name of the extension that would be deleted
- **THEN** the preview includes the Grail routes (metrics, logs, spans) that would be removed
- **THEN** the three-option prompt appears after the preview

#### Scenario: Preview omits extension removal when experimental is disabled

- **GIVEN** the experimental flag is not enabled
- **WHEN** `dtwiz uninstall otel` renders the preview
- **THEN** no extension-related line appears in the preview
- **THEN** the standard yes/no confirmation appears after the preview
