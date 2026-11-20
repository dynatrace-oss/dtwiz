# Proposal

## Why

`dtwiz watch` only ends when someone presses Enter or answers the 10-minute "Continue watching?" prompt. Scripts and AI agents that run it just to confirm that data is reaching Dynatrace have no clean way to get an answer and continue. They have to type into the terminal or kill the process. A flag that ends the watch as soon as data shows up turns the command into a quick check.

## What Changes

- Add a `--exit-on-data` flag to `dtwiz watch`.
- With the flag, the watch ends on its own after the first poll that finds data in any section, and leaves the last frame on screen.
- With the flag, the watch ends at the 10-minute mark when no data arrived, without showing the "Continue watching? [Y/n]" prompt, on a TTY too.
- With the flag, pressing Enter still ends the watch early, as it does today.
- The command exits with code `0` in all of these cases, as today.
- Without the flag, `dtwiz watch` behaves exactly as today.
- The watches that start after `install`, `update` and `setup` do not change. Their `--yes` flag keeps its meaning of skipping confirmation prompts and does not end the watch early.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ingest-watch`: adds a requirement for the `--exit-on-data` flag on the standalone `dtwiz watch` command, covering exit on first data, exit at the 10-minute mark without a prompt, and the unchanged post-install watch.
- `selfmonitoring-watch`: adds a requirement that a session ended by the flag reports like any other session end, and that the snapshot event for the data that ended it is delivered before the process exits.

## Impact

- `cmd/watch.go`: new flag, passed to the watch.
- `pkg/installer/ingest_watch.go`: the watch loop gets an exit-on-data mode, reached through a new entry point used only by `dtwiz watch`. The existing entry points used by installers keep their behaviour.
- Self-monitoring: a new end path for the watch session; no new event types or fields.
- Docs: `README.md`, `AGENTS.md` and `skills/dtwiz/SKILL.md` list `dtwiz watch` flags and need the new one.
- No feature flag: the behaviour is opt-in through the CLI flag and does not affect existing calls. Rollback is removing the flag.
