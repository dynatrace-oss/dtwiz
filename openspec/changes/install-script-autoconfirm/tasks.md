# Tasks

## 1. Bash install script (`scripts/install.sh`)

- [ ] 1.1 Initialize `AUTO_CONFIRM=""` next to `INSTALL_DIR=""` and add a `--yes|-y)` case to the argument loop. Verify that `sh scripts/install.sh --yess` still prints `Unknown argument: --yess` and exits non-zero, and that `bash -n scripts/install.sh` and `sh -n scripts/install.sh` report no syntax errors.
- [ ] 1.2 Wrap the `Continue? [Y/n]` `printf`/`read`/`case` block in a check that `AUTO_CONFIRM` is empty, and keep the preview lines unconditional. Verify that `bash scripts/install.sh --yes --install-dir "$(mktemp -d)"` prints the preview, prints no `Continue?` line, and installs the binary.
- [ ] 1.3 Update the header comment: document `--yes` / `-y` next to `--install-dir`, and show `source <(curl -sSL .../install.sh) --yes` and `curl -sSL .../install.sh | sh -s -- --yes` as the unattended forms. Verify by reading the header.
- [ ] 1.4 Run the bash verification matrix. Use `T="$(mktemp -d)"` per case, and delete the test directories afterwards. Every case passes:
  - Default run: `bash scripts/install.sh --install-dir "$T"` shows `Continue? [Y/n]`. Enter installs. `n` prints "Installation cancelled." and exits 0 with nothing in `$T`.
  - Short form in any order: `bash scripts/install.sh --install-dir "$T" -y` installs without a prompt.
  - Sourced in bash: `bash -c 'source <(cat scripts/install.sh) --yes --install-dir "'"$T"'"; command -v dtwiz'` installs without a prompt and resolves `dtwiz` from `$T`.
  - Sourced in zsh: the same command with `zsh -c` behaves the same way.
  - Piped: `cat scripts/install.sh | sh -s -- --yes --install-dir "$T"` installs without a prompt.
  - No terminal: run the `--yes` case from a session with no controlling terminal (on Linux `setsid -w bash scripts/install.sh --yes --install-dir "$T" </dev/null`, on macOS from the agent's non-TTY shell tool). It installs and prints no `/dev/tty` error.
  - Environment ignored: `DTWIZ_YES=true AUTO_CONFIRM=1 bash scripts/install.sh --install-dir "$T"` still shows the prompt.
  - Self-monitoring: with `DT_ENVIRONMENT` pointing at a test tenant, a `--yes` run produces an `invoked` and a `completed` "dtwiz install script" event with the same `executionId`, and no `cancelled` event.

## 2. PowerShell install script (`scripts/install.ps1`)

- [ ] 2.1 Change `[CmdletBinding()]` to `[CmdletBinding(PositionalBinding = $false)]` and add `[Alias('y')] [switch]$Yes` to `param()`. Verify on Windows that `.\install.ps1 C:\x` and `.\install.ps1 --yes` both fail with a parameter error before any output, and that no `--yes` or `C:\x` directory is created.
- [ ] 2.2 Skip the `Read-Host "Continue? [Y/n]"` block (and its `prompt_failed` assignment) when `$Yes` is set, and keep the preview `Write-Host` lines unconditional. Verify that `.\install.ps1 -Yes -InstallDir $T` prints the preview, prints no `Continue?` line, and installs `dtwiz.exe`.
- [ ] 2.3 Update the header comment: document `-Yes` / `-y` next to `-InstallDir`, and show `& ([scriptblock]::Create((irm .../install.ps1))) -Yes` as the unattended form, noting that `irm | iex` cannot pass parameters. Verify by reading the header.
- [ ] 2.4 Run the PowerShell verification matrix on Windows in both Windows PowerShell 5.1 (`powershell.exe`) and PowerShell 7 (`pwsh`). Use a fresh `$T = Join-Path $env:TEMP ([IO.Path]::GetRandomFileName())` per case. Every case passes:
  - Default run: `.\install.ps1 -InstallDir $T` shows `Continue? [Y/n]`. Enter installs. `n` prints "Installation cancelled." with nothing installed.
  - Short form: `.\install.ps1 -y -InstallDir $T` installs without a prompt.
  - Script block from the local file: `& ([scriptblock]::Create((Get-Content -Raw .\scripts\install.ps1))) -Yes -InstallDir $T` installs without a prompt and leaves the caller's `$ErrorActionPreference` unchanged.
  - Non-interactive: `pwsh -NonInteractive -File .\scripts\install.ps1 -Yes -InstallDir $T` installs. The same command without `-Yes` still fails with `prompt_failed`.
  - Environment ignored: with `$env:DTWIZ_YES="true"`, `.\install.ps1 -InstallDir $T` still prompts.
  - `irm https://raw.githubusercontent.com/dynatrace-oss/dtwiz/<branch>/scripts/install.ps1 | iex` without parameters still works and prompts.
  - Self-monitoring: with `DT_ENVIRONMENT` set, a `-Yes` run produces `invoked` and `completed` events and no `cancelled` event.

## 3. Documentation

- [ ] 3.1 In `README.md` under "Installation", add a short "Unattended install (CI, AI agents)" block with `source <(curl -sSL .../install.sh) --yes` and `& ([scriptblock]::Create((irm .../install.ps1))) -Yes`, and leave the Quickstart unchanged. Verify that the README renders and that both commands match the forms tested in 1.4 and 2.4.
- [ ] 3.2 In `skills/dtwiz/ONBOARDING.md` Step 1, switch the install commands to the auto-confirm forms, with one sentence noting that `--yes` / `-Yes` only skips the install confirmation. Verify that the wording matches the guide's existing `--yes` rules (around "`--yes` only skips the confirmation").
- [ ] 3.3 In `skills/dtwiz/SKILL.md`, update the two install commands (around lines 30–31) the same way. Verify with `rg -n "install\.(sh|ps1)" skills README.md` that every agent-facing command uses the auto-confirm form and every human-facing one does not.
- [ ] 3.4 Add `CHANGELOG.md` `[Unreleased]` entries. Under `Added`: `install.sh --yes`/`-y` and `install.ps1 -Yes`/`-y` skip the install confirmation for unattended installs. Under `Changed`: `install.ps1` no longer accepts the install directory positionally; use `-InstallDir`. Verify that the entries follow the existing Keep a Changelog style.

## 4. Integration check

- [ ] 4.1 With the branch pushed, run the agent onboarding Step 1 commands as written in `skills/dtwiz/ONBOARDING.md` from a Cursor agent shell on macOS and Windows, with the raw URL pointed at the branch (`.../dtwiz/<branch>/scripts/...`) instead of `main`. Verify that the install completes with no prompt and that `dtwiz version` runs in the same shell.

## Workflow follow-up

- Consider a scripted test harness for `scripts/` (stubbed `curl` for bash, Pester with a mocked `Invoke-WebRequest` for PowerShell) so the matrices above can run in CI.
- Archive the change after review with `/opsx:archive`.
