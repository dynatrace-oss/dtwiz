# Proposal: K8s Uninstall Dry-Run

## Why

`dtwiz uninstall kubernetes` was the only uninstall subcommand missing `--dry-run` support, making it inconsistent with `oneagent`, `otel`, `aws`, `aws-lambda`, and `azure`. Users had no safe way to preview what the command would do before running it against a live cluster.

## What Changes

- `UninstallKubernetes` accepts a `dryRun bool` parameter
- When `--dry-run` is set, the command prints the cluster context and the four uninstall steps, then exits without running any `kubectl` or `helm` commands

## Capabilities

### New Capabilities

- `k8s-uninstall-dry-run`: Dry-run mode for `dtwiz uninstall kubernetes` — prints the uninstall plan (cluster info + steps) without executing anything

### Modified Capabilities

none — no existing spec-level behavior changes

## Impact

- `pkg/installer/kubernetes_uninstall.go` — `UninstallKubernetes` signature change (added `dryRun bool`)
- `cmd/uninstall.go` — passes `uninstallDryRun` to `UninstallKubernetes`
- `pkg/installer/kubernetes_uninstall_test.go` — all existing call sites updated; new dry-run test added
