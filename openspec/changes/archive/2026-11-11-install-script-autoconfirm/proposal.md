# Proposal

## Why

The agent onboarding flow (`skills/dtwiz/ONBOARDING.md`) installs dtwiz by running `install.sh` or `install.ps1`, and both scripts stop at a `Continue? [Y/n]` prompt that an agent must not answer on the user's behalf. In a terminal the agent drives, the bash script waits on the terminal indefinitely. In a non-interactive PowerShell session the prompt throws and the install fails with `prompt_failed`. Every `dtwiz install`, `update` and `uninstall` command already has `--yes`/`-y` to skip its confirmation. The install scripts are the one step of agentic onboarding that still cannot run unattended.

## What Changes

- `install.sh` accepts `--yes` / `-y`. With it, the script still prints the install preview but skips the `Continue? [Y/n]` prompt and proceeds as if the user had pressed Enter. It reads nothing from the terminal.
- `install.ps1` accepts `-Yes` / `-y`, a switch parameter with the same behavior. Because `irm ... | iex` cannot pass parameters, the documented unattended Windows form is `& ([scriptblock]::Create((irm .../install.ps1))) -Yes`.
- `install.ps1` stops binding arguments by position, so a mistyped `--yes` is rejected instead of silently becoming the install directory. `-InstallDir` keeps working when passed by name, which is the only form documented. **BREAKING** for anyone who passed the install directory positionally (`.\install.ps1 C:\tools`); this form was never documented.
- No environment variable is added. Auto-confirm is opt-in per run, like the dtwiz binary's `--yes`.
- Without the flag, both scripts behave exactly as today.
- Docs: script header comments, the README installation section, and the agent onboarding guide and skill (`skills/dtwiz/ONBOARDING.md`, `skills/dtwiz/SKILL.md`) show the unattended form.

Not gated by a feature flag: the flag is opt-in and has no effect unless passed.

## Capabilities

### New Capabilities

- `install-script-autoconfirm`: how the bash and PowerShell install scripts accept an auto-confirm option, what it skips, what it keeps (the install preview), and how unrecognized arguments are rejected.

### Modified Capabilities

None. `autoconfirm-flag` covers the binary's command groups only and stays unchanged. `install-script-selfmonitoring` already covers the events an auto-confirmed run sends: it is invoked and then completes or fails. It is never cancelled at the prompt.

## Impact

- `scripts/install.sh`: argument parsing, the confirmation block, and the usage header comment.
- `scripts/install.ps1`: the `param()` block, `[CmdletBinding()]`, the confirmation block, and the usage header comment.
- `README.md` (Installation section), `skills/dtwiz/ONBOARDING.md` (Step 1), `skills/dtwiz/SKILL.md` (install commands), `CHANGELOG.md` (`[Unreleased]`).
- No Go code, no dependencies, no self-monitoring event changes.
- Rollback: revert the commit. The scripts are served from `main` via raw GitHub URLs, so a revert takes effect for new runs immediately. Callers that pass `--yes` would then fail with `unknown_argument` (bash) or a parameter-binding error (PowerShell), so revert the onboarding docs in the same commit.
