# Proposal

## Why

Step 1 of .NET auto-instrumentation (`add-dotnet-otel-instrumentation`) finds .NET projects and services but ends in the manual-steps outcome. Steps 2 and 3 of that plan make dtwiz able to prepare the machine: check what the application needs, show the user exactly what will happen, and install the upstream OpenTelemetry .NET automatic instrumentation in a dtwiz-owned directory. They are specified together because a plan preview that cannot be executed, or an installer without a preview and confirmation, is not shippable on its own.

## What Changes

- Specify **step 2: prerequisites and plan**:
  - check that `dotnet` is available and that a runtime matching each service's target framework is installed;
  - build a .NET instrumentation plan for the selected project (per service: supported or not and why, launch form, service name) and show it as a preview before anything changes;
  - after confirmation, run the plan; `--dry-run` and `--yes` behave as for other runtimes.
- Specify **step 3: instrumentation installation**:
  - choose the upstream archive for the operating system, C library and architecture and reject unsupported combinations before downloading;
  - download the latest release, verify it against the published checksum (and the release attestation when the GitHub CLI is usable), extract it into the dtwiz-owned directory and validate the extracted layout;
  - reuse an existing installation when it is current or the release is unreachable; replace it atomically so a failed update never leaves a half-installed directory.
- The plan's execution ends after the installation. Stopping running instances and launching the instrumented services stay in **step 4**; until it ships, a successful install is followed by the manual-steps outcome, and nothing is stopped or started. Token and exporter environment handling is therefore also specified in step 4, since step 2/3 launch nothing.
- Gate everything behind the existing `DTWIZ_DOTNET_OTEL` flag. No new flag.
- Refine the umbrella change's step wording (in `add-dotnet-otel-instrumentation`, not yet archived) so it states that steps 2 and 3 ship together and that stop/relaunch belongs to step 4. That is an edit of the sibling change's artifacts, tracked as a task, not a delta here.
- **No breaking changes.**

## Capabilities

### New Capabilities

- `dotnet-instrumentation-plan`: Step 2. Prerequisite checks, the plan per service, the preview/confirmation flow and the step-2/3 outcome until launching ships.
- `dotnet-instrumentation-install`: Step 3. Platform archive selection, download, verification, extraction into the dtwiz-owned directory, layout validation, reuse and replacement of an existing installation.

### Modified Capabilities

None. The umbrella capability `dotnet-otel-instrumentation` exists only as an unarchived delta in `add-dotnet-otel-instrumentation`; if it is archived before this change, its "Delivery in steps" requirement gets a MODIFIED delta here.

## Impact

- **Flag**: `DTWIZ_DOTNET_OTEL` / `--dotnet-otel` (existing).
- **Code (in follow-up implementation)**:
  - `pkg/installer/otel/dotnet/`: archive selection (OS, libc, architecture), release lookup, download, checksum and attestation verification, extraction and layout validation, with injectable HTTP client and command runner and no imports from `otel`;
  - `pkg/installer/otel/dotnet_plan.go`: the `InstrumentationPlan` adapter (`Runtime`, `PrintPlanSteps`, `Execute`), the prerequisite check and the step-1 outcome replaced by the plan when the flag is on;
  - `pkg/installer/otel/otel.go`: `createRuntimePlan` gains the .NET case.
- **Dependencies**: standard library only (`net/http`, `archive/zip`, `crypto/sha256`); the `gh` CLI is optional and invoked through the existing command-runner seam.
- **Network**: first use needs HTTPS access to the upstream release host; tests use a fake server and never touch the network.
- **Docs**: `CHANGELOG.md` `[Unreleased]` entry when implemented.
- **Rollback**: the flag stays off by default; the only machine change is the dtwiz-owned instrumentation directory, which `dtwiz uninstall` removes in step 5 and which can be deleted by hand without affecting any project.
- **Follow-ups**: step 4 (stop and launch, token/exporter environment), step 5 (uninstall and watch).
