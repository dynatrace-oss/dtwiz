## Context

The watch completion event (`st=com`) is emitted by `buildWatchEventCallback` in `cmd/selfmonitoring.go`. This callback is passed to `installer.WatchIngestWithEvent` and fires when first data arrives across any signal. It can be triggered from:

- `dtwiz watch` (standalone)
- `dtwiz install <method>` (post-install watch)
- `dtwiz setup` (post-install watch)

Currently `buildWatchEventCallback` zeroes `params.Cmd` after calling `buildEventParams`, dropping the `c=` field from the User-Agent. The original reason was header budget: with all 8 signals at 600s max, the full worst-case header including `c=ins;s=otlc` would be 65 chars, one over the 64-char HAProxy capture limit.

## Goals / Non-Goals

**Goals:**
- The watch completion event always carries `c=wch` in the User-Agent
- Correlation between the watch event and the triggering command is achieved via the shared `execID` in Tab-Id
- The User-Agent stays within 64 chars in all cases

**Non-Goals:**
- Changing the `t=` encoding granularity or format
- Surfacing triggering-command information directly in the watch event header

## Decisions

**Watch events always report as `c=wch` with no subcommand.**

Because the watch session is a distinct observability step independent of what triggered it, reporting it always as `c=wch` is semantically correct. The triggering context (`install otel`, `setup`, etc.) is already captured in earlier events sharing the same `execID`. Consumers join on `execID` to reconstruct the full invocation picture.

This also solves the header budget problem without changing the `t=` format: with no subcommand, the worst-case header is:

```
dtwiz/1.9.0;c=wch;st=com;t=600,600,600,600,600,600,600,600  = 58 chars  ✓
```

Alternative considered: strip `c=` only when a subcommand is present (to save space). Rejected because it makes the header format inconsistent and harder to query.

Alternative considered: change `t=` to 10-second granularity to save 8 chars. Rejected because it loses resolution without any real gain now that the simpler fix works.

## Risks / Trade-offs

Queries that previously filtered on absence of `c=` to find watch events will now need to filter on `c=wch` — but since the field was previously absent, such queries would have been unreliable anyway.

No rollback needed: this is additive (field present vs. absent in a telemetry header) and gated by `DTWIZ_SELF_MONITORING_POC`.
