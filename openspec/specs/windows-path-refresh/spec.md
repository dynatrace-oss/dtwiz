# Windows PATH Refresh

## Purpose

Define how dtwiz refreshes the PATH environment variable inside the current process on Windows after installing a new tool.

## Requirements

### Requirement: Refresh PATH inside the current process

Installers that depend on Windows package-manager installs SHALL refresh the current process PATH from the Windows user PATH.

#### Scenario: Installer needs a newly installed Windows tool

- **WHEN** a Windows installer may need a tool that was just added to the user PATH
- **THEN** it SHALL call the shared PATH refresh helper before looking for that tool
- **AND** duplicate PATH entries SHALL not be added

#### Scenario: PATH refresh fails

- **WHEN** the Windows PATH refresh fails
- **THEN** the installer SHALL continue when possible and show or log a warning
