package installer

import (
	"fmt"
	"os/exec"
)

// isPodmanAvailable returns true when the `podman` binary is on PATH and the
// Podman daemon is accessible.
func isPodmanAvailable() bool {
	if _, err := exec.LookPath("podman"); err != nil {
		return false
	}
	return exec.Command("podman", "info", "--format", "{{.Host.Hostname}}").Run() == nil
}

// InstallPodman deploys Dynatrace OneAgent as a Podman container using the
// official `dynatrace/oneagent` image, mounting the necessary host paths for
// full-stack monitoring.
func InstallPodman(envURL, token string, dryRun bool) error {
	apiURL := APIURL(envURL)

	containerName := "dynatrace-oneagent"

	installerURL := fmt.Sprintf("%s/api/v1/deployment/installer/agent/unix/default/latest?Api-Token=%s", apiURL, token)

	podmanArgs := []string{
		"run",
		"--detach",
		"--name", containerName,
		"--pid=host",
		"--net=host",
		"--privileged",
		"--restart=always",
		"-v", "/:/mnt/root",
		"-e", fmt.Sprintf("ONEAGENT_INSTALLER_SCRIPT_URL=%s", installerURL),
		"dynatrace/oneagent",
	}

	if dryRun {
		fmt.Println("[dry-run] Would install Dynatrace OneAgent as a Podman container")
		fmt.Printf("  Installer URL:  %s\n", installerURL)
		fmt.Printf("  Container name: %s\n", containerName)
		fmt.Printf("  Command:        podman %v\n", podmanArgs)
		return nil
	}

	if !isPodmanAvailable() {
		return fmt.Errorf("Podman is not available — install Podman and ensure the daemon is running") //nolint:staticcheck // ST1005: keep brand capitalization
	}

	// Remove any existing container with the same name.
	_ = exec.Command("podman", "rm", "-f", containerName).Run()

	fmt.Printf("  Starting Dynatrace OneAgent container %q...\n", containerName)
	if err := RunCommand("podman", podmanArgs...); err != nil {
		return fmt.Errorf("starting Dynatrace OneAgent container: %w", err)
	}

	fmt.Printf("  OneAgent container %q started successfully.\n", containerName)
	fmt.Println("  To view logs: podman logs -f " + containerName)
	return nil
}
