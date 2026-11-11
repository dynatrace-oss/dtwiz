# Tasks

## 1. Bash install script (`scripts/install.sh`)

- [x] 1.1 Initialize `AUTO_CONFIRM=""` next to `INSTALL_DIR=""` and add a `--yes|-y)` case to the argument loop. Verify that `sh scripts/install.sh --yess` still prints `Unknown argument: --yess` and exits non-zero, and that `bash -n scripts/install.sh` and `sh -n scripts/install.sh` report no syntax errors.
- [x] 1.2 Wrap the `Continue? [Y/n]` `printf`/`read`/`case` block in a check that `AUTO_CONFIRM` is empty, and keep the preview lines unconditional. Verify that `bash scripts/install.sh --yes --install-dir "$(mktemp -d)"` prints the preview, prints no `Continue?` line, and installs the binary.
- [x] 1.3 Update the header comment: document `--yes` / `-y` next to `--install-dir`, and show `source <(curl -sSL .../install.sh) --yes` and `curl -sSL .../install.sh | sh -s -- --yes` as the unattended forms. Verify by reading the header.

## 2. PowerShell install script (`scripts/install.ps1`)

- [x] 2.1 Change `[CmdletBinding()]` to `[CmdletBinding(PositionalBinding = $false)]` and add `[Alias('y')] [switch]$Yes` to `param()`. Verify on Windows that `.\install.ps1 C:\x` and `.\install.ps1 --yes` both fail with a parameter error before any output, and that no `--yes` or `C:\x` directory is created.
- [x] 2.2 Skip the `Read-Host "Continue? [Y/n]"` block (and its `prompt_failed` assignment) when `$Yes` is set, and keep the preview `Write-Host` lines unconditional. Verify that `.\install.ps1 -Yes -InstallDir $T` prints the preview, prints no `Continue?` line, and installs `dtwiz.exe`.
- [x] 2.3 Update the header comment: document `-Yes` / `-y` next to `-InstallDir`, and show `& ([scriptblock]::Create((irm .../install.ps1))) -Yes` as the unattended form, noting that `irm | iex` cannot pass parameters. Verify by reading the header.

## 3. Documentation

- [x] 3.1 In `README.md` under "Installation", add a short "Unattended install (CI, AI agents)" block with `source <(curl -sSL .../install.sh) --yes` and `& ([scriptblock]::Create((irm .../install.ps1))) -Yes`, and leave the Quickstart unchanged. Verify that the README renders and that both commands match the forms tested in 1.4 and 2.4.
- [x] 3.2 In `skills/dtwiz/ONBOARDING.md` Step 1, switch the install commands to the auto-confirm forms, with one sentence noting that `--yes` / `-Yes` only skips the install confirmation. Verify that the wording matches the guide's existing `--yes` rules (around "`--yes` only skips the confirmation").
- [x] 3.3 In `skills/dtwiz/SKILL.md`, update the two install commands (around lines 30–31) the same way. Verify with `rg -n "install\.(sh|ps1)" skills README.md` that every agent-facing command uses the auto-confirm form and every human-facing one does not.
- [x] 3.4 Add `CHANGELOG.md` `[Unreleased]` entries. Under `Added`: `install.sh --yes`/`-y` and `install.ps1 -Yes`/`-y` skip the install confirmation for unattended installs. Under `Changed`: `install.ps1` no longer accepts the install directory positionally; use `-InstallDir`. Verify that the entries follow the existing Keep a Changelog style.
