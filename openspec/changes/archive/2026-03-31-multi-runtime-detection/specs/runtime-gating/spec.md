# Runtime Gating

## ADDED Requirements

### Requirement: Coming-soon runtimes excluded from project scanning

Only Python is considered GA for runtime instrumentation. All other runtimes (Java, Node.js, Go) are "coming soon" by default — their projects SHALL NOT be scanned or shown. The `enabled` flag is controlled by a package-level function that checks the `DTWIZ_ALL_RUNTIMES` environment variable. When `DTWIZ_ALL_RUNTIMES=true` is set, all runtimes are treated as GA, enabling end-to-end testing of unreleased runtimes.

#### Scenario: Default behavior — only Python projects shown

- **GIVEN** the `DTWIZ_ALL_RUNTIMES` env var is not set
- **WHEN** `python3`, `java`, `node`, and `go` are on PATH
- **THEN** only Python projects appear in the unified list (Java, Node.js, Go projects are not scanned)

#### Scenario: All runtimes unlocked for testing

- **GIVEN** `DTWIZ_ALL_RUNTIMES=true` is set in the environment
- **WHEN** `python3`, `java`, `node`, and `go` are on PATH
- **THEN** projects from all four runtimes appear in the unified list

### Requirement: Dry-run support

When `--dry-run` is set, the unified project list and combined preview SHALL be printed but no collector or instrumentation SHALL be installed.

#### Scenario: Dry-run with projects detected

- **GIVEN** `--dry-run` is set on the `install otel` command
- **WHEN** projects are detected across GA runtimes
- **THEN** the system prints the unified project list, shows the collector dry-run plan, and exits without installing anything

#### Scenario: Dry-run without projects

- **GIVEN** `--dry-run` is set on the `install otel` command
- **WHEN** no projects are detected
- **THEN** the system prints the collector-only dry-run plan as before

### Requirement: Collector-only path unaffected by runtime detection

The collector-only installation path SHALL install the OTel Collector without presenting a project list, prompting for runtime selection, or performing any instrumentation.

#### Scenario: Collector-only install skips runtime detection

- **GIVEN** a collector-only installation is initiated
- **WHEN** the installation runs
- **THEN** no project list is shown, no runtime is selected, and only the collector is installed
