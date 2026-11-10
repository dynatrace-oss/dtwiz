# Design

## Context

See proposal.md for the motivation. The current state is as follows:

- `scripts/install.sh` parses arguments in a `while`/`case` loop that only knows `--install-dir` and fails with `unknown_argument` on anything else. The confirmation is `printf 'Continue? [Y/n] '` plus `read -r REPLY </dev/tty`. When there is no controlling terminal, the redirection fails, the shell prints an error, `REPLY` stays empty, and the script carries on as if confirmed. So an agent shell without a TTY already "works", but noisily and only by accident. An agent shell with a PTY waits forever.
- `scripts/install.ps1` has `[CmdletBinding()] param([string]$InstallDir, [string]$Tag, [string]$Channel)`. The confirmation is `Read-Host`, which throws in non-interactive sessions. The `catch` block reports that as `prompt_failed`.
- The documented Windows invocation `irm .../install.ps1 | iex` runs the script text through `Invoke-Expression`, so no parameters can be passed. `Tag` and `Channel` work there only because they default to `$env:DTWIZ_TAG` and `$env:DTWIZ_CHANNEL`.
- The binary's equivalent (`installer.AutoConfirm` in `pkg/installer/installer.go`) returns before printing the prompt and prints nothing in its place. The preview is still printed by each installer.
- No CI job runs, lints or tests the install scripts, and neither `shellcheck` nor `pwsh` is part of the dev setup.

## Goals / Non-Goals

**Goals:**

- Both scripts can run start to finish with no terminal input when the auto-confirm option is passed.
- The flag names and behavior match the binary's `--yes`/`-y` as closely as each shell allows.
- Agents get a single copy-paste command per OS in the onboarding guide.

**Non-Goals:**

- No `DTWIZ_YES` (or similar) environment variable. This was decided with the user: auto-confirm stays an explicit per-run choice, and a value left over in the environment can never auto-confirm a later run.
- No self-monitoring change. Events do not record whether a run was auto-confirmed. This was also decided with the user.
- No change to the no-TTY behavior when `--yes` is *not* passed. Making the bash script fail cleanly without a terminal, or making PowerShell's `prompt_failed` friendlier, is a separate concern.
- Not bypassing `sudo`. With the default install directory selection, sudo is only needed when the user passes an unwritable `--install-dir`.
- No automated test harness for the scripts. There is none today, and building one (stubbed `curl`/`Invoke-WebRequest`, Pester) is larger than this change. Verification is a scripted manual matrix in tasks.md.

## Decisions

### Bash: add `--yes|-y` to the existing `case`, guard the prompt

Add `--yes|-y) AUTO_CONFIRM=1; shift ;;` to the parse loop and wrap the `printf`/`read`/`case` block in `if [ -z "$AUTO_CONFIRM" ]`. Initialize `AUTO_CONFIRM=""` next to `INSTALL_DIR=""`. That way a value in the caller's environment, or one left by an earlier sourced run in the same shell, cannot turn it on. When auto-confirmed, the script prints nothing in place of the prompt, as the binary does. The preview's trailing blank line is kept, so the output just reads preview, blank line, "Downloading dtwiz ...".

Argument passing in the documented forms:

- `source <(curl -sSL .../install.sh) --yes`: bash and zsh both set the sourced file's positional parameters from the arguments after the file name. This form still updates the current shell's `PATH`, which is the reason it is the recommended form.
- `curl -sSL .../install.sh | sh -s -- --yes` also works. It is the fallback for agents whose shell is not bash or zsh.

The onboarding guide uses the `source` form, so `dtwiz` is on `PATH` for the agent's next command.

*Alternative considered:* `DTWIZ_YES=true` read by the script. Rejected; see Non-Goals.

### PowerShell: `[Alias('y')][switch]$Yes` plus `PositionalBinding=$false`

Add `[Alias('y')] [switch]$Yes` to `param()` and skip the `Read-Host` block when `$Yes` is set. `-y` would already resolve by prefix, since no common parameter starts with `y`. The alias makes the short form explicit and keeps it stable if parameters are added later.

Set `[CmdletBinding(PositionalBinding = $false)]`. Without this, the first declared parameter, `-InstallDir`, binds positionally, and an agent typing the bash-style `--yes` could end up installing into a directory literally named `--yes` while still being prompted. Turning positional binding off makes PowerShell reject any unnamed argument before the script body runs. The cost is that `.\install.ps1 C:\tools` stops working; the header and README only ever document `-InstallDir <dir>`.

Because binding fails before any script code runs, a rejected argument sends no self-monitoring events. Unknown named parameters like `-Foo` already behave this way today, so `install-script-selfmonitoring`, which lists `unknown_argument` as bash only, stays accurate.

### Unattended Windows invocation: script block, not `iex`

`irm | iex` cannot pass `-Yes`. The documented unattended form is:

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.ps1))) -Yes
```

A script block honors `[CmdletBinding()]` and `param()`, so `-Yes`, `-InstallDir`, `-Tag` and `-Channel` all bind normally. It also runs in a child scope, so `$ErrorActionPreference = "Stop"` no longer leaks into the caller's session as it does with `iex`. The user PATH update writes to the registry and `$env:PATH`, so it works the same way.

*Alternative considered:* save to a temp file and run `-File`. This needs execution-policy handling and cleanup, and it is more to paste into an agent prompt.

### Documentation placement

- Script header comments: add the flag next to `--install-dir` / `-InstallDir`, with the unattended invocation.
- `README.md` Installation section: one short "Unattended install (CI, agents)" block per OS. Keep the Quickstart human-oriented and unchanged.
- `skills/dtwiz/ONBOARDING.md` Step 1 and `skills/dtwiz/SKILL.md`: switch the install commands to the auto-confirm forms. The agent may run the install without asking because it is the step the user already asked for when they pasted the onboarding prompt. That matches how the guide already uses `--yes` for dtwiz commands the user chose.
- `.github/workflows/preview.yml` PR comments stay as they are; they are for humans testing snapshots.

## Risks / Trade-offs

- [Positional `-InstallDir` breaks for anyone relying on it] → It was never documented, and the parameter error names the problem. Call it out in the CHANGELOG under `Changed`.
- [`source <(...) --yes` behaves differently in an exotic shell] → The script already requires bash or zsh for the `source` form, and both pass arguments. The `| sh -s -- --yes` form is documented as the fallback.
- [The script block form behaves differently from `iex` in Windows PowerShell 5.1] → Verify both 5.1 and 7.x in the manual matrix (tasks.md). `[scriptblock]::Create` and `param()` binding are available in 5.1.
- [`exit 0` on the cancel path inside a script block] → It is unreachable with `-Yes`. Without `-Yes` it behaves as it does under `iex` today.
- [No automated regression coverage for the scripts] → The manual matrix in tasks.md covers every spec scenario. A script test harness is noted as a possible follow-up.

## Migration Plan

Additive. The new flags take effect for new runs as soon as the commit reaches `main`, because the scripts are fetched from `main` via raw URLs. Rollback: revert the script and doc changes in one commit (see proposal.md, Impact).
