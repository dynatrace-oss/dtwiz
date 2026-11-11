# Spec Delta

## Purpose

Lets the bash and PowerShell install scripts run unattended, for example when an AI agent installs dtwiz during onboarding, by skipping the install confirmation in the same way the dtwiz binary's `--yes` flag does.

## ADDED Requirements

### Requirement: Install scripts accept an auto-confirm option

The bash install script SHALL accept `--yes` and its short form `-y`. The PowerShell install script SHALL accept `-Yes` and its short form `-y`. The option takes no value and can be combined with the scripts' other options in any order.

#### Scenario: Bash long form

- **GIVEN** the user runs the bash install script
- **WHEN** they pass `--yes`
- **THEN** the script accepts the argument and runs the install

#### Scenario: Bash short form with another option

- **GIVEN** the user runs the bash install script
- **WHEN** they pass `-y --install-dir "$HOME/bin"`
- **THEN** the script installs to `$HOME/bin` without asking for confirmation

#### Scenario: Bash script sourced with arguments

- **GIVEN** the user installs with the recommended `source <(curl -sSL .../install.sh)` form in bash or zsh
- **WHEN** they append `--yes` after the process substitution
- **THEN** the script receives `--yes` and installs without asking for confirmation

#### Scenario: Bash script piped to sh

- **GIVEN** the user installs with `curl -sSL .../install.sh | sh -s -- --yes`
- **WHEN** the script runs
- **THEN** the script installs without asking for confirmation

#### Scenario: PowerShell switch

- **GIVEN** the user runs the PowerShell install script as a script block, with `& ([scriptblock]::Create((irm .../install.ps1))) -Yes`, or from a local file with `.\install.ps1 -Yes`
- **WHEN** the script runs
- **THEN** the script installs without asking for confirmation

### Requirement: Auto-confirm skips only the install confirmation

When the auto-confirm option is passed, the install scripts SHALL NOT show the `Continue? [Y/n]` prompt and SHALL NOT read from the terminal for it. They SHALL proceed as if the user had confirmed. They SHALL still print the install preview (version, channel or tag, download source, install directory, PATH change) before proceeding.

#### Scenario: Preview is still shown

- **GIVEN** the auto-confirm option is passed
- **WHEN** the script reaches the confirmation step
- **THEN** it prints the same "This will download and install dtwiz ..." preview as without the option, prints no `Continue? [Y/n]` prompt, and continues to the download

#### Scenario: Bash without a terminal

- **GIVEN** the bash script runs in a session with no controlling terminal, as some agent shells do
- **WHEN** `--yes` is passed
- **THEN** the install completes and prints no error about the terminal being unavailable

#### Scenario: Non-interactive PowerShell

- **GIVEN** the PowerShell script runs in a session that cannot prompt, such as `pwsh -NonInteractive`
- **WHEN** `-Yes` is passed
- **THEN** the install completes instead of failing with `prompt_failed`

#### Scenario: Auto-confirmed run reports completed

- **GIVEN** a Dynatrace environment is configured for self-monitoring and the auto-confirm option is passed
- **WHEN** the install succeeds
- **THEN** the run sends its `invoked` event and then a `completed` event, and no `cancelled` event

#### Scenario: Elevated privileges are not bypassed

- **GIVEN** the bash script is run with `--yes` and an install directory that needs elevated privileges
- **WHEN** the binary is moved into place
- **THEN** the system's own privilege prompt (for example the sudo password prompt) still appears, because auto-confirm skips only the script's confirmation

### Requirement: Without auto-confirm the scripts prompt as before

When the auto-confirm option is not passed, the install scripts SHALL show the `Continue? [Y/n]` prompt and treat the answer exactly as before: Enter or yes proceeds, no cancels. No environment variable SHALL turn auto-confirm on.

#### Scenario: Default run prompts

- **GIVEN** the user runs either install script without the auto-confirm option
- **WHEN** the script reaches the confirmation step
- **THEN** it shows `Continue? [Y/n]` and waits for an answer

#### Scenario: Decline still cancels

- **GIVEN** the user runs either install script without the auto-confirm option
- **WHEN** they answer `n`
- **THEN** the script prints "Installation cancelled.", installs nothing, and exits successfully

#### Scenario: Environment does not enable auto-confirm

- **GIVEN** the user's environment has a variable such as `DTWIZ_YES=true` set
- **WHEN** they run either install script without the auto-confirm option
- **THEN** the script still shows the confirmation prompt

### Requirement: Unrecognized arguments are rejected

The install scripts SHALL refuse to run when given an argument they do not recognize, rather than reinterpreting it as another option's value. The bash script SHALL keep reporting this as `unknown_argument`. The PowerShell script SHALL accept its install directory only through the named `-InstallDir` parameter.

#### Scenario: Bash rejects an unknown flag

- **GIVEN** the user runs the bash install script
- **WHEN** they pass `--yess`
- **THEN** the script prints `Unknown argument: --yess`, installs nothing, and exits with a non-zero status

#### Scenario: PowerShell rejects a bash-style flag

- **GIVEN** the user runs the PowerShell install script
- **WHEN** they pass `--yes` instead of `-Yes`
- **THEN** PowerShell refuses to start the script with a parameter error, nothing is installed, and no directory named `--yes` is created

#### Scenario: PowerShell rejects a positional install directory

- **GIVEN** the user runs the PowerShell install script
- **WHEN** they pass an install directory without `-InstallDir`
- **THEN** PowerShell refuses to start the script with a parameter error and nothing is installed
