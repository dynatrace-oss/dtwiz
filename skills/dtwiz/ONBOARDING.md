# Get your data into Dynatrace with dtwiz and an AI agent

## Use this prompt

Paste this into Claude Code, Cursor, or any coding agent. Replace `<env-id>` with your
environment ID.

```text
Connect my systems to my Dynatrace environment https://<env-id>.apps.dynatrace.com with dtwiz (https://github.com/dynatrace-oss/dtwiz): this machine and its apps, plus any Kubernetes, AWS, Azure or GCP environments it can reach. I choose what gets connected. First read and follow https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/skills/dtwiz/ONBOARDING.md
```

The agent installs dtwiz, opens the token page and a terminal for you, shows what dtwiz detected
(this machine and its apps, Kubernetes clusters, AWS, Azure and GCP accounts you are signed in
to), and sets up only what you choose. Your platform token never goes into the chat: you only
copy it, and the terminal reads it from the clipboard and then clears the clipboard.

---

## Instructions for the agent

You are helping the user get their data into Dynatrace with dtwiz: this machine and its
applications, and the Kubernetes clusters and AWS, Azure or GCP accounts it can reach. The user's environment URL is the one in
their message (`https://<env-id>.apps.dynatrace.com` with their real ID). Never use the
placeholder from this file as the URL. If the message has no real URL, ask for it before
anything else.

Work through the steps in order. Guide the user one small action at a time, in short friendly
messages, and wait for them to confirm before moving on. Keep every message short and say only
what the user needs for the next action. Don't repeat earlier status or mention options they did
not ask about.

The user chooses what to connect. Never install, update or change anything the user did not
select.

Every time you wait for the user (token copied, a choice, a go-ahead), make the answer a click
or a single word, never free text:

- If your tool has an interactive question or reply buttons (for example the `AskUserQuestion`
  tool, or the app's reply buttons), use it, with short options such as "Copied" and "I need
  help".
- If it does not, tell the user the exact word to reply with, such as "copied".

Never leave the user facing an open prompt after asking them to copy the token: that is when
people paste it into the chat.

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
   here in the chat." and ask for confirmation as a button or single word, for example an
   interactive question "Token copied?" with the options "Copied" and "I need help". Wait.

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
- Let the user choose a recommendation. Offer ALL of them, in priority order (lowest `priority`
  first, same order as the JSON), each with its `title`, and mark the first one as your
  suggestion without reordering or dropping any. Wait for the answer.
  - If your tool has an interactive picker (a multiple-choice question) that can show every
    entry, use it, with the entry number in each label.
  - If the picker cannot show every entry (many tools allow only about four), show the complete
    numbered list as plain text and ask for the number instead.
- Map the chosen recommendation's `method` to a command, and run it with `--yes`:
  - `otel`: ask the user once, "Instrument one of your applications, or host monitoring only?"
    - Host only: run `dtwiz install otel-collector --yes`.
    - Application: dtwiz only lists the detected projects in an interactive picker, which you
      cannot answer. So read the list from it and let the user choose in the chat:
      1. In the terminal tab, run `dtwiz install otel` and read the project list it prints.
         dtwiz asks before it does anything. The list depends on the folder it runs from, and
         it only scans to a limited depth, so a project can be missing. If the user's project
         is not listed, offer an entry such as "A project in another folder" and ask for the
         folder path.
      2. Cancel the command (Ctrl-C, for example with the terminal tool's stop action) without
         choosing anything.
      3. Let the user choose, in plain words. Never show dtwiz's menu codes (the numbers and the
         letters g, n, p, c, r, u, e, l, o, s) or ask the user which dtwiz menu item to press.
         Offer exactly these entries, in dtwiz's order:
         - One entry per detected project, labelled with its language, name and folder, for
           example "Python: schnitzel (~/.dtwiz/examples/schnitzel)".
         - "Host monitoring only (no application)".
         - "A project in another folder", only if the user's project may be missing.
         - "My application uses another language (Go, .NET, PHP, ...)" as one entry, not one per
           language.

         Use an interactive picker if it can show every entry. Otherwise show the complete
         numbered list as plain text and ask for the number. The user never needs to touch the
         terminal.
      4. For a project, run `dtwiz install otel --project <folder of the chosen project> --yes`.
      5. For "host monitoring only", or "another language", run `dtwiz install otel-collector
         --yes`. For "another language", also tell them dtwiz cannot instrument that language
         automatically yet, so only the host is monitored.
      If you cannot read or stop the terminal, ask the user for the project path instead.
  - `kubernetes`, `aws`, `azure`, `gcp`: these change a real cluster or cloud account. First
    say in one line what will change (for example "install the Dynatrace Operator into cluster
    X"), using the detection details from `recommend --json`, and wait for the user's yes. Then
    run `dtwiz install <method> --yes`.
  - `oneagent`, `docker`: `dtwiz install <method> --yes`
  - `azure-update`, `gcp-update`: these change a cloud account too. Say in one line what will
    change, wait for the user's yes, then run `dtwiz update azure --yes` or
    `dtwiz update gcp --yes`.
  - `otel-update`: `dtwiz update otel --yes --experimental`. If the user did not set up a
    collector themselves, this may be a false positive (dtwiz can mistake an open `dtwiz install`
    terminal for a running collector). Say so, and suggest the `otel` option instead.
  - `already-installed`, `not-supported`: nothing to install; say so and offer the next option.
- `--yes` only skips the confirmation. If a command still waits for input, you cannot type into
  it: stop it and give the user the exact command to run themselves in the terminal.
- dtwiz then checks that data arrives. For a Collector install, success is dtwiz printing
  "View in Dynatrace Logs"; the watch that follows then lists hosts, services and logs as they
  appear. Tell the user when data shows up. This can take up to about 4 minutes, so keep waiting
  that long and tell the user it is normal. Only if nothing shows up after about 4 minutes, say
  so, stop the watch, and point the user to the collector log that dtwiz printed.

When finished, remind the user to close the terminal or run `unset DT_PLATFORM_TOKEN`, to
delete the token in Dynatrace if they no longer need it, and to delete the token from their
clipboard manager's history if they use one. The token is gone once the terminal
closes, so a later session needs a new one.
