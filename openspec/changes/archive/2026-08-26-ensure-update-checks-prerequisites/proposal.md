# Proposal: ensure-update-checks-prerequisites

## Why

`dtwiz update otel` on a Dynatrace-managed collector updates credentials and restarts the collector, but never verifies that the tenant-side prerequisites (OTel Host Monitoring extension and OpenPipeline dynamic routes) are in place. If either was removed or never set up, the update silently leaves host monitoring broken with no indication to the user.

## What Changes

- Before the existing confirmation prompt in `updateDynatraceCollector`, show the same extension activation status and OpenPipeline route plan preview that `install otel` already shows.
- After the user confirms, activate the extension (if needed) and apply any missing or disabled dynamic routes, using the same post-confirmation logic as `install otel`.
- Both additions are gated behind `--experimental` / `DTWIZ_EXPERIMENTAL=true` and require a platform token, matching the install flow's guards exactly.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `otel-extension-activation`: extend to also cover `dtwiz update otel` — the Dynatrace collector update path SHALL show the extension status in its preview and ensure the extension is active after confirmation, using the same guard (`--experimental` + platform token) as `install otel`.
- `otel-host-monitoring-grail-routes`: extend to also cover `dtwiz update otel` — the Dynatrace collector update path SHALL show the OpenPipeline route plan in its preview and apply missing or disabled routes after confirmation, using the same logic (including the bounded pipeline wait and plan rebuild) as `install otel`.

## Impact

- `pkg/installer/otel/update_dynatrace.go` — restructured to defer the up-to-date exit so tenant-side prerequisite reconciliation runs even when the config file has not changed.
- `pkg/installer/otel/grail_routes.go` — `buildGrailRoutePlansFn` injectable function pointer added here (not pre-existing) to make the preview step stubbable in tests.
- `pkg/installer/otel/update_dynatrace_test.go` — new and updated tests covering the prerequisite preview, post-confirmation reconciliation, dry-run gating, and the up-to-date config path.
- `openspec/changes/ensure-update-checks-prerequisites/` — proposal, design, tasks, and spec files for this change.
- No changes to `updateOtelConfig` (third-party collector path): that path injects a forwarding exporter and does not involve host monitoring.
- No new feature flags or CLI flags.
- The experimental flag (`featureflags.Experimental`) already gates this behavior in install; update adopts the same gate with no registry changes needed.
