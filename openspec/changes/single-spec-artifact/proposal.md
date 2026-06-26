## Why

`dtwiz uninstall kubernetes` would stop at the first failing step, leaving the cluster in a dirty state — most commonly a lingering `dynatrace` namespace when helm had no release to uninstall. Error output was also noisy: the usage block appeared on runtime errors, and every error was printed twice.

## What Changes

- `UninstallKubernetes` now runs all 4 steps regardless of individual failures; errors are printed inline and collected, returning a single sentinel at the end
- Usage block is suppressed once a command starts executing (`PersistentPreRun` fires), still shown for invalid flags / unknown subcommands
- Duplicate error print in `Execute` removed — Cobra's `ExecuteC` already prints errors from `RunE`

## Capabilities

### New Capabilities

- `k8s-uninstall-resilience`: Kubernetes uninstall continues all steps on partial failure, ensuring namespace cleanup even when the helm release is already gone

### Modified Capabilities

<!-- none -->

## Impact

- `pkg/installer/kubernetes_uninstall.go` — step error handling and return logic
- `cmd/root.go` — `SilenceUsage` set in `PersistentPreRun`; duplicate `fmt.Fprintln` removed from `Execute`
- `cmd/install.go`, `cmd/update.go`, `cmd/uninstall.go` — `SilenceUsage` set in their overriding `PersistentPreRun` hooks
