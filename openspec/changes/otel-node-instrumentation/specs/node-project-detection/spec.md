## ADDED Requirements

### Requirement: Detect Node.js projects
The system SHALL detect Node.js projects by scanning for `package.json` files in common development directories.

#### Scenario: Projects found in working directory
- **WHEN** the user runs `dtwiz install otel-node` and `package.json` files exist in the current directory or immediate subdirectories
- **THEN** the system SHALL list them with project name (from `package.json` `name` field) and path

#### Scenario: Projects found in common dev directories
- **WHEN** no projects are found in the current directory but `package.json` files exist in `$HOME/Code`, `$HOME/projects`, `$HOME/src`, or `$HOME/dev`
- **THEN** the system SHALL scan those directories (two levels deep) and list found projects

#### Scenario: Entrypoint detection
- **WHEN** a project is selected
- **THEN** the system SHALL identify the entrypoint from `main` field in `package.json`, `scripts.start` field, or common filenames (`index.js`, `server.js`, `app.js`, `main.js`)

#### Scenario: No projects found
- **WHEN** no `package.json` files are found in any scanned location
- **THEN** the system SHALL inform the user and exit with a helpful message
