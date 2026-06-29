# Tasks

## 1. Uninstall Step Resilience

- [x] 1.1 Refactor `UninstallKubernetes` in `pkg/installer/kubernetes_uninstall.go` to collect step errors into `[]error` and continue past failures
- [x] 1.2 Print each step error inline with `fmt.Printf("  Error: %v\n", err)` before continuing
- [x] 1.3 Return `errors.New("uninstall: one or more steps failed (see above)")` sentinel when any errors were collected
- [x] 1.4 Move EdgeConnect deletion outside the DynaKube `else` block so it always runs independently (`pkg/installer/kubernetes_uninstall.go`)

## 2. Usage Suppression

- [x] 2.1 Set `cmd.Root().SilenceUsage = true` in `rootCmd.PersistentPreRun` (`cmd/root.go`)
- [x] 2.2 Set `cmd.Root().SilenceUsage = true` in `installCmd.PersistentPreRun` (`cmd/install.go`)
- [x] 2.3 Set `cmd.Root().SilenceUsage = true` in `updateCmd.PersistentPreRun` (`cmd/update.go`)
- [x] 2.4 Set `cmd.Root().SilenceUsage = true` in `uninstallCmd.PersistentPreRun` (`cmd/uninstall.go`)

## 3. Duplicate Error Print

- [x] 3.1 Remove redundant `fmt.Fprintln(os.Stderr, err)` from `Execute` in `cmd/root.go`

## 4. Tests

- [x] 4.1 Update `TestUninstallKubernetes_HelmUninstallFails` to assert sentinel error message (`pkg/installer/kubernetes_uninstall_test.go`)
- [x] 4.2 Add `TestUninstallKubernetes_HelmFailContinuesToNamespaceDeletion` — helm fails, namespace deletion still runs
- [x] 4.3 Add `TestUninstallKubernetes_KubectlDeleteFailContinuesToEnd` — step 1 fails, helm and namespace steps still run
- [x] 4.4 Add `TestUninstallKubernetes_MultipleStepsFail` — multiple failures, sentinel returned, summary message printed
