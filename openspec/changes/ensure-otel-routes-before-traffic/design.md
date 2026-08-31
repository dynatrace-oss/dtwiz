# Design: Ensure OTel Routes Before Traffic

## Context

`dtwiz install otel` currently builds a route preview before confirmation, then activates the OpenTelemetry Host Monitoring extension after confirmation. After that, the flow splits: if the user does not select a project, dtwiz starts the collector and sends a synthetic OTLP verification log; if the user selects a project, dtwiz starts the collector, skips the synthetic verification log, and starts the selected application with OTel instrumentation. In both branches, OpenPipeline dynamic routes are applied only after collector verification or application instrumentation has already had a chance to emit telemetry. The logs route matcher requires host attributes and sends matching OTLP logs into the OTel Host Monitoring pipeline. If a first-run verification log is emitted before the route exists, the verification still proves ingestion but not host-monitoring assignment.

The collector config already adds system resource detection to the logs pipeline, so the local payload has the attributes needed by the route. The gap is ordering.

## Goals / Non-Goals

**Goals:**

- Apply and validate OTel Host Monitoring dynamic routes after extension activation and before collector verification telemetry is sent.
- Apply routes before selected app instrumentation starts emitting telemetry.
- Preserve dry-run, preview, single confirmation, and advisory route failure behavior.
- Keep the same route reconciliation logic and bounded wait semantics.

**Non-Goals:**

- Change route matchers or the collector template.
- Make route failures fatal.
- Add new tenant APIs or change update/uninstall behavior.

## Decisions

1. Move route application earlier in the install sequence.

   After confirmation, the installer will activate the extension, apply dynamic routes, validate successfully applied routes with a bounded readback, then start the collector and any selected application instrumentation. This reuses the existing preview snapshot, post-activation pipeline wait, and final plan rebuild. Alternative considered: send a second verification log after route application. That would prove routing for collector-only installs, but it would not prevent app telemetry from racing ahead of route setup.

2. Keep route application advisory.

   Existing behavior treats route write failures as warnings and continues the install. Validation failures follow the same model so an API-side propagation issue does not block basic OTLP ingestion. Alternative considered: aborting before collector start when route setup or validation fails. That would improve host-assignment guarantees but would be a larger behavior change and could prevent useful telemetry from flowing.

3. Apply the same ordering to `install otel-collector`.

   The standalone collector path also sends a verification log and already previews/applies host-monitoring routes. Moving its route application before collector execution keeps both OTel install entry points consistent.

## Risks / Trade-offs

- Route setup now happens before collector startup, so the user may see the existing bounded route wait earlier in the flow. Mitigation: the wait is already bounded and advisory.
- If route setup fails, the verification log can still be unassigned to host monitoring. Mitigation: preserve the visible route warning so the user knows to re-run once prerequisites are ready.
