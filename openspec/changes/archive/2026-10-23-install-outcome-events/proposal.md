# Proposal: Install Outcome Events

## Why

The `ist` event mixes two concerns: what the install did (feature outcomes, work time) and how the run ended (error, cancellation). `dtwiz setup` reports the end of the run a second time in its terminal event, so a failed setup install carries the same error twice. `dtwiz install <method>` has no terminal event at all, so queries for failure rates or unfinished runs have to special-case install. Splitting the two concerns gives every command the same ending and makes the upcoming `docs/self-monitoring.md` queries simple.

## What Changes

- The `ist` event reports only what the install did: the feature outcomes and the work time. It no longer carries an error type or error attributes. **BREAKING** for queries that read `error` from `step == "install"`.
- The `ist` event is sent only when install work started. It is no longer sent for dry-run, for a declined confirmation, or for a failure before the confirmation. **BREAKING** for queries that count `ist` events as install attempts.
- `dtwiz install <method>` ends every run that reaches the installer with exactly one terminal event:
  - `com` on success, including dry-run;
  - `can` with error `user_cancelled` when the user declines the confirmation;
  - `fai` with the classified error type and its attributes otherwise.
- `dtwiz setup` keeps its existing terminal events after an install, which now are the only place the install error is reported.
- Pre-install failures (credentials, token validation, client setup) keep sending a single `fai`.
- The post-install watch keeps sending its own events under the `watch` command.
- Covered install subcommands: `oneagent`, `kubernetes`, `otel`, `otel-collector`, `otel-python`, `otel-node`, `otel-java`, `aws`, `aws-lambda`, `azure`, `gcp`, `demo`, and the experimental `docker` (gated by `DTWIZ_EXPERIMENTAL`). Covered setup paths: every selectable method, the update choices, and the demo.

Event sequences after the change:

```text
                          install <method>        setup (install part)
success                   inv → ist → com         … rsl → ist → com
failure after work began  inv → ist → fai(err)    … rsl → ist → fai(err)
failure before work       inv → fai(err)          … rsl → fai(err)
declined confirmation     inv → can               … rsl → can
dry-run                   inv → com               … rsl → com
```

Out of scope: `docs/self-monitoring.md` and the README update, which follow as a separate documentation change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `install-selfmonitoring`: `ist` carries only install results and is sent only when install work started; install runs end with exactly one terminal event; the watch `com` event is the watch session's own event.
- `setup-selfmonitoring`: setup's `ist` follows the same rules; the install error is reported only on setup's terminal event, which this change specifies.

## Impact

- `cmd/selfmonitoring.go`: one helper builds the `ist` event for both install and setup; one helper sends the install terminal event.
- `cmd/install.go`: every install subcommand sends the terminal event after the `ist` event.
- `cmd/setup.go`: the install paths use the shared `ist` helper; terminal events are unchanged.
- `cmd/selfmonitoring_test.go`: tests for the new sequences.
- Queries and dashboards: error and failure-rate queries move from `ist` to `fai`/`can`. Events from older versions keep the old shape, so historical queries need a version cutoff.
- Tenant event volume: about the same; install gains a terminal event and loses `ist` for runs without install work.
