## Why

The watch completion event (`st=com`) currently clears the `c=` (command) field from the User-Agent header to save space, but this drops the context of which command triggered the watch session. Because the watch callback fires from multiple call sites (standalone `dtwiz watch`, post-install watches from `install *`, `setup`), the missing command field makes it impossible to distinguish them in HAProxy-captured telemetry.

## What Changes

- The watch completion event always emits `c=wch` in the User-Agent header, with no subcommand (`s=` omitted)
- The `params.Cmd = ""` override in `buildWatchEventCallback` is replaced with `params.Cmd = "watch"` and `params.Sub = ""`
- Correlation between the watch event and the triggering install/setup event is achieved via the shared `execID` (already present in Tab-Id)
- The `selfmonitoring-watch` spec is updated to reflect that `c=wch` is always present and `c=` is never absent

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `selfmonitoring-watch`: The `st=com` event now always carries `c=wch` in the User-Agent; the spec previously stated `c=` was absent from watch completion events

## Impact

- `cmd/selfmonitoring.go`: `buildWatchEventCallback` — remove `params.Cmd = ""`, add explicit `params.Cmd = "watch"` and `params.Sub = ""`
- `openspec/specs/selfmonitoring-watch/spec.md`: update scenarios and worst-case header length calculation
- No behavioral change visible to users; no new dependencies
