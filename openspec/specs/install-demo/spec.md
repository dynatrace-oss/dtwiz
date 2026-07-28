# Spec: install demo

## ADDED Requirements

### Requirement: Demo installation plan preview

Before executing any action, `install demo` SHALL display a compact plan and prompt the user for confirmation.

#### Scenario: Full install required (python not present, path missing)

- **WHEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** `python3` is not on PATH
- **THEN** the plan SHALL list the steps: download schnitzel from release, install Python, install OTel Collector, instrument the demo app
- **AND** end with a single `Proceed with installation? [Y/n]` prompt (default yes)

#### Scenario: Path missing, python present

- **WHEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **AND** `python3` is already on PATH
- **THEN** the plan SHALL list the steps: download schnitzel from release, install OTel Collector, instrument the demo app
- **AND** omit the Python install step

#### Scenario: Path present, python not present

- **WHEN** `~/.dtwiz/examples/schnitzel/` already exists
- **AND** `python3` is not on PATH
- **THEN** the plan SHALL list the steps: install Python, install OTel Collector, instrument the demo app
- **AND** omit the extraction step

#### Scenario: Path present, python present

- **WHEN** `~/.dtwiz/examples/schnitzel/` already exists
- **AND** `python3` is already on PATH
- **THEN** the plan SHALL list only: install OTel Collector, instrument the demo app
- **AND** omit both the extraction and Python install steps

#### Scenario: Dry run

- **WHEN** `--dry-run` is passed
- **THEN** the plan SHALL be printed
- **AND** the command SHALL exit without making any changes

---

### Requirement: OTel setup targeting bundled demo app

After confirming, `install demo` SHALL invoke the OTel Collector installation followed by Python auto-instrumentation, targeting `~/.dtwiz/examples/schnitzel/`.

#### Scenario: Path present, no download needed

- **WHEN** `~/.dtwiz/examples/schnitzel/` exists on the user's machine
- **THEN** `InstallOtelCollector` SHALL be called with the absolute bundled path and `AutoConfirm = true`
- **AND** no project selection prompts SHALL be shown to the user

#### Scenario: Path missing, download from release then proceed

- **WHEN** `~/.dtwiz/examples/schnitzel/` does not exist
- **THEN** the binary SHALL download `dtwiz-examples.tar.gz` from the current dtwiz release and extract it to `~/.dtwiz/examples/`
- **AND** `InstallOtelCollector` SHALL then be called with that path and `AutoConfirm = true`

#### Scenario: Python installation fails

- **WHEN** Python is not on PATH
- **AND** the platform package manager fails to install it (e.g. brew, apt-get, or winget returns an error)
- **THEN** the command SHALL exit with an error message that includes the root cause
- **AND** no OTel steps SHALL be attempted

#### Scenario: OTel install fails

- **WHEN** `InstallOtelCollector` returns an error
- **THEN** `install demo` SHALL surface the error and exit with a non-zero status

---

### Requirement: Install demo option is hidden in setup when demo is already monitored

When the user runs `dtwiz setup` and the schnitzel demo services are already actively running, the `[d] Install demo app` option SHALL NOT be shown. The presence of the `~/.dtwiz/examples/schnitzel/` directory alone is not sufficient: it may exist from a previous demo install or manual extraction even when the demo is not running.

#### Scenario: Install demo option hidden when demo services are running

- **WHEN** Python processes from `~/.dtwiz/examples/schnitzel/` are actively running
- **THEN** the `[d] Install demo app` option SHALL NOT appear in the setup menu

#### Scenario: Install demo option shown when demo not yet running

- **WHEN** no Python processes from `~/.dtwiz/examples/schnitzel/` are running
- **THEN** the `[d] Install demo app` option SHALL appear in the setup menu
- **AND** this applies regardless of whether `~/.dtwiz/examples/schnitzel/` already exists on disk

---

### Requirement: Uninstalling OTel leaves the extracted demo files in place

Running `dtwiz uninstall otel` SHALL NOT remove `~/.dtwiz/examples/schnitzel/`. The extracted files are owned by the user and may contain modifications. Cleanup is the user's responsibility.

#### Scenario: Demo files remain after OTel uninstall

- **WHEN** the user runs `dtwiz uninstall otel` after having run `dtwiz install demo`
- **THEN** the OTel Collector and instrumentation configuration SHALL be removed
- **AND** `~/.dtwiz/examples/schnitzel/` SHALL remain on disk unchanged

---

### Requirement: Demo install produces a working instrumented setup

Running `dtwiz install demo` to completion SHALL result in an OTel Collector configured to instrument the schnitzel app, and the user SHALL be able to start the schnitzel services and generate traces.

#### Scenario: After demo install, schnitzel services can be started and traced

- **GIVEN** `dtwiz install demo` has completed without error
- **WHEN** the user starts the schnitzel services from `~/.dtwiz/examples/schnitzel/`
- **THEN** the OTel Collector SHALL receive traces from the schnitzel services
- **AND** traces SHALL be forwarded to the configured Dynatrace environment

#### Scenario: Re-running install demo on the same machine does not break the setup

- **GIVEN** `dtwiz install demo` has already been run and the OTel Collector is running
- **WHEN** the user runs `dtwiz install demo` again
- **THEN** the command SHALL complete without error
- **AND** the resulting setup SHALL be equivalent to a clean first install
