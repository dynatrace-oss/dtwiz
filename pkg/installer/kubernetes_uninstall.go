package installer

import (
	"errors"
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
)

var runCmdQuietFunc = RunCommandQuiet

// UninstallKubernetes removes the Dynatrace Operator and all managed resources
// from the cluster following the official uninstallation sequence:
//  1. Delete all DynaKube and EdgeConnect CRs
//  2. Wait for managed pods to terminate (up to 5 min)
//  3. Helm uninstall dynatrace-operator
//  4. Delete the dynatrace namespace
func UninstallKubernetes(kubeCtx, distro string) error {
	if err := refreshWindowsPath(); err != nil {
		fmt.Printf("  Warning: could not refresh PATH: %v\n", err)
	}

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Kubernetes Uninstall")
	fmt.Println()

	if kubeCtx != "" {
		fmt.Printf("  The affected cluster is: %s context=%s\n", distro, kubeCtx)
		fmt.Println()
	}

	fmt.Println("  This will perform the following steps:")
	fmt.Println("    1. Delete all DynaKube and EdgeConnect custom resources")
	fmt.Println("    2. Wait for managed pods to terminate (up to 5 min)")
	fmt.Println("    3. Helm uninstall dynatrace-operator")
	fmt.Println("    4. Delete the dynatrace namespace")
	fmt.Println()

	ok, err := confirmProceed("  Proceed with uninstall?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Uninstall cancelled.")
		return nil
	}
	fmt.Println()

	var errs []error

	// 1. Delete DynaKube and EdgeConnect CRs.
	fmt.Println("  Step 1: Deleting DynaKube and EdgeConnect custom resources...")
	if err := runCmdQuietFunc("kubectl", "delete", "dynakube", "-n", "dynatrace", "--all"); err != nil {
		fmt.Printf("  Error: %v\n", err)
		errs = append(errs, fmt.Errorf("deleting DynaKube resources: %w", err))
	}
	// EdgeConnect may not exist — ignore failure. Always attempt regardless of DynaKube result.
	_ = runCmdQuietFunc("kubectl", "delete", "edgeconnect", "-n", "dynatrace", "--all")
	if len(errs) == 0 {
		fmt.Println("  Custom resources deleted.")
	}

	// 2. Wait for managed pods to terminate.
	fmt.Println("  Step 2: Waiting for managed pods to terminate (up to 5 min)...")
	waitErr := runCmdQuietFunc(
		"kubectl", "-n", "dynatrace", "wait", "pod",
		"--for=delete",
		"-l", "app.kubernetes.io/managed-by=dynatrace-operator",
		"--timeout=300s",
	)
	if waitErr != nil {
		// Non-fatal: pods may already be gone or label may not match.
		fmt.Printf("  Warning: %v\n", waitErr)
	} else {
		fmt.Println("  Managed pods terminated.")
	}

	// 3. Helm uninstall.
	fmt.Println("  Step 3: Helm uninstall dynatrace-operator...")
	if err := runCmdQuietFunc("helm", "uninstall", "dynatrace-operator", "-n", "dynatrace"); err != nil {
		fmt.Printf("  Error: %v\n", err)
		errs = append(errs, fmt.Errorf("helm uninstall failed: %w", err))
	} else {
		fmt.Println("  Dynatrace Operator uninstalled.")
	}

	// 4. Delete namespace.
	fmt.Println("  Step 4: Deleting dynatrace namespace...")
	if err := runCmdQuietFunc("kubectl", "delete", "namespace", "dynatrace"); err != nil {
		fmt.Printf("  Error: %v\n", err)
		errs = append(errs, fmt.Errorf("deleting namespace: %w", err))
	} else {
		fmt.Println("  Namespace deleted.")
	}

	fmt.Println()
	if len(errs) > 0 {
		fmt.Println("  Uninstall completed with errors.")
		return errors.New("uninstall: one or more steps failed (see above)")
	}
	fmt.Println("  Dynatrace Operator uninstalled successfully.")
	return nil
}
