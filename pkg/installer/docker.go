package installer

import (
	"fmt"
	"os/exec"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
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

	dockerArgs := []string{
		"run",
		"--detach",
		"--name", containerName,
		"--pid=host",
		"--net=host",
		"--privileged",
		"--restart=always",
		"-v", "/:/mnt/root",
		"-e", fmt.Sprintf("DT_SERVER=%s/communication", apiURL),
		"-e", fmt.Sprintf("DT_TENANT=%s", ExtractTenantID(envURL)),
		"-e", fmt.Sprintf("DT_TENANT_TOKEN=%s", token),
		"dynatrace/oneagent",
	}

	if dryRun {
		fmt.Println("[dry-run] Would install Dynatrace OneAgent as a Docker container")
		fmt.Printf("  API URL:        %s\n", apiURL)
		fmt.Printf("  Container name: %s\n", containerName)
		fmt.Printf("  Command:        docker %v\n", dockerArgs)
		return nil
	}

	if !isDockerAvailable() {
		return fmt.Errorf("Docker is not available — install Docker and ensure the daemon is running") //nolint:staticcheck // ST1005: keep brand capitalization
	}

	// Remove any existing container with the same name.
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	display.InstallHeader("dtwiz · OneAgent (Docker)")
	err := display.RunStep("Started OneAgent container", func(s *display.Step) error {
		s.Detail("%s", containerName)
		if err := RunCommandQuiet("docker", dockerArgs...); err != nil {
			return fmt.Errorf("starting Dynatrace OneAgent container: %w", err)
		}
		return nil
	})
	RecordFeature(FeatureHostMonitoring, err == nil)
	if err != nil {
		return err
	}

	display.Println("View logs: docker logs -f %s", containerName)
	return nil
}
