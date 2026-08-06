# Design: otel-host-monitoring-extension

## Context

`dtwiz install otel --experimental` configures the OTel Collector with host metrics and logs receivers. The data is shaped to match what the Dynatrace OpenTelemetry Host Monitoring extension (`com.dynatrace.extension.opentelemetry`) expects, but the extension itself is not installed or activated as part of the flow. As a result, host and process entity creation and I&O visualizations never appear despite correct data arriving.

The dtctl SDK (`github.com/dynatrace-oss/dtctl/sdk@v0.2.0`) provides `extension.Handler.InstallFromHub` for hub installs, but does not expose the `POST /platform/extensions/v2/extensions/{name}/environment-configuration` endpoint needed to activate an installed version. `ExtensionClient` in `pkg/installer/extension_client.go` already wraps the SDK for use by azure/gcp/aws installers and provides the building blocks (`EnsureInstalled`, `IsExtensionActive`, `LatestExtensionVersion`) but lacks an activate method.

## Goals / Non-Goals

**Goals:**

- Ensure `com.dynatrace.extension.opentelemetry` is installed from the Dynatrace Hub if not already present
- Explicitly activate the installed version via the environment-configuration endpoint
- Advisory behavior: a failure or timeout during activation logs a warning but does not abort the collector install
- Gated behind `featureflags.Experimental`, consistent with the rest of host monitoring behavior

**Non-Goals:**

- Activation during `update otel` (deferred; revisit if straightforward)
- Creating monitoring configurations for the extension (not required for host monitoring; the extension works at environment scope with no per-host config)
- Exposing a user-facing flag to skip extension activation
- Modifying the dtctl SDK

## Decisions

**Decision 1: Add `ActivateExtension` directly to `ExtensionClient` via raw REST, not through the SDK.**

The SDK's `extension.Handler` does not expose the environment-configuration endpoint. Options were: (a) add a method to `ExtensionClient` that calls the endpoint directly using the underlying `httpclient.Client`, or (b) wait for the SDK to add support. We choose (a) — the existing `ExtensionClient` already has access to `e.C` (the `httpclient.Client`) and the pattern is already established for SDK gaps. This keeps the change self-contained.

**Decision 2: Placement in `otel.go`, not `collector.go`.**

The extension activation is not collector configuration — it is a tenant-side prerequisite. `collector.go` handles config generation and binary execution. `otel.go` owns the overall install orchestration. The activation step belongs in `InstallOtelCollectorWithProject` in `otel.go`, after `ConfirmProceed()` and before `cp.execute()`, mirroring how azure/gcp/aws installers handle extension prerequisites before their main install steps.

**Decision 3: Advisory failure mode.**

If `ActivateExtension` or `EnsureInstalled` fails, log a warning and continue rather than aborting. The OTel Collector will still work for service monitoring even if extension activation fails. This matches the principle that monitoring data should flow even when visualization prerequisites are partially missing.

**Decision 4: No polling after `ActivateExtension`.**

Unlike hub install (which is async and requires `WaitForExtensionActive` polling), the environment-configuration endpoint is synchronous. After a successful `ActivateExtension` call, the extension is active. No additional polling is needed. `WaitForExtensionActive` is not used in this flow.

**Decision 5: `LatestExtensionVersion` after `EnsureInstalled` to get the version to activate.**

`EnsureInstalled` returns a bool indicating whether a fresh install was triggered, but not the version. `LatestExtensionVersion` provides the version regardless of whether it was just installed or already present. This avoids modifying `EnsureInstalled`'s signature and keeps the logic in the caller.

## Risks / Trade-offs

- **Extension name assumption**: The extension ID `com.dynatrace.extension.opentelemetry` was identified from Hub CDN metadata. If the actual ID differs on some tenants or environments, activation will fail silently (advisory mode). Mitigation: define the name as a package-level constant so it is easy to correct.

- **Activation on already-active extension**: Calling `POST /environment-configuration` when the extension is already active should be idempotent (the API updates the active version). If the API returns an error for "already active with same version", `ActivateExtension` should treat it as success, similar to how `InstallExtension` handles 409 Conflict.

- **Token permissions**: Activation requires `extensions:definitions:write`. Users who provide a platform token without this scope will see an advisory warning. No mitigation beyond clear log output.
