# Design

## Context

The AWS CloudFormation integration predated the shared `installer.ExtensionClient` abstraction. When Azure and GCP were added, they used the `dtctl` SDK through a shared client wrapper. AWS was left on the old path: inline HTTP calls via `pkg/extensions` functions that took a `*client.PlatformClient`, which in turn required `setupClient()` in every cmd handler. This created three divergences from the cloud-installer pattern:

1. AWS cmd handlers called `setupClient()` + `setupClientFromCreds()` (Azure/GCP only call `getDtEnvironment()` + `validateCredentials()`).
2. AWS function signatures took `c *client.PlatformClient` as an argument; Azure/GCP take only `envURL, token string`.
3. AWS had no `enableMonitoringConfig` step — monitoring configs were created but left disabled.

## Goals / Non-Goals

**Goals:**

- Make AWS structurally identical to Azure and GCP (same package layout, same client abstraction, same cmd wiring).
- Fix the missing `enableMonitoringConfig` step so installed stacks actually emit data.
- Keep the `aws_lambda.go` helpers accessible without introducing an import cycle.

**Non-Goals:**

- Changing user-visible behavior, flags, or output.
- Removing `pkg/extensions` (it may be used in future).

## Decisions

### 1. `pkg/installer/aws/` subdirectory, matching Azure and GCP

Azure lives in `pkg/installer/azure/`, GCP in `pkg/installer/gcp/`. AWS now lives in `pkg/installer/aws/`. Each subdirectory owns: `config.go` (types and constants), `dtapi.go` (the `dtclient` interface + `sdkDTClient` impl), `install.go`, `uninstall.go`, and tests. The shared entry point pattern is identical: `Install<Cloud>(envURL, token string, ...) error` creates a real client and delegates to `install<Cloud>WithClient(..., dtc dtclient)`.

### 2. `installer.ExtensionClient` as the shared client abstraction

`installer.ExtensionClient` bundles `httpclient.Client`, `settings.Handler`, and `extension.Handler` from the dtctl SDK. `sdkDTClient` in `pkg/installer/aws/dtapi.go` embeds `*installer.ExtensionClient` and implements the `dtclient` interface by delegating to `d.Extension.*` methods. This is the same pattern used by Azure and GCP.

### 3. `dtclient` interface for testability

The `dtclient` interface is consumed by `installAWSWithClient` and `uninstallAWSWithClient` — the testable cores. Tests inject a mock that implements `dtclient`. The public entry points are thin wrappers that build a real `sdkDTClient` and delegate. This matches the Azure and GCP test patterns.

### 4. `enableMonitoringConfig` via GET+PUT round-trip

The dtctl SDK's `UpdateMonitoringConfiguration` replaces the entire config, so a full GET first is required. `enableMonitoringConfig`:

1. GETs the monitoring configuration by objectId.
2. Unmarshals the `scope` and `value` fields.
3. Sets `value["enabled"] = true`.
4. Iterates `value["aws"]["credentials"]` and sets `enabled = true` on each entry.
5. PUTs the modified document back.

This ensures both the top-level config and all per-credential `enabled` flags are set, which is required for data to flow.

### 5. `GetAWSCallerInfo` and `IsAWSCLIInstalled` stay in `pkg/installer`

`aws_lambda.go` (in `package installer`) calls both helpers. Moving them to `pkg/installer/aws/` would create an import cycle (aws/ imports installer, installer imports aws/). They are exported from `pkg/installer/aws.go` and called by both `aws_lambda.go` and the new `pkg/installer/aws/` package without a cycle.

### 6. AWS cmd handlers no longer call `setupClient()`

Before this change, `installAWSCmd` and `uninstallAWSCmd` called `setupClientFromCreds()`. After, they call `getDtEnvironment()` + `validateCredentials()` only, matching the Azure and GCP cmd handlers exactly. The `pkg/client.Client` struct (Classic + Platform pair) is not needed by AWS.
