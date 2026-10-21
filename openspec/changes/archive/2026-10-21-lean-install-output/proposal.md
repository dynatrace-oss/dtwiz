# Proposal

## Why

`dtwiz install <method>` prints a lot in regular mode: download URLs, file paths, full manifests and configs, the exact CLI commands, one line per sub-step, and several different output styles (`display.*`, raw `fmt.Printf`, `\r` redraws, progress dots). Users cannot tell at a glance what happened, and during longer steps nothing signals that dtwiz is still working before the watch view takes over.

## What Changes

- Add `display.RunStep` (spinner while a step runs, collapsing to `✓ label  detail` or `✗ label` plus the error tail), `display.StepDone`, `display.StepWarn` and `display.Detail` (lines shown only with `-v`/`--debug`).
- Print step lines without animation when stdout is not a terminal or `-v`/`--debug` is active.
- Mark errors already shown under a failed step so the root command does not print them a second time; cobra's own error printing is silenced in favour of one handler.
- Convert the OneAgent, OTel Collector (including the tenant-setup preview and the Python, Node.js and Java runtime executors), Kubernetes, AWS (including Lambda instrumentation), Azure and GCP install flows to steps, with a compact pre-confirmation summary.
- Move commands, URLs, paths, manifests and config dumps to `-v`; `--dry-run` keeps the full plan; `--debug` keeps diagnostics.
- The confirmation prompt for the converted flows reads `Apply?`.
- Not in scope: uninstall commands and the manual Go SDK guidance, which is instructional text.

## Capabilities

### New Capabilities

- `install-step-output`: how install flows present progress, verbose detail and failures.

### Modified Capabilities

None.

## Impact

- `pkg/display`: new `step.go`.
- `pkg/installer/oneagent`, `otel`, `kubernetes`, `aws`, `azure`, `gcp`: output and step structure.
- `cmd/root.go`: single top-level error handler.
- Tests asserting preview output (commands and manifests now require `-v`).
