package installer

import (
	"errors"
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
)

var runCmdQuietFunc = RunCommandQuiet

func printK8sUninstallSteps() {
	display.PrintSteps("This will perform the following steps:",
		"Delete all DynaKube and EdgeConnect custom resources",
		"Wait for managed pods to terminate (up to 5 min)",
		"Helm uninstall dynatrace-operator",
		"Delete the dynatrace namespace",
	)
}

// handleK8sUninstallDryRun prints what the uninstallation would do without executing anything.
func handleK8sUninstallDryRun() {
	display.Println("[dry-run] Would remove Dynatrace Operator from Kubernetes")
	fmt.Println()
	printK8sUninstallSteps()
}

// UninstallKubernetes removes the Dynatrace Operator and all managed resources
// from the cluster following the official uninstallation sequence:
//  1. Delete all DynaKube and EdgeConnect CRs
//  2. Wait for managed pods to terminate (up to 5 min)
//  3. Helm uninstall dynatrace-operator
//  4. Delete the dynatrace namespace
func UninstallKubernetes(kubeCtx, distro string, dryRun bool) error {
	fmt.Println()
	display.PrintlnColored(display.ColorMessage, "Dynatrace Kubernetes Uninstall")
	fmt.Println()

	if kubeCtx != "" {
		display.Println("The affected cluster is: %s context=%s\n", distro, kubeCtx)
	}

	if dryRun {
		handleK8sUninstallDryRun()
		return nil
	}

	if err := refreshWindowsPath(); err != nil {
		display.Println("Warning: could not refresh PATH: %v", err)
	}

	printK8sUninstallSteps()
	fmt.Println()

	ok, err := confirmProceed("  Proceed with uninstall?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		display.Println("Uninstall cancelled.")
		return nil
	}
	fmt.Println()

	const dtNamespace = "dynatrace"
	var errs []error

	// 1. Delete DynaKube and EdgeConnect CRs.
	display.Println("Step 1: Deleting DynaKube and EdgeConnect custom resources...")
	if err := runCmdQuietFunc("kubectl", "delete", "dynakube", "-n", dtNamespace, "--all"); err != nil {
		display.Println("Error: %v", err)
		errs = append(errs, fmt.Errorf("deleting DynaKube resources: %w", err))
	}
	// EdgeConnect may not exist — ignore failure. Always attempt regardless of DynaKube result.
	_ = runCmdQuietFunc("kubectl", "delete", "edgeconnect", "-n", dtNamespace, "--all")
	if len(errs) == 0 {
		display.Println("Custom resources deleted.")
	}

	// 2. Wait for managed pods to terminate.
	display.Println("Step 2: Waiting for managed pods to terminate (up to 5 min)...")
	waitErr := runCmdQuietFunc(
		"kubectl", "-n", dtNamespace, "wait", "pod",
		"--for=delete",
		"-l", "app.kubernetes.io/managed-by=dynatrace-operator",
		"--timeout=300s",
	)
	if waitErr != nil {
		// Non-fatal: pods may already be gone or label may not match.
		display.Println("Warning: %v", waitErr)
	} else {
		display.Println("Managed pods terminated.")
	}

	// 3. Helm uninstall.
	display.Println("Step 3: Helm uninstall dynatrace-operator...")
	if err := runCmdQuietFunc("helm", "uninstall", "dynatrace-operator", "-n", dtNamespace); err != nil {
		display.Println("Error: %v", err)
		errs = append(errs, fmt.Errorf("helm uninstall failed: %w", err))
	} else {
		display.Println("Dynatrace Operator uninstalled.")
	}

	// 4. Delete namespace.
	display.Println("Step 4: Deleting dynatrace namespace...")
	if err := runCmdQuietFunc("kubectl", "delete", "namespace", dtNamespace); err != nil {
		display.Println("Error: %v", err)
		errs = append(errs, fmt.Errorf("deleting namespace: %w", err))
	} else {
		display.Println("Namespace deleted.")
	}

	fmt.Println()
	if len(errs) > 0 {
		display.Println("Uninstall completed with errors.")
		return errors.New("uninstall: one or more steps failed (see above)")
	}
	display.Println("Dynatrace Operator uninstalled successfully.")
	return nil
}
