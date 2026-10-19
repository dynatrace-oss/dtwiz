# Proposal

## Why

`otel-rum-detection` determines whether and where a RUM JavaScript tag can be auto-injected into a
project, but deliberately stops short of obtaining the tag itself — that was named as a follow-on.
Before any injection or manual-print step can run, dtwiz needs the actual JavaScript tag content
for the Smartscape FRONTEND entity that represents the user's application.

This change adds that retrieval: given a known FRONTEND entity ID, fetch its current JavaScript
tag from the Dynatrace Platform API. Resolving the FRONTEND entity itself (searching for or
creating it) is a separate concern and is explicitly out of scope — this change assumes the ID is
already known.

## What Changes

- Add JavaScript tag retrieval to `pkg/installer/otel/rum/`: given a Smartscape FRONTEND entity
  ID, fetch the most recent OpenTelemetry RUM JavaScript tag text from the Dynatrace Platform API.
- An unknown or non-existent FRONTEND entity ID is reported as a distinct, dedicated failure rather
  than a generic error.
- Only the OpenTelemetry-based tag is covered. The OneAgent-based tag variant is explicitly out of
  scope for this change.
- The FRONTEND entity ID is not resolved dynamically in this change — real entity discovery or
  creation is deferred to a later change. The capability is validated through its own test suite
  only; it is not wired into the `dtwiz install otel` command flow yet.

## Capabilities

### New Capabilities

- `rum-javascript-tag`: Given a known Smartscape FRONTEND entity ID, retrieve the current
  OpenTelemetry RUM JavaScript tag.

### Modified Capabilities

- None.

## Impact

- New file(s) in `pkg/installer/otel/rum/`, additive to the detection logic introduced by
  `otel-rum-detection` (not yet implemented at the time of this proposal).
- One new Dynatrace Platform API call.
- No CLI flags, no user-facing output, and no changes to `dtwiz install otel`'s command flow —
  mirrors how `otel-rum-detection` landed detection as a standalone, not-yet-consumed capability.
  A later change wires detection, tag retrieval, and injection/printing together end to end.
