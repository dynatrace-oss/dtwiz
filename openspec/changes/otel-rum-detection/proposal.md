# Proposal

## Why

`dtwiz install otel` instruments server-side code but leaves frontend performance invisible. Real User Monitoring (RUM) closes that gap by injecting a JavaScript tag into the application's HTML. Before injection can happen, dtwiz must determine whether auto-injection is possible: whether the project has static HTML files that can be modified directly, and whether a framework or build tool prevents direct HTML modification.

This change adds the detection layer: a cross-platform scanner that classifies a project directory as auto-injectable or requiring manual setup, producing a structured result consumed by subsequent steps of the RUM onboarding flow.

## What Changes

- Add a new package `pkg/installer/otel/rum/` containing the detection logic.
- Scan the project directory for `.html` files, excluding build output directories.
- Detect frameworks and build tools by inspecting config files and `package.json` dependencies.
- Determine injection mode (`auto` or `manual`) and populate a result struct with injectable files, the chosen mode, and the reason for manual mode when applicable.

## Capabilities

### New Capabilities

- `rum-detection`: Scan a project directory and return a structured result classifying whether RUM auto-injection is possible and which HTML files are candidates for injection.

### Modified Capabilities

- None. The OTel installer is not modified in this change; it will call the detector in a follow-on change.

## Impact

- New package `pkg/installer/otel/rum/` with no external dependencies beyond the Go standard library.
- No changes to CLI commands, flags, or UX in this change.
- No new Dynatrace API calls.
- Table-driven unit tests using in-memory temp directories; no filesystem side effects in CI.
