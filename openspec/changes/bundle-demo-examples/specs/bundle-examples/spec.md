# Spec: bundle-examples

## ADDED Requirements

### Requirement: Example apps are embedded in the binary

The `examples/schnitzel/` directory SHALL be embedded in the dtwiz binary at compile time using Go's `embed` package. A dedicated package SHALL expose the embedded filesystem so other packages can access it.

#### Scenario: Binary contains schnitzel example files

- **WHEN** the dtwiz binary is built
- **THEN** it SHALL contain all files from `examples/schnitzel/` embedded at compile time
- **AND** the embedded files SHALL be accessible at runtime without any network or filesystem access

---

### Requirement: Examples are extracted to a fixed location on demand

When `~/.dtwiz/examples/schnitzel/` does not exist, the binary SHALL extract the embedded examples there before proceeding. The extraction path is:

- macOS and Linux: `$HOME/.dtwiz/examples/schnitzel/`
- Windows: `%USERPROFILE%\.dtwiz\examples\schnitzel\`

#### Scenario: Extraction creates the expected directory structure

- **WHEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** the demo command is run
- **THEN** the binary SHALL extract all embedded schnitzel files to `~/.dtwiz/examples/schnitzel/`
- **AND** all files and subdirectories SHALL be present after extraction

#### Scenario: Extraction is skipped when path already exists

- **WHEN** `~/.dtwiz/examples/schnitzel/` already exists on disk
- **AND** demo command is run
- **THEN** the binary SHALL skip extraction and use the existing files

#### Scenario: OTel Collector keeps running after examples are re-extracted on upgrade

- **WHEN** dtwiz is upgraded and a new binary is installed
- **AND** an OTel Collector is already running and configured to instrument files in `~/.dtwiz/examples/schnitzel/`
- **THEN** the Collector SHALL continue running without interruption
- **AND** the updated Python source files SHALL be picked up the next time the schnitzel services are restarted

---

### Requirement: Binary is self-contained for demo use

The dtwiz binary SHALL work for the demo command without requiring an install script, network access, or any separately distributed file.

#### Scenario: Demo works after manual binary installation with no network calls to fetch the app

- **GIVEN** a user has placed the dtwiz binary on their PATH without using an install script
- **WHEN** the user runs `dtwiz install demo`
- **THEN** the demo command SHALL extract the embedded schnitzel files and proceed
- **AND** no network request SHALL be made to fetch the demo app

#### Scenario: Demo works after upgrading by replacing the binary

- **GIVEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** the user has replaced the dtwiz binary with a newer version
- **WHEN** the user runs `dtwiz install demo`
- **THEN** the command SHALL extract the embedded files from the new binary and proceed
