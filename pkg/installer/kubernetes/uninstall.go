package kubernetes

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var runCmdQuietFunc = installer.RunCommandQuiet

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
	display.Header("dtwiz · Kubernetes uninstall")

	if kubeCtx != "" {
		display.PrintAlignedStatusLines(display.ColorDefault, "Cluster", fmt.Sprintf("%s · context=%s", distro, kubeCtx))
	}

	if dryRun {
		handleK8sUninstallDryRun()
		return nil
	}

	if err := installer.RefreshWindowsPath(); err != nil {
		display.StepWarn("Could not refresh PATH", err.Error())
	}

	display.PrintAlignedStatusLines(display.ColorDefault, "Removes", "DynaKube resources, Dynatrace Operator (helm), namespace dynatrace")
	if logger.Enabled() {
		fmt.Println()
		printK8sUninstallSteps()
	}
	fmt.Println()

	ok, err := installer.ConfirmProceed("  Apply?")
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
	if err := display.RunStep("Deleted DynaKube and EdgeConnect resources", func(*display.Step) error {
		display.Detail("kubectl delete dynakube -n %s --all", dtNamespace)
		err := runCmdQuietFunc("kubectl", "delete", "dynakube", "-n", dtNamespace, "--all")
		// EdgeConnect may not exist — ignore failure. Always attempt regardless of DynaKube result.
		_ = runCmdQuietFunc("kubectl", "delete", "edgeconnect", "-n", dtNamespace, "--all")
		return err
	}); err != nil {
		errs = append(errs, fmt.Errorf("deleting DynaKube resources: %w", err))
	}

	// 2. Wait for managed pods to terminate.
	_ = display.RunStep("Waited for managed pods to terminate", func(s *display.Step) error {
		s.Detail("up to 5 min")
		waitErr := runCmdQuietFunc(
			"kubectl", "-n", dtNamespace, "wait", "pod",
			"--for=delete",
			"-l", "app.kubernetes.io/managed-by=dynatrace-operator",
			"--timeout=300s",
		)
		if waitErr != nil {
			// Non-fatal: pods may already be gone or label may not match.
			logger.Debug("waiting for managed pods failed", "error", waitErr)
			s.Detail("not confirmed: %s", firstLine(waitErr.Error()))
			return nil
		}
		return nil
	})

	// 3. Helm uninstall.
	if err := display.RunStep("Uninstalled Dynatrace Operator", func(*display.Step) error {
		display.Detail("helm uninstall dynatrace-operator -n %s", dtNamespace)
		return runCmdQuietFunc("helm", "uninstall", "dynatrace-operator", "-n", dtNamespace)
	}); err != nil {
		errs = append(errs, fmt.Errorf("helm uninstall failed: %w", err))
	}

	// 4. Delete namespace.
	if err := display.RunStep("Deleted namespace "+dtNamespace, func(*display.Step) error {
		return runCmdQuietFunc("kubectl", "delete", "namespace", dtNamespace)
	}); err != nil {
		errs = append(errs, fmt.Errorf("deleting namespace: %w", err))
	}

	if len(errs) > 0 {
		return errors.New("uninstall: one or more steps failed (see above)")
	}
	return nil
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return s
}
