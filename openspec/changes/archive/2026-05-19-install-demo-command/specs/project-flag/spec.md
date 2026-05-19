# Spec: project flag

## ADDED Requirements

### Requirement: --project flag on install otel and install otel-python

`install otel` and `install otel-python` SHALL accept a `--project <path>` flag that skips interactive project scanning and selection, using the given path directly.

#### Scenario: --project provided to install otel

- **WHEN** `--project <path>` is passed to `install otel`
- **THEN** `InstallOtelCollector` SHALL skip `detectAllProjects` and `selectProject`
- **AND** SHALL detect the runtime from the files at `<path>`
- **AND** SHALL build the instrumentation plan directly from that path

#### Scenario: --project provided to install otel-python

- **WHEN** `--project <path>` is passed to `install otel-python`
- **THEN** `InstallOtelPython` SHALL use the provided path directly without scanning

#### Scenario: --project path does not exist

- **WHEN** `--project <path>` is passed and `<path>` does not exist on disk
- **THEN** the installer SHALL exit with a clear error: `project path not found: <path>`

#### Scenario: --project not provided (default behavior unchanged)

- **WHEN** `--project` is not passed
- **THEN** the existing scan → list → select flow SHALL run unchanged

#### Scenario: --project and --yes together

- **WHEN** both `--project <path>` and `--yes` are passed
- **THEN** the project is pre-selected and no confirmation prompt is shown
- **AND** installation proceeds fully non-interactively
