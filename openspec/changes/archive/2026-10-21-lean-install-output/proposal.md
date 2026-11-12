# Proposal

## Why

`dtwiz install <method>` prints a lot in regular mode: download URLs, file paths, full manifests and configs, the exact CLI commands, one line per sub-step, and several different output styles (`display.*`, raw `fmt.Printf`, `\r` redraws, progress dots). Users cannot tell at a glance what happened, and during longer steps nothing signals that dtwiz is still working before the watch view takes over.

## What Changes

- Add `display.RunStep` (spinner while a step runs, collapsing to `✓ label  detail`, or `✗ label  failed` plus the error tail), `display.StepDone`, `display.StepWarn` and `display.Detail` (lines shown only with `-v`/`--debug`).
- Print step lines without animation when stdout is not a terminal. With `-v`/`--debug` the spinner stays on and log lines and subprocess output clear the spinner line first, so the current step is always visible.
- Print an installer's section header only once: `dtwiz setup` already prints "Installing: …", so installers skip their own header there (`display.InstallHeader`).
- Mark errors already shown under a failed step so the root command does not print them a second time; cobra's own error printing is silenced in favour of one handler.
- Convert the OneAgent, OTel Collector (including the tenant-setup preview and the Python, Node.js and Java runtime executors), Kubernetes, AWS (including Lambda instrumentation), Azure and GCP install flows to steps, with a compact pre-confirmation summary.
- Apply the same style to the `uninstall` commands (kubernetes, aws, aws-lambda, azure, gcp, otel, oneagent, self on Unix), the azure, gcp and otel `update` commands, `install demo` and `install docker`.
- Show started services (Python, Node.js, Java) as per-service `✓`/`✗` lines instead of a free-text summary.
- Add `display.RunQuiet` (spinner without a result line) so phases whose helpers print their own result lines, such as tenant setup, lookups and scans, still show progress.
- Use the same compact preview, `dtwiz · <name>` header and `Apply?` prompt in the standalone `install otel-python`, `otel-node` and `otel-java` commands, and skip the nested preview and prompt in the demo flow, which already showed and confirmed its own plan.
- Move commands, URLs, paths, manifests and config dumps to `-v`; `--dry-run` keeps the full plan; `--debug` keeps diagnostics.
- The confirmation prompt for the converted flows reads `Apply?`.
- Not in scope: the manual Go SDK guidance and the Windows self-uninstall instructions, which are instructional text.

## Capabilities

### New Capabilities

- `install-step-output`: how install flows present progress, verbose detail and failures.

### Modified Capabilities

None.

## Impact

- `pkg/display`: new `step.go`; `pkg/logger`: log output goes through a replaceable writer.
- `pkg/installer` (including AWS Lambda, docker, self uninstall), `oneagent`, `otel`, `kubernetes`, `aws`, `azure`, `gcp`: output and step structure.
- `cmd/setup.go`: marks its own install header as shown.
- `cmd/root.go`: single top-level error handler.
- Tests asserting preview output (commands and manifests now require `-v`).
