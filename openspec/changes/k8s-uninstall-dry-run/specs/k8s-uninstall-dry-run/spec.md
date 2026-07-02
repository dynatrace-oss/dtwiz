# Spec: K8s Uninstall Dry-Run

## ADDED Requirements

### Requirement: Dry-run mode prints uninstall plan without executing

When `--dry-run` is passed, `dtwiz uninstall kubernetes` SHALL print the cluster context (if available) and the four uninstall steps, then exit with a zero return value. It SHALL NOT execute any `kubectl` or `helm` commands.

#### Scenario: Dry-run with cluster context

- **WHEN** `UninstallKubernetes` is called with a non-empty `kubeCtx` and `dryRun=true`
- **THEN** the output contains the cluster info line and all four step descriptions
- **THEN** no kubectl or helm commands are invoked
- **THEN** the function returns nil

#### Scenario: Dry-run with empty cluster context

- **WHEN** `UninstallKubernetes` is called with an empty `kubeCtx` and `dryRun=true`
- **THEN** no cluster info line appears in output
- **THEN** the four step descriptions are still printed
- **THEN** no kubectl or helm commands are invoked
- **THEN** the function returns nil
