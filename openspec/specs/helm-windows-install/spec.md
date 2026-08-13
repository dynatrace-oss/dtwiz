# Helm Windows Install

## Purpose

Define how `dtwiz install kubernetes` auto-installs Helm on Windows via winget when Helm is not on PATH.

## Requirements

### Requirement: Helm auto-install on Windows via winget

When `dtwiz install kubernetes` runs on Windows and Helm is not found on PATH, the CLI SHALL attempt to install Helm using `winget install --id Helm.Helm -e --source winget` before proceeding.

#### Scenario: winget available and install succeeds

- **WHEN** the OS is Windows AND `helm` is not on PATH AND `winget` is on PATH
- **THEN** the CLI runs `winget install --id Helm.Helm -e --source winget` and continues with the Kubernetes installation

#### Scenario: winget not available on Windows

- **WHEN** the OS is Windows AND `helm` is not on PATH AND `winget` is not on PATH
- **THEN** the CLI returns an error that includes: the command `winget install --id Helm.Helm`, and the URL `https://helm.sh/docs/intro/install/`
- **THEN** the CLI does NOT attempt to run `bash`

#### Scenario: winget available but install fails

- **WHEN** the OS is Windows AND `helm` is not on PATH AND `winget` is on PATH AND `winget install` exits with a non-zero code
- **THEN** the CLI returns an error that includes the winget failure detail, the command `winget install --id Helm.Helm`, and the URL `https://helm.sh/docs/intro/install/`

### Requirement: Process PATH refreshed after winget install on Windows

After a successful winget Helm install, the CLI SHALL append the Windows registry user PATH to the current process PATH so that `helm` is immediately usable without a shell restart.

#### Scenario: PATH refreshed after successful winget install

- **WHEN** winget installs Helm successfully
- **THEN** the new Helm binary directory is appended to the current process PATH
- **THEN** subsequent helm invocations in the same process succeed without error

#### Scenario: PATH refresh failure is non-fatal

- **WHEN** winget installs Helm successfully AND the PATH refresh via PowerShell fails
- **THEN** the CLI prints a warning and continues
- **THEN** the install does not abort due to the PATH refresh failure

### Requirement: Windows PATH refreshed at startup of kubernetes commands

The CLI SHALL refresh the current process PATH from the Windows registry user PATH at the start of both `dtwiz install kubernetes` and `dtwiz uninstall kubernetes` so that a previously winget-installed Helm is found across separate dtwiz invocations.

#### Scenario: helm found on subsequent dtwiz run after prior winget install

- **WHEN** the OS is Windows AND `helm` was installed by winget in a previous dtwiz session AND the shell has not been restarted
- **THEN** `dtwiz install kubernetes` and `dtwiz uninstall kubernetes` both find `helm` on PATH

#### Scenario: PATH refresh is no-op on Unix

- **WHEN** the OS is not Windows
- **THEN** the PATH refresh call returns immediately without modifying the process environment

### Requirement: Unix Helm auto-install unchanged

The existing Unix auto-install path (bash + curl pipe) SHALL remain unchanged.

#### Scenario: Unix with helm missing

- **WHEN** the OS is not Windows AND `helm` is not on PATH
- **THEN** the CLI runs `bash -c "curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"` exactly as before
