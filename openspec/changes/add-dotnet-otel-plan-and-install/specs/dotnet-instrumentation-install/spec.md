# Spec Delta

## Purpose

Define step 3 of .NET OpenTelemetry auto-instrumentation: how dtwiz selects, downloads, verifies and installs the upstream OpenTelemetry .NET automatic instrumentation into a directory it owns, and how an existing installation is reused or replaced.

## ADDED Requirements

### Requirement: Archive chosen for the platform

The system SHALL choose the upstream archive matching the operating system, C library (glibc or musl on Linux) and processor architecture of the machine, before any download starts. Combinations the upstream release does not provide SHALL be rejected with a message naming the supported combinations and SHALL end in the manual-steps outcome.

#### Scenario: Linux glibc x64

- **GIVEN** a Linux x64 machine using glibc
- **WHEN** installation is planned
- **THEN** the Linux glibc x64 archive is selected

#### Scenario: Linux musl arm64

- **GIVEN** a Linux arm64 machine using musl (for example Alpine)
- **WHEN** installation is planned
- **THEN** the Linux musl arm64 archive is selected

#### Scenario: macOS on Apple silicon

- **GIVEN** a macOS machine with an Apple silicon processor
- **WHEN** installation is planned
- **THEN** the macOS archive is selected

#### Scenario: Windows x64

- **GIVEN** a Windows x64 machine
- **WHEN** installation is planned
- **THEN** the Windows archive is selected

#### Scenario: Unsupported combination

- **GIVEN** macOS on an Intel processor, or Windows on arm64, or an unknown C library
- **WHEN** installation is planned
- **THEN** the user is told the platform is not supported by the upstream release and which platforms are
- **AND** nothing is downloaded

### Requirement: Latest release is downloaded

When no usable installation exists, the system SHALL install the latest upstream release and SHALL record the installed version. The download SHALL happen only after the plan was confirmed.

#### Scenario: Fresh install

- **GIVEN** no installation exists and the plan was confirmed
- **WHEN** the plan is executed
- **THEN** the latest release archive is downloaded and installed
- **AND** the installed version is recorded

#### Scenario: Not confirmed

- **GIVEN** the user declined the plan
- **WHEN** the command ends
- **THEN** nothing was downloaded

### Requirement: Isolated install location

The system SHALL install into a directory owned by dtwiz under the user's home directory (the user profile on Windows), never into system-wide locations, the global .NET installation or the project, and SHALL NOT require elevated privileges.

#### Scenario: Default location

- **GIVEN** no override is configured
- **WHEN** the instrumentation is installed
- **THEN** its files are inside a dtwiz-owned directory in the user's home directory
- **AND** no elevation prompt appears

#### Scenario: Files outside the directory

- **GIVEN** an installation completed
- **WHEN** the machine is inspected
- **THEN** no file was created outside the dtwiz-owned directory except temporary files that were removed

### Requirement: Integrity verification before extraction

The system SHALL verify the downloaded archive against the checksum published with the same release before extracting it. It SHALL abort when the checksum is missing or does not match. When the GitHub CLI is installed and signed in, it SHALL additionally verify the release attestation and abort when that fails. A failed stronger check SHALL NOT fall back to a weaker one, and a missing or unauthenticated GitHub CLI SHALL NOT produce an error.

#### Scenario: Checksum matches

- **GIVEN** the downloaded archive matches the published checksum
- **WHEN** verification runs
- **THEN** extraction proceeds

#### Scenario: Checksum mismatch

- **GIVEN** the archive does not match the published checksum
- **WHEN** verification runs
- **THEN** installation aborts with an error naming the archive
- **AND** nothing is extracted and an existing installation is untouched

#### Scenario: Checksum not published

- **GIVEN** the release has no checksum entry for the chosen archive
- **WHEN** verification runs
- **THEN** installation aborts and nothing is extracted

#### Scenario: GitHub CLI unavailable

- **GIVEN** the GitHub CLI is not installed or not signed in
- **WHEN** verification runs
- **THEN** only the checksum is verified
- **AND** no error or warning about the missing attestation check is shown

#### Scenario: Attestation fails

- **GIVEN** the GitHub CLI is usable
- **AND** attestation verification fails
- **WHEN** verification runs
- **THEN** installation aborts although the checksum matched

### Requirement: Safe extraction and layout validation

The system SHALL extract the archive only below the install directory, rejecting entries that would escape it or be unreasonably large, and SHALL validate that the extracted layout contains the files the instrumentation needs for the platform. An invalid layout SHALL abort the installation with an error that names the missing item.

#### Scenario: Archive entry escapes the directory

- **GIVEN** an archive entry whose path points outside the target directory
- **WHEN** extraction runs
- **THEN** installation aborts and nothing outside the target directory is written

#### Scenario: Oversized archive

- **GIVEN** an archive whose uncompressed content far exceeds the size of real releases
- **WHEN** extraction runs
- **THEN** installation aborts and the partial content is removed

#### Scenario: Layout changed upstream

- **GIVEN** the extracted archive lacks an expected file
- **WHEN** validation runs
- **THEN** installation aborts naming the missing file
- **AND** the instrumentation is not reported as installed

### Requirement: Atomic replacement

The system SHALL make an installation visible only when it is complete and valid, so that a failed or interrupted install or update leaves the previous installation, or none, intact.

#### Scenario: Failure while replacing an unusable installation

- **GIVEN** the install directory holds an unusable installation and installing the latest release fails during extraction
- **WHEN** the command ends
- **THEN** the previous content of the install directory is untouched

#### Scenario: Interrupted first install

- **GIVEN** no installation exists
- **AND** the user interrupts the install
- **WHEN** the command ends
- **THEN** no partial installation remains in the install directory

### Requirement: Reuse of an existing installation

When a usable installation exists, the system SHALL use it as it is: it SHALL NOT look up the latest release, download, verify or replace anything, and SHALL NOT need network access. An installation is usable when its layout is valid for the platform. The system SHALL NOT update an installation by itself.

#### Scenario: Installation present

- **GIVEN** a usable installation exists
- **WHEN** the plan is executed
- **THEN** nothing is downloaded or verified and the installed version is reported

#### Scenario: Newer release exists

- **GIVEN** a usable installation of an older version exists and a newer release was published
- **WHEN** the plan is executed
- **THEN** the installed version is used and no newer release is looked up

#### Scenario: Installation present without network

- **GIVEN** a usable installation exists and the release host cannot be reached
- **WHEN** the plan is executed
- **THEN** the installed version is used without error

#### Scenario: Installation is unusable

- **GIVEN** the install directory exists but its layout is invalid for the platform
- **WHEN** the plan is executed
- **THEN** the latest release is installed as for a fresh install and replaces the unusable content only after it was verified and validated

#### Scenario: Release unreachable without an installation

- **GIVEN** the release cannot be reached and no usable installation exists
- **WHEN** installation is planned
- **THEN** the command fails with an error that says to check the network connection and names the manual-steps walkthrough

### Requirement: Failures are classified

Installation failures SHALL name the stage that failed (release lookup, download, verification, extraction, layout validation) and SHALL distinguish user-fixable causes (no network, no disk space, no permission to write the directory) from other errors.

#### Scenario: Not writable directory

- **GIVEN** the install directory cannot be written
- **WHEN** installation runs
- **THEN** the error names the directory and the permission problem

#### Scenario: Network failure during download

- **GIVEN** the connection drops during the download
- **WHEN** installation runs
- **THEN** the error names the download stage and no partial file remains
