## ADDED Requirements

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

### Requirement: Unix Helm auto-install unchanged
The existing Unix auto-install path (bash + curl pipe) SHALL remain unchanged.

#### Scenario: Unix with helm missing
- **WHEN** the OS is not Windows AND `helm` is not on PATH
- **THEN** the CLI runs `bash -c "curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"` exactly as before
