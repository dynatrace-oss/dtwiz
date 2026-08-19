## Context

OTel host monitoring currently affects several parts of the OTel flow: collector config generation, install messaging, tenant extension activation, OpenPipeline route setup, uninstall cleanup, and Dynatrace collector config regeneration. These behaviors are complete but are guarded by `featureflags.Experimental`, which also controls unrelated experimental features.

## Goals / Non-Goals

Goals:

- Make host monitoring the default behavior for `dtwiz install otel`.
- Preserve dry-run safety: previews may be shown, but no extension activation, route writes, process changes, or file changes occur during dry run.
- Preserve user choice on uninstall by keeping the Delete all / Only collector / Cancel prompt.
- Leave unrelated `Experimental` flag behavior unchanged.

Non-goals:

- Making `install docker`, `install demo`, or `update otel` generally available.
- Changing the host-monitoring collector template shape.
- Changing tenant API failure semantics; extension and route failures remain advisory during install/uninstall.

## Decisions

- Remove `Experimental` checks only from OTel host-monitoring behavior. The feature flag remains registered for other experimental commands.
- Keep platform-token checks around tenant previews and writes. If no platform token is available, local collector installation continues with warnings/debug output as today.
- For uninstall, always render the extension/routes removal preview and use the host monitoring removal prompt. Selecting Only collector keeps the extension and routes on the tenant.
- For `update otel`, regenerated Dynatrace collector configs include host-monitoring settings by default even though the `update otel` command itself remains experimental.

## Risks / Trade-offs

- Default install now attempts tenant extension and OpenPipeline changes. Existing behavior treats those failures as advisory, reducing risk but requiring clear preview/warning output.
- Default collector config is larger and may require elevated privileges for full host/process coverage. Existing platform notices remain in the install output.
- Uninstall now presents three choices by default. This is intentional because deleting tenant routes and extensions is more destructive than local cleanup.
