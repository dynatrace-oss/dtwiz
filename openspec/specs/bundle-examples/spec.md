# Spec: bundle-examples

## Purpose

Define how dtwiz examples are packaged and published as a release asset alongside each dtwiz release.

## Requirements

### Requirement: All examples are published as a release asset

The `examples/` directory SHALL be packaged as `dtwiz-examples.tar.gz` and published as a GitHub release asset alongside each dtwiz release. Schnitzel is one of the examples inside this archive.

#### Scenario: Release asset is available for the current version

- **WHEN** a dtwiz release is published
- **THEN** `dtwiz-examples.tar.gz` SHALL be available as a release asset for that version
- **AND** extracting it SHALL produce an `examples/` directory with subdirectories for each example app, including `schnitzel/`

---

### Requirement: Examples are downloaded to a fixed location on demand

When `~/.dtwiz/examples/schnitzel/` does not exist, the binary SHALL download `dtwiz-examples.tar.gz` from a dtwiz GitHub release and extract it to `~/.dtwiz/examples/` before proceeding. Release builds build the download URL from the binary's built-in version string. Snapshot builds published by CI have a `SnapshotTag` injected at build time and use the corresponding snapshot pre-release URL. Local dev builds (no `SnapshotTag`) fall back to the latest stable release asset.

The extraction path is:

- macOS and Linux: `$HOME/.dtwiz/examples/`
- Windows: `%USERPROFILE%\.dtwiz\examples\`

#### Scenario: Download creates the expected directory structure

- **WHEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** the binary is a release build
- **THEN** the binary SHALL download `dtwiz-examples.tar.gz` from the release asset URL for the current version
- **AND** extract it to `~/.dtwiz/examples/`
- **AND** all example directories and their files SHALL be present after extraction

#### Scenario: Snapshot build uses its own pre-release asset

- **WHEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** the binary is a snapshot build with `SnapshotTag` set
- **THEN** the binary SHALL download `dtwiz-examples.tar.gz` from the snapshot pre-release URL
- **AND** extract it to `~/.dtwiz/examples/`

#### Scenario: Local dev build uses latest stable release asset

- **WHEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** the binary is a local dev build with no `SnapshotTag`
- **THEN** the binary SHALL download `dtwiz-examples.tar.gz` from the latest stable release asset URL
- **AND** extract it to `~/.dtwiz/examples/`

#### Scenario: Download is skipped when path already exists

- **WHEN** `~/.dtwiz/examples/schnitzel/` already exists on disk
- **AND** the demo command is run
- **THEN** the binary SHALL skip the download and use the existing files

#### Scenario: OTel Collector keeps running after dtwiz upgrade

- **WHEN** dtwiz is upgraded and a new binary is installed
- **AND** an OTel Collector is already running and configured to instrument files in `~/.dtwiz/examples/schnitzel/`
- **THEN** the Collector SHALL continue running without interruption
- **AND** existing files under `~/.dtwiz/examples/schnitzel/` SHALL NOT be modified by the binary upgrade

---

### Requirement: Demo files do not affect binary size for non-demo users

The dtwiz binary and normal OS-specific install archives SHALL NOT contain any example app files. Install scripts SHALL NOT copy examples during normal dtwiz installation. Examples are only fetched when a user explicitly runs `dtwiz install demo`.

#### Scenario: Demo works after manual binary installation

- **GIVEN** a user has placed the dtwiz binary on their PATH without using an install script
- **WHEN** the user runs `dtwiz install demo`
- **THEN** the demo command SHALL download `dtwiz-examples.tar.gz` from the release asset URL and proceed
- **AND** no files from `examples/` SHALL be embedded in the binary itself

#### Scenario: Install scripts do not install examples eagerly

- **WHEN** a user installs dtwiz through an install script
- **THEN** only the dtwiz binary and normal release archive contents SHALL be installed
- **AND** `~/.dtwiz/examples/` SHALL NOT be created by the install script

#### Scenario: Local release asset artifact is kept out of version control

- **WHEN** the release process creates `dtwiz-examples.tar.gz` for publication
- **THEN** the file SHALL be created in the repository working directory by a GoReleaser `before` hook
- **AND** the file SHALL be listed in `.gitignore` so it is never committed to the repository

#### Scenario: Demo works after upgrading by replacing the binary

- **GIVEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** the user has replaced the dtwiz binary with a newer version
- **WHEN** the user runs `dtwiz install demo`
- **THEN** the command SHALL download the release asset for the new version and proceed
