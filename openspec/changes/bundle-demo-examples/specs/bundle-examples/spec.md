# Spec: bundle-examples

## ADDED Requirements

### Requirement: All examples are published as a release asset

The `examples/` directory SHALL be packaged as `dtwiz-examples.tar.gz` and published as a GitHub release asset alongside each dtwiz release. Schnitzel is one of the examples inside this archive.

#### Scenario: Release asset is available for the current version

- **WHEN** a dtwiz release is published
- **THEN** `dtwiz-examples.tar.gz` SHALL be available as a release asset for that version
- **AND** extracting it SHALL produce an `examples/` directory with subdirectories for each example app, including `schnitzel/`

---

### Requirement: Examples are downloaded to a fixed location on demand

When `~/.dtwiz/examples/schnitzel/` does not exist, the binary SHALL download `dtwiz-examples.tar.gz` from the current dtwiz GitHub release and extract it to `~/.dtwiz/examples/` before proceeding. The download URL is built from the binary's built-in version string.

The extraction path is:

- macOS and Linux: `$HOME/.dtwiz/examples/`
- Windows: `%USERPROFILE%\.dtwiz\examples\`

#### Scenario: Download creates the expected directory structure

- **WHEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** the demo command is run
- **THEN** the binary SHALL download `dtwiz-examples.tar.gz` from the release asset URL for the current version
- **AND** extract it to `~/.dtwiz/examples/`
- **AND** all example directories and their files SHALL be present after extraction

#### Scenario: Download is skipped when path already exists

- **WHEN** `~/.dtwiz/examples/schnitzel/` already exists on disk
- **AND** the demo command is run
- **THEN** the binary SHALL skip the download and use the existing files

#### Scenario: OTel Collector keeps running after examples are re-downloaded on upgrade

- **WHEN** dtwiz is upgraded and a new binary is installed
- **AND** an OTel Collector is already running and configured to instrument files in `~/.dtwiz/examples/schnitzel/`
- **THEN** the Collector SHALL continue running without interruption
- **AND** the updated Python source files SHALL be picked up the next time the schnitzel services are restarted

---

### Requirement: Demo files do not affect binary size for non-demo users

The dtwiz binary SHALL NOT contain any example app files. They are only fetched when a user explicitly runs `dtwiz install demo`.

#### Scenario: Demo works after manual binary installation

- **GIVEN** a user has placed the dtwiz binary on their PATH without using an install script
- **WHEN** the user runs `dtwiz install demo`
- **THEN** the demo command SHALL download `dtwiz-examples.tar.gz` from the release asset URL and proceed
- **AND** no files from `examples/` SHALL be embedded in the binary itself

#### Scenario: Demo works after upgrading by replacing the binary

- **GIVEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** the user has replaced the dtwiz binary with a newer version
- **WHEN** the user runs `dtwiz install demo`
- **THEN** the command SHALL download the release asset for the new version and proceed
