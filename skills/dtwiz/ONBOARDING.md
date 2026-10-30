# Onboard data with dtwiz using an AI agent

## Use this prompt

Paste this into Claude Code, Cursor, or any coding agent. Replace `<env-id>` with your
environment ID.

```text
Onboard this machine to my Dynatrace environment https://<env-id>.apps.dynatrace.com using dtwiz (https://github.com/dynatrace-oss/dtwiz). First read and follow https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/skills/dtwiz/ONBOARDING.md. Guide me step by step, and never ask for or accept my platform token in the chat.
```

The agent installs dtwiz, opens the token page and a terminal for you, and runs the onboarding.
Your platform token never goes into the chat: you only copy it, and the terminal reads it from
the clipboard and then clears the clipboard.

---

## Instructions for the agent

You are onboarding the user to Dynatrace with dtwiz. The user's environment URL is the one in
their message (`https://<env-id>.apps.dynatrace.com` with their real ID). Never use the
placeholder from this file as the URL. If the message has no real URL, ask for it before
anything else.

Work through the steps in order. Guide the user one small action at a time, in short friendly
messages, and wait for them to confirm before moving on.

### Security rules (always apply)

- Never ask the user for their Dynatrace platform token in the chat, and never accept it there.
  If they paste something that starts with `dt0s16.` or `dt0c01.`, stop, tell them to revoke
  that token in Dynatrace and create a new one.
- Never print, echo, log or write the token anywhere. Do not run `env`, `printenv`, `set`, or
  `echo $DT_PLATFORM_TOKEN`. Check only whether it is set, with:
  `[ -n "$DT_PLATFORM_TOKEN" ] && echo set || echo missing`
- Never pass the token as a command-line argument (no `--platform-token`). dtwiz reads it from
  the `DT_PLATFORM_TOKEN` environment variable.
- Never run any `dtwiz uninstall` command during onboarding. `uninstall otel`, for example, also
  deletes tenant-wide configuration (the OTel extension and the OpenPipeline routes) that other
  people in the same environment may rely on. Only uninstall if the user explicitly asks, and
  show them the preview first.

### Step 1: Install dtwiz

- Run `command -v dtwiz`. If it is missing, install it:
  - macOS/Linux: `source <(curl -sSL https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.sh)`
  - Windows (PowerShell): `irm https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.ps1 | iex`
- Confirm with `dtwiz version`.

### Step 2: Credentials (guided, one action at a time)

Check whether `DT_ENVIRONMENT` and `DT_PLATFORM_TOKEN` are set in the shell you will use for
dtwiz (see the check above). If both are, go to step 3.

Otherwise guide the user through these moves. After each "Wait", stop and send nothing else
until the user replies.

1. Open the token page in the user's default browser (not in a built-in browser you can read):
   macOS `open`, Linux `xdg-open`, Windows `start`.

   ```text
   https://<env-id>.apps.dynatrace.com/ui/apps/dynatrace.quickstart/add-data/automatic
   ```

   Then say: "I opened the Dynatrace page. Copy the platform token shown there. Don't paste it
   here in the chat. Tell me when it's copied." Wait.

2. When the user confirms, open a visible terminal tab for them (with a terminal tool, or ask
   them to open one if you cannot). Tell them: "Don't type in that terminal, I'll run
   everything there." Then send exactly ONE line to it. It reads the token from the clipboard
   into `DT_PLATFORM_TOKEN`, clears the clipboard, and prints only whether it worked. Never run
   a clipboard command on its own or show its output.

   macOS:

   ```bash
   export DT_ENVIRONMENT=https://<env-id>.apps.dynatrace.com; export DT_PLATFORM_TOKEN="$(pbpaste)"; pbcopy </dev/null; case "$DT_PLATFORM_TOKEN" in dt0s16.*) echo "token set";; *) unset DT_PLATFORM_TOKEN; echo "clipboard has no platform token";; esac
   ```

   Linux (Wayland or X11):

   ```bash
   export DT_ENVIRONMENT=https://<env-id>.apps.dynatrace.com; export DT_PLATFORM_TOKEN="$(wl-paste 2>/dev/null || xclip -selection clipboard -o)"; (wl-copy --clear 2>/dev/null || xclip -selection clipboard </dev/null); case "$DT_PLATFORM_TOKEN" in dt0s16.*) echo "token set";; *) unset DT_PLATFORM_TOKEN; echo "clipboard has no platform token";; esac
   ```

   Windows (PowerShell):

   ```powershell
   $env:DT_ENVIRONMENT='https://<env-id>.apps.dynatrace.com'; $env:DT_PLATFORM_TOKEN=(Get-Clipboard -Raw).Trim(); Set-Clipboard -Value $null; if ($env:DT_PLATFORM_TOKEN -like 'dt0s16.*') { 'token set' } else { Remove-Item Env:DT_PLATFORM_TOKEN; 'clipboard has no platform token' }
   ```

   If it prints "clipboard has no platform token", ask the user to copy the token again and
   repeat this move.

3. Run `dtwiz status` in that same tab. If it shows a connection, say "Connected, the token
   works" and continue. If not, say what failed and offer to repeat move 1.

If you cannot open or drive a terminal, give the user the line from move 2 to run in their
own terminal and ask them to paste back only the `dtwiz status` output, which contains no
token.

Run all later dtwiz commands in that same terminal tab and read their output from it.

### Step 3: Analyze and recommend

Run `dtwiz analyze` and `dtwiz recommend --json`. Always use `--json` for `recommend`: the plain
output is an interactive menu (with a demo-app option) that you must not answer. Summarize what
was detected (OS, containers, Kubernetes, cloud, services) and list the recommendations by
`priority` (lowest number first), using their `title`.

### Step 4: Install

- Do NOT run a dry-run and do not offer one.
- Ask the user once which option to install (recommend the first one), and wait.
- Map the chosen recommendation's `method` to a command, and run it with `--yes`:
  - `otel`: ask the user once, "Instrument one of your applications, or host monitoring only?"
    - Host only: run `dtwiz install otel-collector --yes`.
    - Application: dtwiz only lists the detected projects in an interactive picker, which you
      cannot answer. So read the list from it and let the user choose in the chat:
      1. In the terminal tab, run `dtwiz install otel` and read the numbered project list it
         prints. dtwiz asks before it does anything.
      2. Cancel the command (Ctrl-C, for example with the terminal tool's stop action) without
         choosing anything.
      3. Show the numbered list in the chat and ask the user to pick one by number, or "host
         only". The user never needs to touch the terminal.
      4. Run `dtwiz install otel --project <path of the chosen project> --yes`.
      5. If they pick a language without automatic detection (Go, .NET, PHP and so on) or
         "skip", run `dtwiz install otel-collector --yes` and tell them dtwiz cannot
         instrument that language automatically yet.
      If you cannot read or stop the terminal, ask the user for the project path instead.
  - `kubernetes`, `oneagent`, `docker`, `aws`, `azure`, `gcp`: `dtwiz install <method> --yes`
  - `azure-update`, `gcp-update`: `dtwiz update azure --yes` or `dtwiz update gcp --yes`
  - `otel-update`: `dtwiz update otel --yes --experimental`
  - `already-installed`, `not-supported`: nothing to install; say so and offer the next option.
- `--yes` only skips the confirmation. If a command still waits for input, you cannot type into
  it: stop it and give the user the exact command to run themselves in the terminal.
- dtwiz then watches for incoming data. Tell the user when data shows up in Dynatrace. If nothing
  shows up after about two minutes, say so instead of waiting longer, stop the watch, and point
  the user to the collector log that dtwiz printed.

When finished, remind the user to close the terminal or run `unset DT_PLATFORM_TOKEN`, to
delete the token in Dynatrace if they no longer need it, and to delete the token from their
clipboard manager's history if they use one. The token is gone once the terminal
closes, so a later session needs a new one.
