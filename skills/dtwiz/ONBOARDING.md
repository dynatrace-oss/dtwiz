# Onboard data with dtwiz using an AI agent

## Use this prompt

Paste this into Claude Code, Cursor, or any coding agent. Replace `<env-id>` with your
environment ID.

```text
Onboard this machine to my Dynatrace environment https://<env-id>.apps.dynatrace.com using dtwiz (https://github.com/dynatrace-oss/dtwiz). First read and follow https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/skills/dtwiz/ONBOARDING.md. Guide me step by step, and never ask for or accept my platform token in the chat.
```

The agent installs dtwiz, opens the token page and a terminal for you, and runs the onboarding.
Your platform token never goes into the chat: you paste it into a terminal with hidden input.

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

   Then say: "I opened the Dynatrace page. Create the platform token there and copy it. Don't
   paste it here in the chat. Tell me when it's copied." Wait.

2. When the user confirms, open a visible terminal tab for them (with a terminal tool, or ask
   them to open one if you cannot). Send exactly ONE line to it, and nothing else.

   macOS/Linux:

   ```bash
   export DT_ENVIRONMENT=https://<env-id>.apps.dynatrace.com; printf 'Paste platform token (hidden): '; stty -echo; read DT_PLATFORM_TOKEN; stty echo; echo; export DT_PLATFORM_TOKEN
   ```

   Windows (PowerShell):

   ```powershell
   $env:DT_ENVIRONMENT='https://<env-id>.apps.dynatrace.com'; $t = Read-Host 'Paste platform token (hidden)' -AsSecureString; $env:DT_PLATFORM_TOKEN = [System.Net.NetworkCredential]::new('', $t).Password
   ```

   The terminal now waits for input. Do NOT send any further text, command or keypress to that
   terminal until the user says they are done: the waiting prompt would swallow it as the
   token. Then say: "I opened a terminal and it's waiting for the token. Click into it, paste
   the token and press Enter. Nothing will show while you paste, that's expected. Tell me when
   you're done." Wait.

3. When the user confirms, run `dtwiz status` in that tab (now it is safe to send input
   again). If it shows a connection, say "Connected, the token works" and continue. If not, say
   what failed and offer to repeat move 2.

If you cannot open or drive a terminal, give the user the command from move 2 to run in their
own terminal and ask them to paste back only the `dtwiz status` output, which contains no
token.

Run all later dtwiz commands in that same terminal tab and read their output from it.

### Step 3: Analyze and recommend

Run `dtwiz analyze` and `dtwiz recommend`. Summarize what was detected (OS, containers,
Kubernetes, cloud, services) and which ingestion method dtwiz ranks first.

### Step 4: Install

- Do NOT run a dry-run and do not offer one.
- Ask the user once which option to install (recommend the top-ranked one), and wait.
- When they choose, run `dtwiz install <method> --yes` right away.
- dtwiz then watches for incoming data. Tell the user when data shows up in Dynatrace, or what
  failed and the next fix to try.

When finished, remind the user to close the terminal or run `unset DT_PLATFORM_TOKEN`, and to
delete the token in Dynatrace if they no longer need it. The token is gone once the terminal
closes, so a later session needs a new one.
