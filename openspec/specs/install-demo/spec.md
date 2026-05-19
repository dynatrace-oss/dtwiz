# Spec: install demo

## ADDED Requirements

### Requirement: Demo app download and extraction

The `install demo` command SHALL download the schnitzel demo app from its public ZIP URL and extract it to `./schnitzel/` in the current working directory.

#### Scenario: Demo directory does not exist

- **WHEN** `./schnitzel/` does not exist in the current working directory
- **THEN** the installer SHALL download the ZIP from `https://github.com/dietermayrhofer/schnitzel/archive/refs/heads/master.zip`
- **AND** extract it to `./schnitzel/` atomically (extract to temp dir, rename on success)

#### Scenario: Demo directory already exists

- **WHEN** `./schnitzel/` already exists in the current working directory
- **THEN** the installer SHALL skip the download and extraction step
- **AND** proceed directly to the OTel setup step

#### Scenario: Download fails

- **WHEN** the download of the ZIP fails (network error, 4xx/5xx response)
- **THEN** the installer SHALL exit with a clear error message including the URL and HTTP status or error detail

#### Scenario: Dry run

- **WHEN** `--dry-run` is passed
- **THEN** the installer SHALL show what would be downloaded and extracted without executing

---

### Requirement: Demo installation plan preview

Before executing any action, `install demo` SHALL display a compact plan and prompt the user for confirmation.

#### Scenario: Full install required (no schnitzel, no python)

- **WHEN** `./schnitzel/` does not exist and `python3` is not on PATH
- **THEN** the plan SHALL list all steps: download schnitzel, install Python, install OTel Collector, instrument schnitzel
- **AND** end with a single `Proceed with installation? [Y/n]` prompt (default yes)

#### Scenario: Partial install (schnitzel exists, python present)

- **WHEN** `./schnitzel/` already exists and `python3` is on PATH
- **THEN** the plan SHALL list only the remaining steps: install OTel Collector, instrument schnitzel
- **AND** omit steps that are already satisfied

#### Scenario: --yes flag skips confirmation

- **WHEN** `--yes` is passed
- **THEN** the installer SHALL skip the confirmation prompt and proceed immediately

---

### Requirement: OTel setup after demo preparation

After the demo app is ready, `install demo` SHALL invoke the OTel Collector installation followed by Python auto-instrumentation, targeting `./schnitzel/`.

#### Scenario: OTel install is called with project path and autoconfirm

- **WHEN** demo preparation completes successfully
- **THEN** `InstallOtelCollector` SHALL be called in-process with `--project ./schnitzel` and `AutoConfirm = true`
- **AND** no further project selection prompts SHALL be shown to the user

#### Scenario: OTel install fails

- **WHEN** `InstallOtelCollector` returns an error
- **THEN** `install demo` SHALL surface the error and exit with a non-zero status
