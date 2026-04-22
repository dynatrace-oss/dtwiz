package installer

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
)

// UninstallKubernetes removes the Dynatrace Operator and all managed resources
// from the cluster following the official uninstall sequence:
//  1. Delete all DynaKube and EdgeConnect CRs
//  2. Wait for managed pods to terminate (up to 5 min)
//  3. Helm uninstall dynatrace-operator
//  4. Delete the dynatrace namespace
func UninstallKubernetes() error {
	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Kubernetes Uninstall")
	fmt.Println()
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

	// 1. Delete DynaKube and EdgeConnect CRs.
	fmt.Println("  Step 1: Deleting DynaKube and EdgeConnect custom resources...")
	if err := RunCommandQuiet("kubectl", "delete", "dynakube", "-n", "dynatrace", "--all"); err != nil {
		return fmt.Errorf("deleting DynaKube resources: %w", err)
	}
	// EdgeConnect may not exist — ignore failure.
	_ = RunCommandQuiet("kubectl", "delete", "edgeconnect", "-n", "dynatrace", "--all")
	fmt.Println("  Custom resources deleted.")

	// 2. Wait for managed pods to terminate.
	fmt.Println("  Step 2: Waiting for managed pods to terminate (up to 5 min)...")
	waitErr := RunCommandQuiet(
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
	if err := RunCommandQuiet("helm", "uninstall", "dynatrace-operator", "-n", "dynatrace"); err != nil {
		return fmt.Errorf("helm uninstall failed: %w", err)
	}
	fmt.Println("  Dynatrace Operator uninstalled.")

	// 4. Delete namespace.
	fmt.Println("  Step 4: Deleting dynatrace namespace...")
	if err := RunCommandQuiet("kubectl", "delete", "namespace", "dynatrace"); err != nil {
		return fmt.Errorf("deleting namespace: %w", err)
	}
	fmt.Println("  Namespace deleted.")

	fmt.Println()
	fmt.Println("  Dynatrace Operator uninstalled successfully.")
	return nil
}
