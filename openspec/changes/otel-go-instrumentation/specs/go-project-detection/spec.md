## ADDED Requirements

### Requirement: Detect Go projects
The system SHALL detect Go projects by scanning for `go.mod` files in common development directories.

#### Scenario: Projects found in working directory
- **WHEN** the user runs `dtwiz install otel-go` and `go.mod` files exist in the current directory or immediate subdirectories
- **THEN** the system SHALL list them with module name (from `go.mod`) and path

#### Scenario: Projects found in common dev directories
- **WHEN** no projects are found in the current directory but `go.mod` files exist in `$HOME/Code`, `$HOME/projects`, `$HOME/src`, or `$HOME/dev`
- **THEN** the system SHALL scan those directories (two levels deep) and list found projects

#### Scenario: No projects found
- **WHEN** no `go.mod` files are found in any scanned location
- **THEN** the system SHALL inform the user and exit with a helpful message
