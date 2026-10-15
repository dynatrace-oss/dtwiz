## Why

RUM onboarding cannot always inject the RUM JavaScript tag automatically: frameworks such as Next.js and Nuxt have no source HTML file, and some projects have no writable HTML at all (see `otel-rum-detection`). For those cases dtwiz needs a fallback that prints the tag together with short instructions, so the user can insert it manually and link the application to a Dynatrace frontend, following the [agentless monitoring guide](https://docs.dynatrace.com/docs/observe/digital-experience/rum/web-frontends/initial-setup/set-up-agentless-monitoring).

## What Changes

- Add `PrintManualInstructions(w io.Writer, in ManualInstructions)` to the new package `pkg/installer/otel/rum/`.
- `ManualInstructions` carries the tag plus an optional reason (why automatic injection is not possible) and an optional hint (where to put the tag in the detected framework). The caller decides the content; the printer only renders it.
- The output prints the tag on its own line, flush left and uncolored so it can be copied as is, followed by the placement, verification, and docs lines from the agentless monitoring guide.
- The docs line links to the [RUM documentation](https://docs.dynatrace.com/docs/observe/digital-experience/rum). The URL is a package constant so a later switch to a short link changes one line.
- The function is not called from any command yet. The tag is a plain string; tests use a fixed dummy tag. A test patch in the PR description lets reviewers see the output.

## Capabilities

### New Capabilities

- `rum-manual-instructions`: Render the RUM JavaScript tag and manual insertion instructions to a writer, with optional reason and framework hint lines.

### Modified Capabilities

- None.

## Impact

- New file(s) in `pkg/installer/otel/rum/`; standard library and `pkg/display` only.
- No CLI, flag, or UX changes for users: nothing calls the new function yet.
- No Dynatrace API calls, no telemetry changes (`install.rum` stays `not_tried`), no changes to `WatchIngest`.
- No feature flag needed, because the code is not reachable from any command. Rollback is deleting the package.
- Follow-on changes (not part of this one): wiring into `dtwiz install otel` (preview, dry run never prints a tag, footer hint in the watch), fetching the real tag from the frontend created in step 2, and a body-only `install.rum_mode` telemetry property.
