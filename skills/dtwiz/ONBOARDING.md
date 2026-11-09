# Get your data into Dynatrace with dtwiz and an AI agent

## Use this prompt

Paste this into Claude Code, Cursor, or any coding agent. Replace `<env-id>` with your
environment ID.

```text
Connect my systems to my Dynatrace environment https://<env-id>.apps.dynatrace.com with dtwiz (https://github.com/dynatrace-oss/dtwiz): this machine and its apps, plus any Kubernetes, AWS, Azure or GCP environments it can reach. I choose what gets connected. First read and follow https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/skills/dtwiz/ONBOARDING.md
```

The agent installs dtwiz, opens a terminal for you, shows what dtwiz detected (this machine and
its apps, Kubernetes clusters, AWS, Azure and GCP accounts you are signed in to), and sets up only
what you choose. Your platform token never goes into the chat: you only copy it, and the terminal
reads it from the clipboard and then clears the clipboard. If you already copied a token (for
example from the QuickStart app), the agent uses it right away. Otherwise it opens the token page
and asks you to copy one.

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
- Never print, echo, log or write the token anywhere. The one exception is `dtctl config
  set-credentials` in step 2, which stores it in the OS keychain (step 5 removes it again). Do
  not run `env`, `printenv`, `set`, or
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
until the user replies. The user may already have copied a token (the QuickStart app can do that
when it opens the prompt), so try the clipboard first and only open the token page if it holds
no token.

1. Open a visible terminal tab for the user (with a terminal tool, or ask them to open one if
   you cannot). Tell them: "Don't type in that terminal, I'll run everything there." This tab is
   where every dtwiz command runs from now on.

2. Try the clipboard. Send exactly ONE line to the tab. It looks at the clipboard and, only if
   it holds a platform token (it starts with `dt0s16.`), loads it into `DT_PLATFORM_TOKEN` and
   clears the clipboard. Any other clipboard content is left untouched. It prints only whether
   it worked. Never run a clipboard command on its own or show its output.

   macOS:

   ```bash
   export DT_ENVIRONMENT=https://<env-id>.apps.dynatrace.com; _t="$(pbpaste)"; case "$_t" in dt0s16.*) export DT_PLATFORM_TOKEN="$_t"; pbcopy </dev/null; echo "token set";; *) echo "clipboard has no platform token";; esac; unset _t
   ```

   Linux (Wayland or X11):

   ```bash
   export DT_ENVIRONMENT=https://<env-id>.apps.dynatrace.com; _t="$(wl-paste 2>/dev/null || xclip -selection clipboard -o 2>/dev/null)"; case "$_t" in dt0s16.*) export DT_PLATFORM_TOKEN="$_t"; (wl-copy --clear 2>/dev/null || xclip -selection clipboard </dev/null); echo "token set";; *) echo "clipboard has no platform token";; esac; unset _t
   ```

   Windows (PowerShell):

   ```powershell
   $env:DT_ENVIRONMENT='https://<env-id>.apps.dynatrace.com'; $t="$(Get-Clipboard -Raw)".Trim(); if ($t -like 'dt0s16.*') { $env:DT_PLATFORM_TOKEN=$t; Set-Clipboard -Value $null; 'token set' } else { 'clipboard has no platform token' }; Remove-Variable t
   ```

   - If it prints "token set", say "I found the platform token you copied and loaded it into
     the terminal." and go to move 4. Do not open the token page.
   - If it prints "clipboard has no platform token", go to move 3.

3. Get a token from the user. Open the token page in their default browser (not in a built-in
   browser you can read): macOS `open`, Linux `xdg-open`, Windows `start`.

   ```text
   https://<env-id>.apps.dynatrace.com/ui/apps/dynatrace.quickstart/add-data/automatic
   ```

   Then say: "I opened the Dynatrace page. Copy the platform token shown there. Don't paste it
   here in the chat." and ask for confirmation as a button or single word, for example an
   interactive question "Token copied?" with the options "Copied" and "I need help". Wait.
   When the user confirms, send the same line from move 2 to the same tab again. If it still
   prints "clipboard has no platform token", ask the user to copy the token again and repeat.

4. In that same tab, run `dtwiz status && dtwiz recommend --json` as one command (it saves a
   round trip, and step 3 needs both outputs). `status` shows the connection and the system
   analysis. If it shows a connection, say "Connected, the token works" and continue. If it does
   not (for example the token is expired, or belongs to another environment), run
   `unset DT_PLATFORM_TOKEN` (PowerShell: `Remove-Item Env:DT_PLATFORM_TOKEN`) in the tab, tell
   the user in one line what failed, and go to move 3 for a new token.

5. Once the connection works, and only if `dtctl version` works, store the token for dtctl
   now, while this tab is still free (the install in step 4 blocks it with a watch). Use the
   command from step 5, item 2. Step 5 then runs from a new tab that needs no token, because
   dtctl reads it from the OS keychain.

If you cannot open or drive a terminal, give the user the line from move 2 to run in their
own terminal and ask them to paste back only the `dtwiz status` output, which contains no
token.

Run all later dtwiz commands in that same terminal tab and read their output from it.

### Step 3: Analyze and recommend

Use the output of `dtwiz status` and `dtwiz recommend --json` from step 2 (run `dtwiz analyze`
only if you need more detail, for example when you got here with the token already set). Always
use `--json` for `recommend`: the plain output is an interactive menu (with a demo-app option)
that you must not answer. If a terminal read returns "no new output", read again with a longer
wait before you assume the command is stuck. Summarize what was detected (OS, containers,
Kubernetes, cloud, services) and list the recommendations by `priority` (lowest number first),
using their `title`.

### Step 4: Install

- Do NOT run a dry-run and do not offer one.
- Let the user choose a recommendation. Offer ALL of them, in priority order (lowest `priority`
  first, same order as the JSON), each with its `title`, and mark the first one as your
  suggestion without reordering or dropping any. Offer exactly what `recommend --json`
  returned: do not add, rename or invent entries (such as "skip for now"). Wait for the answer.
  - If your tool has an interactive picker (a multiple-choice question) that can show every
    entry, use it, with the entry number in each label.
  - If the picker cannot show every entry (many tools allow only about four), show the complete
    numbered list as plain text and ask for the number instead.
- Map the chosen recommendation's `method` to a command, and run it with `--yes` (the one
  exception is the project-listing run for `otel` below):
  - `otel`: ask the user once, "Instrument one of your applications, or host monitoring only?"
    Ask before you run any `dtwiz install otel` command. Never run `dtwiz install otel --yes`
    without `--project`: `--yes` does not skip the project picker, and answering the picker
    yourself would choose an application for the user.
    - Host only: run `dtwiz install otel-collector --yes`.
    - Application: dtwiz only lists the detected projects in an interactive picker, which you
      cannot answer. So read the list from it and let the user choose in the chat:
      1. In the terminal tab, run `dtwiz install otel` and read the project list it prints.
         dtwiz asks before it does anything. The list depends on the folder it runs from, and
         it only scans to a limited depth, so a project can be missing. If the user's project
         is not listed, offer an entry such as "A project in another folder" and ask for the
         folder path.
      2. Cancel the command (Ctrl-C, for example with the terminal tool's stop action) without
         choosing anything. This is the only place where Ctrl-C is fine, because nothing is
         installed yet.
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
- If `install otel --project` ends with "service(s) failed to start" (for example "Port 8081 is
  in use" because the app already runs from an earlier attempt), the Collector may still have
  been installed and there is no watch to close. Do not kill the user's processes. Say which
  ports are taken and by what (`lsof -i :<port>` or the dtwiz log path it prints), offer to
  continue with what works, and let step 5 show what actually arrived.
- `--yes` only skips the confirmation. Never answer a dtwiz menu or prompt on the user's behalf,
  even if your tool can type into a running terminal: letters such as `g`, `n`, `p` or `s` are
  menu codes, not choices to guess. If a command still waits for input, cancel it before
  anything is installed and give the user the exact command to run themselves, or ask them
  what they want.
- dtwiz then checks that data arrives. For a Collector install, success is dtwiz printing
  "View in Dynatrace Logs"; the watch that follows then lists hosts, services and logs as they
  appear. Tell the user when data shows up.
- Waiting takes time, and it is normal:
  - Collector installs: up to about 4 minutes.
  - Kubernetes: up to about 10 minutes. The Operator, ActiveGate and the other pods take around
    8 minutes to become ready, and some pods stay `Pending` for a while when the cluster is short
    on memory, until the cluster autoscaler adds a node. Pending pods are not a failure.
  - Other methods: until dtwiz reports data, or about 10 minutes.

  Before you start waiting, tell the user in one line how long it can take, for example "This
  takes up to about 8 minutes, I'll keep checking." Then keep checking the terminal output
  yourself, with long waits, inside the same turn. Do not end your turn and ask the user to come
  back or to click "check again". Report only when dtwiz shows data, the command fails, or the
  limit is reached. At the limit, say so, give the current state (for Kubernetes
  `kubectl get pods -n dynatrace`), and point to the log or output that dtwiz printed.
- The install command does not exit by itself: after the data check, dtwiz keeps its watch open
  ("Press Enter to continue or keep watching...") until someone presses Enter. So do not wait
  for the command to finish, and do not treat the watch screen as a failure or a hang.
  - Never end the watch with Ctrl-C, and never stop or close its terminal tab. The Collector
    was started from that terminal and shares its process group, so this also kills the
    Collector and the data stops.
  - If your tool can type into a running terminal, send Enter once the data check is done.
  - If it cannot (for example the Claude desktop terminal tool), stop only the dtwiz process
    from another terminal tab, which needs no token. Look up its PID and send it SIGTERM:
    `kill -TERM $(pgrep -f 'dtwiz install')` (several matches mean several installs; pick the one
    you started). The Collector is a separate process and keeps running, which was verified
    (its health check on port 13133 still answers). This is not Ctrl-C: Ctrl-C signals the whole
    process group and kills the Collector too. On Windows use `Stop-Process -Id <pid>` for the
    dtwiz process only (untested). If you cannot find the PID, leave the watch running and tell
    the user they can press Enter in the install tab when they are done.

### Step 5: Show what arrived (dtctl)

Keep the user in the chat: show what is now in Dynatrace with `dtctl`
(<https://github.com/Dynatrace/dtctl>), using the token from step 2. Open a new terminal tab
for it: the install tab may still be held by dtwiz's watch, and dtctl
reads its credentials from the config file created in step 2, not from the environment. Tell
the user in one line that this takes a few seconds, and never print the token.

1. Check `dtctl version`. If dtctl is missing, say so in one line, skip to step 6, and do not
   install it.
2. Use a scratch config file with a read-only context and a credential name of its own, so
   the user's own dtctl contexts and credentials stay untouched. dtctl cannot resolve
   `${DT_PLATFORM_TOKEN}` as a credential, so the shell passes the real value to
   `set-credentials`, which stores it in the OS keychain (the config file holds no token). Step 2,
   move 5 runs the command below in the tab that has the token. If it has not run yet, run it now,
   only in a tab that has the token. Do not use `-v`, `-vv` or `--debug` (they print auth
   headers).
   - macOS/Linux:
     `C="${TMPDIR:-/tmp}/dtwiz-dtctl.yaml"; dtctl --config "$C" config set-context dtwiz --environment "$DT_ENVIRONMENT" --token-ref dtwiz-onboarding --safety-level readonly && dtctl --config "$C" config set-credentials dtwiz-onboarding --token "$DT_PLATFORM_TOKEN" && dtctl --config "$C" config use-context dtwiz`
   - Windows (PowerShell):
     `$C="$env:TEMP\dtwiz-dtctl.yaml"; dtctl --config $C config set-context dtwiz --environment $env:DT_ENVIRONMENT --token-ref dtwiz-onboarding --safety-level readonly; dtctl --config $C config set-credentials dtwiz-onboarding --token $env:DT_PLATFORM_TOKEN; dtctl --config $C config use-context dtwiz`
3. In the new tab, first set the path again (a new tab does not have `C`): macOS/Linux
   `C="${TMPDIR:-/tmp}/dtwiz-dtctl.yaml"`, PowerShell `$C="$env:TEMP\dtwiz-dtctl.yaml"`. Then run
   these read-only queries, one `dtctl --config "$C" query '<DQL>' -o csv` call each (each takes
   about a second). If the terminal tool rejects a long one-line batch, run each query on its
   own or put them in a small script file. Use the last 2 hours, and read the output yourself:
   - Logs: `fetch logs, from:now()-2h | summarize records=count(), by:{dt.system.bucket}`
   - Hosts: `smartscapeNodes OTEL_HOST | fields name` (for OneAgent hosts use `HOST`)
   - Host load: `timeseries cpu=avg(system.cpu.utilization), mem=avg(system.memory.utilization), from:now()-30m | fieldsAdd cpu_pct=arrayAvg(cpu)*100, mem_pct=arrayAvg(mem)*100 | fields cpu_pct, mem_pct`
   - Services: `smartscapeNodes SERVICE | fields name | limit 10`
   - Processes: `smartscapeNodes OTEL_PROCESS | summarize n=count()`
   - Problems: `fetch dt.davis.problems, from:now()-24h | summarize problems=count()`
   - Service names with ids: `smartscapeNodes SERVICE | fields id, name | limit 10`
   - Calls between services (map the ids to names with the services query):
     `smartscapeEdges "calls" | fields source_id, target_id | limit 20`
   - Log levels: `fetch logs, from:now()-2h | summarize n=count(), by:{loglevel} | sort n desc`
   - Top error: `fetch logs, from:now()-2h | filter loglevel=="ERROR" | summarize n=count(), by:{content} | sort n desc | limit 1`
   - Slowest service (duration in nanoseconds): `fetch spans, from:now()-2h | filter span.kind=="server" | summarize p95=percentile(duration,95), by:{service.name} | sort p95 desc | limit 1`
   - After a Kubernetes install also: `smartscapeNodes K8S_CLUSTER | fields name`
   - CPU trend, as a text chart you paste into a code block (works in every client):
     `dtctl --config "$C" query 'timeseries cpu=avg(system.cpu.utilization), from:now()-30m' -o chart --width 60 --height 6`
   An empty result is an answer ("nothing yet"), not an error. A parse error means the query
   does not fit this environment; skip it.
4. Show the result first, as a message of its own, before the cleanup and before any
   question (never inside the question of an interactive choice, where it is overlooked).
   Plain Markdown, so it renders in every client, at most about ten lines, in this order:
   - A headline with the time: "Your <machine> is in Dynatrace: first data after <m> min".
   - A table with what is connected: host (CPU, memory), processes, logs (with errors and
     warnings), services, open problems.
   - The service flow from the calls query as one line, for example
     `loadgenerator → frontend → order, delivery`. Skip it if there are no calls.
   - The CPU chart from the last query, in a code block.
   - One finding, the most useful of: the top error log, the slowest service (p95 in ms), or
     "no open problems" as a healthy baseline. One line, with real numbers.
   - What is not connected (for example Kubernetes, Azure, GCP, or no application traces when no
     instrumented app runs). The `dtctl` service that may appear comes from dtctl itself; say so.
   - Links: Dynatrace Services `https://<env-id>.apps.dynatrace.com/ui/apps/dynatrace.services/`
     and Logs `https://<env-id>.apps.dynatrace.com/ui/apps/dynatrace.logs/`, with the real
     environment ID.
   Do not go on to the cleanup or to step 6 before this message is written: the user should
   see it, and the next-step question must not repeat it.
5. Then clean up: `dtctl --config "$C" config delete-credentials dtwiz-onboarding`, then
   delete the scratch config (`rm -f "$C"`; PowerShell: `Remove-Item $C`). Older dtctl versions
   (for example 0.28) have no `delete-credentials`: then tell the user in one line that a
   keychain item named dtwiz-onboarding stays and can be removed in their keychain app. Also run
   this cleanup if the user stops before the overview.

### Step 6: Next step

Ask what to do next, as an interactive choice or a single word, with these options in this
order:

1. "Open Dynatrace QuickStart" (your recommendation). It is the same page dtwiz points to at the
   end of its watch ("See all your data and findings in Dynatrace QuickStart"). Open
   `https://<env-id>.apps.dynatrace.com/ui/apps/dynatrace.quickstart/` in the user's default
   browser (macOS `open`, Linux `xdg-open`, Windows `start`), with their real environment ID.
   If the install output shows that link as plain text with a `?tf=` time range (this happens
   on Windows), open that exact URL instead, because it is scoped to this run's data.
2. "Connect another system", only if dtwiz detected something the user has not connected yet.
   Name it (for example Kubernetes, Azure or GCP). If chosen, go back to the list in step 4.
3. "Done".

After the user is done, give the reminders below.

When finished, remind the user to run `unset DT_PLATFORM_TOKEN` in that terminal (and not to
close the tab while the Collector is meant to keep running, because it was started from it), to
delete the token in Dynatrace if they no longer need it, and to delete the token from their
clipboard manager's history if they use one. A later session needs a new token.
