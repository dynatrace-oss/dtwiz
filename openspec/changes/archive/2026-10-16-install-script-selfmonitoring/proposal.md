# Proposal: Install Script Self-Monitoring

## Why

The dtwiz binary reports self-monitoring events for every command, but the step before it, getting dtwiz onto the machine via `scripts/install.sh` or `scripts/install.ps1`, is invisible. We cannot tell how often installs are attempted, whether they succeed, or where they break (unsupported platform, version resolution, download, extraction, moving the binary into place).

## What Changes

- Both install scripts send a `CUSTOM_INFO` event to the user's Dynatrace environment at each lifecycle point: invoked, completed, failed (with a reason), and cancelled (user declined the prompt).
- Events carry only what is needed to judge install health: a per-run correlation id, the script (`sh` or `ps1`), the step, the operating system, the failure reason, the resolved dtwiz version (once known), and the requested release source (channel and tag).
- Events are sent only when `DT_ENVIRONMENT` is set; the platform token is used when present but not required, mirroring the binary's behavior. Sending never blocks the install for more than a few seconds and never surfaces errors to the user.
- Failure detection is completed so every failure is reported with the right reason:
  - `install.sh`: the release download, archive extraction, and binary move are currently unchecked; a failed `sudo mv` reports success today.
  - `install.sh`: a failed stable-version lookup currently resolves to the literal `latest` instead of failing.
  - `install.ps1`: download, extraction, and move failures surface as exceptions that only reach a `finally` block; they must be caught and reported.
- `install.ps1` detects the operating system and fails early with a pointer to `install.sh` when run in PowerShell on Linux or macOS, where it currently fails with an unrelated error.
- README documents that the install scripts send events and how to query them.

Out of scope: the existing problem that `exit` in a sourced `install.sh` (or in `install.ps1` run via `iex`) ends the user's shell session; installing stable versions via `DTWIZ_TAG`; linking script events to the binary's events.

## Capabilities

### New Capabilities

- `install-script-selfmonitoring`: lifecycle events sent by the bash and PowerShell install scripts, their header and body contract, failure reasons, and the failure detection needed to report them accurately.

### Modified Capabilities

None. The binary's self-monitoring contract (`self-monitoring` spec) is unchanged; script events use their own `dtwiz/install` User-Agent product and share only the routing header and event type.

## Impact

- `scripts/install.sh`, `scripts/install.ps1`: event sending, failure reasons, added failure checks, OS detection in PowerShell.
- `README.md`: self-monitoring section extended for install script events.
- Dynatrace side: additional `CUSTOM_INFO` events (at most two per install run: invoked plus one outcome) count against the environment's event ingest quota. Existing queries filtering on `startsWith(event.name, "dtwiz")` include them.
- No Go code changes. No feature flag: behavior is the same as the binary's always-on self-monitoring.
- Rollback: revert the script changes. The scripts are served from `main` via raw.githubusercontent.com, so a revert takes effect for all new installs immediately.
