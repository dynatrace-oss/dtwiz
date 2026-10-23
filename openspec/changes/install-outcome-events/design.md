# Design: Install Outcome Events

## Context

Install events are built in two places today:

```text
cmd/install.go (13 subcommands)           cmd/setup.go (methods, updates, demo)
──────────────────────────────            ─────────────────────────────────────
inv            (PersistentPreRun)         inv  (root PersistentPreRun)
fai on pre-install error                  ana, rpr…, rsl
installErr := <installer>                 fai on pre-install error
fireInstallEvent ── ist(err?, report?)    installErr := <installer>
                                          fireSetupInstallEvent ── ist(err?, report?)
                                            (skipped on cancel / ErrUpToDate)
                                          can | fai(err) | com
watch: snp… com (command=watch)           watch: snp… com (command=watch)
```

`fireInstallEvent` and `fireSetupInstallEvent` (`cmd/selfmonitoring.go`) duplicate the error classification and both call `applyInstallOutcomes`, which attaches the install report only when install work started and the error is not a cancellation. Setup therefore reports a failed install's error twice (`ist` and `fai`); direct install reports it only on `ist` and has no terminal event.

Events are sent by `eventSink`, one goroutine per event, so two events fired back to back can arrive in either order.

## Goals / Non-Goals

**Goals:**

- `ist` carries only install results (feature outcomes, work time) and is sent only when there are results to report.
- Every install run that reaches the installer, direct or via setup, ends with exactly one `com`, `fai`, or `can`, which is the only place the error appears.
- One function builds `ist` for both paths.

**Non-Goals:**

- Changing the install report contents, the header encoding of `f=` and `d=`, or the watch events.
- Changing setup's terminal-event logic beyond what the shared helper requires.
- Fixing other stale parts of the `setup-selfmonitoring` spec (feature flag, `rec` step name).
- Documentation (`docs/self-monitoring.md`, README).

## Decisions

### `ist` is sent if and only if the install report is attached

`applyInstallOutcomes` already decides whether install work started. The new `fireInstallReportEvent(cmd, sub, err)` builds the `ist` params, applies the outcomes, and returns without sending when no report was attached. It never sets `Err` or `ExtraProps`. `sub` overrides the subcommand for setup, where the method is not the cobra subcommand; direct install passes `""`.

Alternatives considered:

- *Keep sending an empty `ist` for dry-run and cancellation.* It carries no information once the error moves to the terminal event, and it makes "count of `ist`" ambiguous.
- *Send `ist` only on success.* Loses the outcomes reached before a failure, for example Host Monitoring activated before a collector download failed.

### One terminal helper for direct install

`fireInstallEvents(cmd, err)` calls `fireInstallReportEvent(cmd, "", err)` and then the terminal event:

```text
err == nil                          → com
errors.Is(err, ErrInstallCancelled) → can  (classified: user_cancelled)
otherwise                           → fai  (classified, with attributes)
```

The 13 call sites in `cmd/install.go` switch to it, so a new install method cannot forget the terminal event.

### Setup keeps its terminal branches

Setup's install paths replace `fireSetupInstallEvent(cmd, method, err)` with `fireInstallReportEvent(cmd, method, err)` and keep their existing `can`/`fai`/`com` branches, which already handle `otel.ErrUpToDate`. Sharing the terminal helper with setup would need an `ErrUpToDate` special case that direct install never hits; two short branches are clearer than one helper with a setup-only flag.

`fireSetupInstallEvent` and `fireInstallEvent` are removed once their callers are migrated.

### Ordering is not part of the contract

The spec describes the order events are fired in, and states that arrival order is not guaranteed. Since `ist` and the terminal event share no fields, queries combine them by `executionId` and never need ordering.

## Risks / Trade-offs

- [Queries that read `error` from `step == "install"` stop matching new versions] → The removal is called out as **BREAKING** in the proposal; `docs/self-monitoring.md` will document the version cutoff and query the terminal events instead.
- [Fewer `ist` events can look like fewer installs on existing dashboards] → Install attempts are counted from terminal events with `command == "install"` or from setup's `rsl`; the doc will say so.
- [Two `com` events per install run with a watch] → They differ in `command`; a spec scenario pins this.

## Migration Plan

No data migration. Older dtwiz versions keep sending the old shape; queries covering both can read `error` from `ist` for old versions and from `fai`/`can` for new ones. Rollback is reverting the change.

## Open Questions

- Setup's up-to-date path sends `can` through `fireSelfMonitoringEventWithError(…, ErrUpToDate)`, and `ClassifyError` maps `ErrUpToDate` to the `install_failed` fallback. A `cancelled` event with `error=install_failed` looks wrong. This change keeps the behavior; it belongs to the separate inconsistencies discussion together with `update otel`, which does the same.
