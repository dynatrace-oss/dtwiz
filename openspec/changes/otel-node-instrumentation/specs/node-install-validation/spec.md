## ADDED Requirements

### Requirement: Pre-flight validation for Node.js installer
The `InstallOtelNode()` function SHALL validate prerequisites before proceeding.

#### Scenario: Node.js not in PATH
- **WHEN** `node` is not found in PATH
- **THEN** the installer SHALL exit with a clear error message indicating Node.js is required

#### Scenario: No package manager available
- **WHEN** `node` is found but neither `npm`, `yarn`, nor `pnpm` is available
- **THEN** the installer SHALL exit with a clear error message indicating a package manager is required

#### Scenario: All prerequisites met
- **WHEN** `node` and at least one package manager are in PATH
- **THEN** the installer SHALL proceed with the installation flow
