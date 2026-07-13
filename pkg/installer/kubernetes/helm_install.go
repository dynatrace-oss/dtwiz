package kubernetes

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// isHelmInstalled returns true when the `helm` binary is on PATH.
func isHelmInstalled() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}

// helmMajorVersion returns the major version number of the installed Helm CLI.
func helmMajorVersion() (int, error) {
	out, err := exec.Command("helm", "version", "--short").Output()
	if err != nil {
		return 0, fmt.Errorf("getting helm version: %w", err)
	}
	// Output looks like: v3.14.0+g... or v4.0.0+g...
	ver := strings.TrimSpace(string(out))
	ver = strings.TrimPrefix(ver, "v")
	parts := strings.SplitN(ver, ".", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected helm version output: %q", ver)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("parsing helm major version from %q: %w", ver, err)
	}
	return major, nil
}

// installHelm attempts to install Helm automatically.
// On Unix it runs the official get-helm-3 script; on Windows it tries winget.
// Users who require a verified installation should install Helm manually:
//
//	https://helm.sh/docs/intro/install/
func installHelm() error {
	if runtime.GOOS == "windows" {
		return installHelmWindows()
	}
	fmt.Println("  Helm not found — installing via get.helm.sh...")
	fmt.Println("  NOTE: This executes a script from https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3")
	return installer.RunCommand("bash", "-c",
		"curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash")
}

// installHelmWindows attempts to install Helm via winget. If winget is not
// available or the installation fails, it returns a clear error with manual steps.
func installHelmWindows() error {
	const manualInstructions = "\n  Install Helm manually and re-run dtwiz:\n" +
		"    winget install --id Helm.Helm\n" +
		"  or download from https://helm.sh/docs/intro/install/"

	if _, err := exec.LookPath("winget"); err != nil {
		return fmt.Errorf("helm is not installed and winget was not found on PATH%s", manualInstructions)
	}

	fmt.Println("  Helm not found — installing via winget...")
	if err := installer.RunCommand("winget", "install", "--id", "Helm.Helm", "-e", "--source", "winget"); err != nil {
		return fmt.Errorf("helm installation via winget failed: %w%s", err, manualInstructions)
	}

	// winget adds Helm to the Windows registry PATH but not to the current
	// process's PATH (which was inherited at startup). Refresh it so that
	// subsequent exec.LookPath("helm") calls can find the new binary.
	if err := installer.RefreshWindowsPath(); err != nil {
		fmt.Printf("  Warning: could not refresh PATH after Helm install: %v\n", err)
	}
	return nil
}

// isOperatorInstalled checks whether the dynatrace-operator Helm release
// exists in the dynatrace namespace.
func isOperatorInstalled() bool {
	out, err := exec.Command("helm", "list",
		"--namespace", "dynatrace",
		"--filter", "dynatrace-operator",
		"--short").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "dynatrace-operator")
}

// helmOperatorArgs builds the `helm install` argument slice.
// Helm v3 uses --atomic; Helm v4+ uses --rollback-on-failure.
// disableCSI adds --set csidriver.enabled=false, required on GKE Autopilot.
func helmOperatorArgs(helmMajor int, disableCSI bool) []string {
	rollbackFlag := "--atomic"
	if helmMajor >= 4 {
		rollbackFlag = "--rollback-on-failure"
	}
	args := []string{
		"install", "dynatrace-operator",
		dynatraceOperatorOCI,
		"--version", dynatraceOperatorVersion,
		"--create-namespace",
		"--namespace", "dynatrace",
		rollbackFlag,
		"--timeout", "10m",
	}
	if disableCSI {
		args = append(args, "--set", "csidriver.enabled=false")
	}
	return args
}

// helmOperatorUpgradeArgs builds the `helm upgrade` argument slice used when
// the dynatrace-operator release already exists.
// disableCSI adds --set csidriver.enabled=false, required on GKE Autopilot.
func helmOperatorUpgradeArgs(helmMajor int, disableCSI bool) []string {
	rollbackFlag := "--atomic"
	if helmMajor >= 4 {
		rollbackFlag = "--rollback-on-failure"
	}
	args := []string{
		"upgrade", "dynatrace-operator",
		dynatraceOperatorOCI,
		"--version", dynatraceOperatorVersion,
		"--namespace", "dynatrace",
		rollbackFlag,
		"--timeout", "10m",
	}
	if disableCSI {
		args = append(args, "--set", "csidriver.enabled=false")
	}
	return args
}
