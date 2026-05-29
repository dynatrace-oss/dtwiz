package installer

import (
	"fmt"
	"os/exec"
)

// isDockerAvailable returns true when the `docker` binary is on PATH and the
// Docker daemon is accessible.
func isDockerAvailable() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	// Quick ping to the daemon.
	return exec.Command("docker", "info", "--format", "{{.ID}}").Run() == nil
}

// InstallDocker deploys Dynatrace OneAgent as a Docker container using the
// official `dynatrace/oneagent` image, mounting the necessary host paths for
// full-stack monitoring.
//
// Parameters:
//   - envURL: Dynatrace environment URL
//   - token:  API/PaaS token with agent installation permissions
//   - dryRun: when true, only print what would be done
func InstallDocker(envURL, token string, dryRun bool) error {
	apiURL := APIURL(envURL)

	containerName := "dynatrace-oneagent"

	installerURL := fmt.Sprintf("%s/api/v1/deployment/installer/agent/unix/default/latest?Api-Token=%s", apiURL, token)

	dockerArgs := []string{
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
		fmt.Println("[dry-run] Would install Dynatrace OneAgent as a Docker container")
		fmt.Printf("  Installer URL:  %s\n", installerURL)
		fmt.Printf("  Container name: %s\n", containerName)
		fmt.Printf("  Command:        docker %v\n", dockerArgs)
		return nil
	}

	if !isDockerAvailable() {
		return fmt.Errorf("Docker is not available — install Docker and ensure the daemon is running") //nolint:staticcheck // ST1005: keep brand capitalization
	}

	// Remove any existing container with the same name.
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	fmt.Printf("  Starting Dynatrace OneAgent container %q...\n", containerName)
	if err := RunCommand("docker", dockerArgs...); err != nil {
		return fmt.Errorf("starting Dynatrace OneAgent container: %w", err)
	}

	fmt.Printf("  OneAgent container %q started successfully.\n", containerName)
	fmt.Println("  To view logs: docker logs -f " + containerName)
	return nil
}
