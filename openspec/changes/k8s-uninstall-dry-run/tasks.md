# Tasks: K8s Uninstall Dry-Run

## 1. Installer

- [x] 1.1 Extract `printK8sUninstallSteps()` helper in `pkg/installer/kubernetes_uninstall.go`
- [x] 1.2 Add `handleK8sUninstallDryRun()` that prints the plan and returns without executing
- [x] 1.3 Add `dryRun bool` parameter to `UninstallKubernetes` and branch on it before the confirmation prompt

## 2. Command Wiring

- [x] 2.1 Pass `uninstallDryRun` to `UninstallKubernetes` in `cmd/uninstall.go`

## 3. Tests

- [x] 3.1 Update all existing `UninstallKubernetes` call sites in `pkg/installer/kubernetes_uninstall_test.go` to pass `false`
- [ ] 3.2 Add `TestUninstallKubernetes_DryRun` — verify no kubectl/helm commands run, output contains step descriptions, function returns nil
