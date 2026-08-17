# Spec: demo-python-prerequisites

## Purpose

Define how `dtwiz install demo` automatically installs all Python prerequisites before proceeding with OTel setup.

## Requirements

### Requirement: Auto-install all Python prerequisites for the demo flow

`dtwiz install demo` SHALL check for all Python prerequisites — interpreter, pip, and pip availability inside a fresh virtualenv — and install any that are missing via the platform package manager before proceeding with OTel setup.

#### Scenario: Python not on PATH — Debian/Ubuntu

- **WHEN** `python3` / `python` is not found in PATH
- **AND** `/etc/os-release` indicates Debian or Ubuntu
- **THEN** the plan SHALL include `sudo apt-get install -y python3 python3-pip python3-venv`

#### Scenario: Python present but pip missing — Debian/Ubuntu

- **WHEN** a Python 3 interpreter is found but `python -m pip` fails
- **AND** `/etc/os-release` indicates Debian or Ubuntu
- **THEN** the plan SHALL include `sudo apt-get install -y python3 python3-pip python3-venv`

#### Scenario: pip present globally but missing from fresh venv — Debian/Ubuntu

- **WHEN** `python -m pip` succeeds but a probe virtualenv does not contain pip
- **AND** `/etc/os-release` indicates Debian or Ubuntu
- **THEN** the plan SHALL include `sudo apt-get install -y python3 python3-pip python3-venv`
- **NOTE** this catches the case where `python3-venv` is not installed: venv creation succeeds but the resulting venv omits pip

#### Scenario: Prerequisites missing — RHEL/Fedora/CentOS

- **WHEN** any Python prerequisite (interpreter, pip, or pip-in-venv) is missing
- **AND** `/etc/os-release` indicates RHEL/Fedora/CentOS or the distro is unrecognised
- **THEN** the plan SHALL include `sudo dnf install -y python3 python3-pip python3-venv`

#### Scenario: All prerequisites present

- **WHEN** a Python 3 interpreter is on PATH, `python -m pip` succeeds, and a fresh virtualenv contains pip
- **THEN** no prerequisite install step is added to the plan
