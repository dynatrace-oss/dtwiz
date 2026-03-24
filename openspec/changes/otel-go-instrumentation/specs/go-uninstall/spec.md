## ADDED Requirements

### Requirement: Uninstall OTel Go dependencies
The system SHALL provide a `dtwiz uninstall otel-go` command that removes OTel Go SDK packages from `go.mod`.

#### Scenario: OTel dependencies present
- **WHEN** the user runs `dtwiz uninstall otel-go` in a Go project that has OTel packages in `go.mod`
- **THEN** the system SHALL list the OTel packages, prompt for confirmation, remove them via `go mod edit -droprequire`, and run `go mod tidy`

#### Scenario: No OTel dependencies found
- **WHEN** no OTel packages are found in `go.mod`
- **THEN** the system SHALL inform the user that no OTel dependencies were found

#### Scenario: Dry-run mode
- **WHEN** `--dry-run` is set
- **THEN** the system SHALL list the packages that would be removed without taking action

#### Scenario: Confirmation prompt
- **WHEN** OTel packages are found for removal
- **THEN** the system SHALL show a preview and prompt `Apply? [Y/n]` before proceeding
