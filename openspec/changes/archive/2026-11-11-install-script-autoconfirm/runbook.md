# Runbook: manually verify the install-script `--yes` change

Covers `scripts/install.sh`, `scripts/install.ps1` and the docs. The scripts download a real release from GitHub, so network access is required. Every test installs into a temporary directory, never the real one.

The curl URLs in the docs fetch from `main`, which does not have this change until it is merged. The tests below run the local files instead.

## Why some tests are wrapped in `bash -c '...'`

`source <script>` does not start a new program. It runs the script's lines inside the current terminal session. `install.sh` contains `exit` commands for its cancel and failure paths. If you source it directly and one of those runs, `exit` ends your terminal session (the window closes or you are logged out).

`bash -c '...'` starts a separate throwaway bash process, so `exit` only ends that process. Only the `source` test (B3) needs the wrapper. The other bash tests run the script as its own process and are already safe.

## 1. Bash

Run from the repo root, in order, in the same terminal. The `export` lines matter: the `bash -c` child process only sees exported variables.

```bash
cd ~/wrk/dtwiz

# Setup: sandbox install dir and fake HOME (keeps PATH edits out of your real ~/.zshrc)
export T=$(mktemp -d)
export H=$(mktemp -d)
```

### Commands

```bash
# [WORKS] B1: long form (expect: preview, no "Continue?", installs, exit 0)
HOME=$H bash scripts/install.sh --yes --install-dir $T
ls $T; rm -f $T/dtwiz

# [WORKS] B2: short form, then reversed order (expect: same as B1 both times)
HOME=$H bash scripts/install.sh -y --install-dir $T
rm -f $T/dtwiz
HOME=$H bash scripts/install.sh --install-dir $T -y
rm -f $T/dtwiz

# [WORKS] B3: sourced form, wrapped so exit cannot close your terminal
# (expect: installs, final line shows PATH containing $T)
# Run it in zsh (the macOS default shell). The bash variant needs bash 4 or newer: macOS's
# /bin/bash is 3.2, whose "source <(...)" only reads what is already in the pipe and silently
# runs nothing when the producer is slower than that. Check with: bash -c 'echo $BASH_VERSION'
HOME=$H zsh -c 'source <(cat scripts/install.sh) --yes --install-dir "$T"; echo "PATH contains T: $PATH"'
rm -f $T/dtwiz
# only if "bash" is 4 or newer (e.g. Homebrew bash):
HOME=$H bash -c 'source <(cat scripts/install.sh) --yes --install-dir "$T"; echo "PATH contains T: $PATH"'
rm -f $T/dtwiz

# [WORKS] B4: piped to sh (expect: installs with no prompt)
cat scripts/install.sh | HOME=$H sh -s -- --yes --install-dir $T
rm -f $T/dtwiz

# [WORKS] B5: no controlling terminal (expect: installs, no /dev/tty error in the output)
# setsid() fails with "Operation not permitted" when the caller is already a process-group
# leader, which is what an interactive shell makes a foreground command. So fork first and
# let the child call setsid().
cat > $T/notty.py <<'EOF'
import os, sys
if os.fork():
    _, st = os.wait()
    sys.exit(os.WEXITSTATUS(st))
os.setsid()
os.execvp(sys.argv[1], sys.argv[1:])
EOF
# sanity check (expect: "NO_TTY"):
python3 $T/notty.py bash -c ': </dev/tty 2>/dev/null && echo HAS_TTY || echo NO_TTY'
HOME=$H python3 $T/notty.py bash scripts/install.sh --yes --install-dir $T </dev/null
rm -f $T/dtwiz $T/notty.py

# [WORKS] B6: default still prompts (press Enter when asked; expect: installs)
HOME=$H bash scripts/install.sh --install-dir $T
rm -f $T/dtwiz

# [WORKS] B7: decline cancels (type n; expect: "Installation cancelled.", exit 0, empty dir)
HOME=$H bash scripts/install.sh --install-dir $T
ls $T

# [NOT RELEVANT] B8: env var must NOT enable auto-confirm (expect: the prompt appears; type n)
DTWIZ_YES=true HOME=$H bash scripts/install.sh --install-dir $T

# [WORKS] B9: unknown flag (expect: "Unknown argument: --yess", exit=1, nothing downloaded)
bash scripts/install.sh --yess; echo "exit=$?"

# B10: sudo not bypassed. The directory must already exist and be root-owned: the script
# runs "mkdir -p" without sudo first, so a missing dir under /opt fails with "could not create"
# before it ever reaches sudo (that failure is unrelated to --yes).
# (expect: "requires elevated privileges..." then the sudo password prompt;
# Ctrl+C is fine, the prompt appearing is the pass condition)
sudo mkdir /opt/dtwiz-runbook
HOME=$H bash scripts/install.sh --yes --install-dir /opt/dtwiz-runbook
sudo rm -rf /opt/dtwiz-runbook

# [WORKS] B11: syntax check (expect: no output)
bash -n scripts/install.sh && sh -n scripts/install.sh

# Cleanup
rm -rf $T $H
```

| # | Spec scenario | Pass if |
|---|---|---|
| B1 | Long form | Preview prints, no `Continue?` line, `installed to $T/dtwiz`, exit 0, `$T/dtwiz` exists |
| B2 | Short form plus another option | Same as B1 in both argument orders |
| B3 | Sourced with args | In zsh (and bash 4+ if available): installs with no prompt; final `PATH` line contains `$T`. With macOS `/bin/bash` 3.2 no installer output appears at all; that is a bash 3.2 limitation, not a failure of this change |
| B4 | Piped to `sh` | Installs with no prompt |
| B5 | No terminal | Installs; no `/dev/tty: Device not configured` error |
| B6 | Default prompts | `Continue? [Y/n]` appears and waits; Enter installs |
| B7 | Decline cancels | `Installation cancelled.`, exit 0, nothing installed |
| B8 | Env var does not enable it | The prompt still appears |
| B9 | Unknown flag | `Unknown argument: --yess`, exit 1 |
| B10 | sudo not bypassed | The sudo password prompt still appears |
| B11 | Syntax | No output |

Optional, if `shellcheck` is installed: `shellcheck scripts/install.sh`. No CI lints these scripts, so a clean run is not required.

### B12. Self-monitoring (optional)

Set `DT_ENVIRONMENT` and `DT_PLATFORM_TOKEN`, then run B1. In Dynatrace, `fetch events | filter title == "dtwiz install script"` should show an `invoked` event followed by a `completed` event with the same `executionId`, and no `cancelled` event.

## 2. PowerShell

Run on Windows, in order, in the same window. The Windows machine does not need the repo: the setup step downloads `install.ps1` from the `feature/install-script-autoconfirm` branch on GitHub into a scratch folder. Do the whole sequence once in Windows PowerShell 5.1 (`powershell.exe`) and once in PowerShell 7 (`pwsh`). Where a test starts a child shell, the block shows both versions.

**Before you start:** the branch must contain the script changes. Check that the branch on GitHub has the `-Yes` parameter (the setup step below also checks this and stops if it is missing). The docs checks in section 3 need the repo and are run on the machine that has the code.

The installer adds the install directory to the real user PATH (registry). The `Clear-RbPath` helper below removes it again; run it after any test that installs. A throwaway VM also works.

### Commands

```powershell
# Setup: allow local scripts to run in THIS window only
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12   # needed on Windows PowerShell 5.1

# Setup: download install.ps1 from the branch into a scratch folder, and use it as the "repo root".
# The tests below call .\scripts\install.ps1, so keep the scripts\ subfolder.
$Branch = "feature/install-script-autoconfirm"
$Url    = "https://raw.githubusercontent.com/dynatrace-oss/dtwiz/$Branch/scripts/install.ps1"
$W      = (New-Item -ItemType Directory (Join-Path $env:TEMP "dtwiz-rb-src-$(Get-Random)")).FullName
New-Item -ItemType Directory (Join-Path $W "scripts") | Out-Null
# Do NOT use "Invoke-WebRequest -OutFile": it saves UTF-8 without a BOM, and Windows PowerShell 5.1
# then reads the script's box-drawing characters as ANSI and fails with "string is missing the
# terminator". Invoke-RestMethod decodes the text as UTF-8, and "Set-Content -Encoding UTF8" writes
# a BOM on 5.1, so both 5.1 and 7 parse the saved file correctly.
Invoke-RestMethod -Uri $Url | Set-Content -Encoding UTF8 -Path (Join-Path $W "scripts\install.ps1")
Set-Location $W

# Stop here if the branch does not have the change yet (expect: prints a match)
Select-String -Path .\scripts\install.ps1 -Pattern 'switch\]\$Yes' -Quiet    # expect True

# Setup: sandbox install dir
$T = (New-Item -ItemType Directory (Join-Path $env:TEMP "dtwiz-rb-$(Get-Random)")).FullName

# Helper: removes $T from your user PATH again
function Clear-RbPath {
    $p = [Environment]::GetEnvironmentVariable("PATH","User") -split ";" | Where-Object { $_ -and $_ -ne $T }
    [Environment]::SetEnvironmentVariable("PATH", ($p -join ";"), "User")
}

# P1: switch from a file (expect: preview, no "Continue?", installs)
.\scripts\install.ps1 -Yes -InstallDir $T
Test-Path $T\dtwiz.exe            # expect True
Remove-Item $T\dtwiz.exe*; Clear-RbPath

# P2: short alias (expect: same as P1)
.\scripts\install.ps1 -y -InstallDir $T
Remove-Item $T\dtwiz.exe*; Clear-RbPath

# P3: script-block form, exactly the documented unattended form (with the branch URL)
# (expect: installs with no prompt; the last line prints "Continue", meaning
# $ErrorActionPreference did not leak into your session)
& ([scriptblock]::Create((Invoke-RestMethod $Url))) -Yes -InstallDir $T
$ErrorActionPreference
Remove-Item $T\dtwiz.exe*; Clear-RbPath

# P4: non-interactive (expect: installs in both shells, no "prompt_failed")
pwsh       -NonInteractive -ExecutionPolicy Bypass -File .\scripts\install.ps1 -Yes -InstallDir $T
Remove-Item $T\dtwiz.exe*; Clear-RbPath
powershell -NonInteractive -ExecutionPolicy Bypass -File .\scripts\install.ps1 -Yes -InstallDir $T
Remove-Item $T\dtwiz.exe*; Clear-RbPath

# P5: default still prompts (press Enter when asked; expect: installs)
.\scripts\install.ps1 -InstallDir $T
Remove-Item $T\dtwiz.exe*; Clear-RbPath

# P6: decline cancels (type n; expect: "Installation cancelled.", empty dir)
.\scripts\install.ps1 -InstallDir $T
Get-ChildItem $T                  # expect nothing listed

# P7: non-interactive WITHOUT -Yes (expect: fails with a prompt error in both shells;
# this is unchanged behavior and shows -Yes is what fixes P4)
pwsh       -NonInteractive -ExecutionPolicy Bypass -File .\scripts\install.ps1 -InstallDir $T
powershell -NonInteractive -ExecutionPolicy Bypass -File .\scripts\install.ps1 -InstallDir $T

# P8: rejects bash-style flag (expect: parameter error before any output)
.\scripts\install.ps1 --yes
Test-Path .\--yes                 # expect False

# P9: rejects positional install dir (expect: parameter error before any output)
$X = Join-Path $env:TEMP "dtwiz-positional-$(Get-Random)"
.\scripts\install.ps1 $X
Test-Path $X                      # expect False

# P10: env var must NOT enable auto-confirm (expect: the prompt appears; type n)
$env:DTWIZ_YES = "true"
.\scripts\install.ps1 -InstallDir $T
Remove-Item Env:DTWIZ_YES

# Cleanup
Clear-RbPath
Set-Location $env:TEMP
Remove-Item -Recurse -Force $T, $W
```

| # | Spec scenario | Pass if |
|---|---|---|
| P1 | Switch from a file | Preview prints, no `Continue?` line, `Test-Path` prints `True` |
| P2 | Short alias | Same as P1 |
| P3 | Script-block form | Installs with no prompt; `$ErrorActionPreference` prints `Continue` |
| P4 | Non-interactive | Installs in both shells; no `prompt_failed` |
| P5 | Default prompts | `Continue? [Y/n]` appears and waits; Enter installs |
| P6 | Decline cancels | `Installation cancelled.`, nothing in `$T` |
| P7 | Non-interactive without `-Yes` | Fails with a prompt error in both shells |
| P8 | Rejects bash-style flag | Parameter error before any output; `Test-Path` prints `False` |
| P9 | Rejects positional dir | Parameter error before any output; `Test-Path` prints `False` |
| P10 | Env var does not enable it | The prompt still appears |

P8 and P9 are the checks for task 2.1. P1 and P4 are the checks for task 2.2.

The exact error text for P8 and P9 differs between 5.1 and 7 (for example `A positional parameter cannot be found that accepts argument '--yes'`). What matters is that it is a parameter error and that none of the installer's own output (`Detected platform...`) appears before it. If P7 installs instead of failing, note the shell and output: the prompt is not failing in that environment, which is separate from this change.

## 3. Docs

From the repo root:

```bash
rg -n "install\.(sh|ps1)" skills README.md
```

- Every command under `skills/` ends in `--yes` or `-Yes`.
- In `README.md`, only the "Unattended install" block uses `--yes` / `-Yes`; the Quickstart and the channel table are unchanged.
- Windows commands use `& ([scriptblock]::Create((irm ...))) -Yes`, not `| iex`.
- The sentence in `skills/dtwiz/ONBOARDING.md` Step 1 agrees with the existing rule near line 217 ("`--yes` only skips the confirmation").
- The two `CHANGELOG.md` entries are under `[Unreleased]`, one in `Added` and one in `Changed`.

## 4. After merge

The curl forms only work once the change is on `main`:

```bash
zsh -c 'source <(curl -sSL https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.sh) --yes --install-dir "$(mktemp -d)"'
```

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.ps1))) -Yes -InstallDir $T
```

When P8 and P9 pass, tick tasks 2.1 and 2.2 in `openspec/changes/install-script-autoconfirm/tasks.md`.
