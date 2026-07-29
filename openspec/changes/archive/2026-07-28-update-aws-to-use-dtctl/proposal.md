# Proposal

## Why

The AWS CloudFormation integration used `pkg/extensions` (a custom HTTP client package wrapping `*client.PlatformClient`) and required callers to inject `c.Platform` from `setupClient()`. Azure and GCP had already moved to the shared `installer.ExtensionClient` backed by the `dtctl` SDK. AWS was the odd one out: a custom HTTP path that bypassed the shared client, required three extra API layers, and made AWS the only cloud installer where commands had to call `setupClient()`.

Additionally, the AWS monitoring configuration was deployed but never enabled — the `enableMonitoringConfig` step (flip `enabled` on the config and its credential entries via GET+PUT) was missing, so CloudFormation stacks were deployed but data never flowed until manually enabled.

## What Changes

- Move AWS CloudFormation install/uninstall to `pkg/installer/aws/` subdirectory, matching the Azure and GCP structure.
- Replace `pkg/extensions` + `*client.PlatformClient` with `installer.ExtensionClient` (dtctl SDK), using the same shared wrapper already used by Azure and GCP.
- Add `enableMonitoringConfig`: GET monitoring config → flip `value.enabled` and all `value.aws.credentials[].enabled` to `true` → PUT.
- Add `dtclient` interface for testability; split public entry points from internal `installAWSWithClient` / `uninstallAWSWithClient` functions that accept an injected client.
- Remove `c.Platform` from AWS CLI commands; commands resolve credentials only via `getDtEnvironment()` + `validateCredentials()`, matching Azure and GCP.
- Trim `pkg/installer/aws.go` to two shared helpers: `GetAWSCallerInfo()` and `IsAWSCLIInstalled()`, which remain in the `installer` package to be accessible from `aws_lambda.go` without an import cycle.

## Capabilities

### New Capabilities

None — this is an internal refactor. User-visible behavior is unchanged.

### Modified Capabilities

- `aws-monitor-install`: Now uses dtctl SDK instead of `pkg/extensions`; now enables the monitoring configuration after CFN deployment so data flows without manual intervention.
- `aws-monitor-uninstall`: Now uses dtctl SDK instead of `pkg/extensions`.

## Impact

- New package `pkg/installer/aws/`: `config.go`, `dtapi.go`, `install.go`, `uninstall.go`, `dtapi_test.go`.
- `pkg/installer/aws.go`: stripped from ~511 lines to ~52 lines (only shared helpers).
- `pkg/installer/aws_uninstall.go`: deleted (moved to `pkg/installer/aws/uninstall.go`).
- `pkg/installer/aws_test.go`: deleted (replaced by `pkg/installer/aws/dtapi_test.go`).
- `pkg/installer/aws_lambda.go`: two function renames only (`getAWSCallerInfo` → `GetAWSCallerInfo`, `isAWSCLIInstalled` → `IsAWSCLIInstalled`).
- `cmd/install.go`, `cmd/uninstall.go`, `cmd/setup.go`: import `awspkg` alias; drop `setupClientFromCreds` call; call `awspkg.InstallAWS` / `awspkg.UninstallAWS` directly.
- `pkg/extensions` package: no longer used by AWS; remains for any future use.
- No changes to user-facing flags, commands, or output format.

## Rollback Plan

Revert all files in `pkg/installer/aws/`, restore `pkg/installer/aws.go` and `pkg/installer/aws_uninstall.go` to their pre-migration state, revert the three cmd files to call `installer.InstallAWS` / `installer.UninstallAWS` with `c.Platform`, and restore the function names in `aws_lambda.go`.
