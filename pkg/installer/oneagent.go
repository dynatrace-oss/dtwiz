package installer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// InstallMode controls which OneAgent components are installed.
type InstallMode string

const (
	InstallModeFullStack InstallMode = "fullstack"
	InstallModeInfraOnly InstallMode = "infra"
	InstallModeDiscovery InstallMode = "discovery"
)

// installerType maps to the Dynatrace installer API "type" path segment.
type installerType string

const (
	installerTypeUnix    installerType = "unix"
	installerTypeWindows installerType = "windows"
)

// oneAgentInstallerType returns the installer type string for the current
// OS/architecture combination, to be used in the download URL path.
func oneAgentInstallerType() (installerType, string, error) {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return installerTypeUnix, "x86", nil
		case "arm64":
			return installerTypeUnix, "arm", nil
		default:
			return "", "", fmt.Errorf("unsupported Linux architecture: %s", runtime.GOARCH)
		}
	case "darwin":
		return "", "", fmt.Errorf("OneAgent direct install is not supported on macOS; use Docker or Linux")
	case "windows":
		return installerTypeWindows, "x86", nil
	default:
		return "", "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// oneAgentInstallerFilename returns an appropriate local filename for the
// downloaded installer based on OS.
func oneAgentInstallerFilename() string {
	if runtime.GOOS == "windows" {
		return "dynatrace-oneagent-installer.exe"
	}
	return "dynatrace-oneagent-installer.sh"
}

// checkOneAgentConnectivity performs a quick connectivity check against the
// Dynatrace API endpoint.
func checkOneAgentConnectivity(apiURL, token string) error {
	logger.Debug("checking OneAgent connectivity", "url", apiURL)
	req, err := http.NewRequest(http.MethodGet, apiURL+"/api/v1/time", nil)
	if err != nil {
		return fmt.Errorf("building connectivity request: %w", err)
	}
	req.Header.Set("Authorization", AuthHeader(token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connectivity check failed — cannot reach %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("connectivity check failed: invalid credentials (401 Unauthorized)")
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("connectivity check returned unexpected status %d", resp.StatusCode)
	}
	logger.Debug("connectivity check passed", "status", resp.StatusCode)
	return nil
}

// downloadOneAgentInstaller downloads the OneAgent installer binary to a
// temporary file and returns its path.
func downloadOneAgentInstaller(apiURL, token string) (string, error) {
	iType, arch, err := oneAgentInstallerType()
	if err != nil {
		return "", err
	}

	// API: GET /api/v1/deployment/installer/agent/{osType}/default/latest?arch={arch}
	downloadURL := fmt.Sprintf(
		"%s/api/v1/deployment/installer/agent/%s/default/latest?arch=%s",
		apiURL, iType, arch,
	)
	logger.Debug("downloading OneAgent installer", "url", downloadURL, "os", runtime.GOOS, "arch", arch)

	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("building download request: %w", err)
	}
	req.Header.Set("Authorization", AuthHeader(token))

	fmt.Printf("  Downloading OneAgent installer from %s...\n", apiURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading OneAgent installer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("installer download failed with status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", oneAgentInstallerFilename())
	if err != nil {
		return "", fmt.Errorf("creating temp file for installer: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("writing installer to disk: %w", err)
	}

	// Make the installer executable on Unix systems.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpFile.Name(), 0o755); err != nil {
			os.Remove(tmpFile.Name())
			return "", fmt.Errorf("setting installer executable bit: %w", err)
		}
	}

	logger.Debug("installer downloaded", "path", tmpFile.Name())
	return tmpFile.Name(), nil
}

// InstallOneAgent installs Dynatrace OneAgent on the current host.
//
// Parameters:
//   - envURL: Dynatrace environment URL (apps or live)
//   - token:  API token with installation permissions
//   - dryRun:    when true, only print what would be done
//   - quiet:     when true, suppress output and skip confirmation
//   - hostGroup: optional host group name (passed as --set-host-group)
func InstallOneAgent(envURL, token string, dryRun, quiet bool, hostGroup string) error {
	apiURL := APIURL(envURL)
	logger.Debug("installing OneAgent", "env_url", envURL, "api_url", apiURL, "dry_run", dryRun, "quiet", quiet, "host_group", hostGroup)

	if dryRun {
		iType, arch, _ := oneAgentInstallerType()
		fmt.Println("[dry-run] Would install Dynatrace OneAgent")
		fmt.Printf("  API URL:          %s\n", apiURL)
		fmt.Printf("  Installer type:   %s / arch: %s\n", iType, arch)
		fmt.Printf("  Install mode:     %s\n", InstallModeFullStack)
		if quiet {
			fmt.Println("  Quiet mode:       yes")
		}
		if hostGroup != "" {
			fmt.Printf("  Host group:       %s\n", hostGroup)
		}
		return nil
	}

	if !quiet {
		fmt.Println("  Checking API connectivity...")
	}
	if err := checkOneAgentConnectivity(apiURL, token); err != nil {
		return err
	}

	installerPath, err := downloadOneAgentInstaller(apiURL, token)
	if err != nil {
		return err
	}
	defer os.Remove(installerPath)

	args := buildOneAgentInstallerArgs(installerPath, apiURL, quiet, hostGroup)
	logger.Debug("running installer", "cmd", args[0], "args", args[1:])

	if quiet {
		if err := RunCommandQuiet(args[0], args[1:]...); err != nil {
			return fmt.Errorf("OneAgent installation failed: %w", err)
		}
	} else {
		fmt.Printf("  Running installer: %s\n", filepath.Base(installerPath))
		if err := RunCommand(args[0], args[1:]...); err != nil {
			return fmt.Errorf("OneAgent installation failed: %w", err)
		}
		fmt.Println("  OneAgent installed successfully.")
	}

	return nil
}

// buildOneAgentInstallerArgs returns the command and arguments to run the
// downloaded installer, varying by OS. When quiet is true, the native quiet
// flags are appended so the installer produces no interactive output.
func buildOneAgentInstallerArgs(installerPath, apiURL string, quiet bool, hostGroup string) []string {
	if runtime.GOOS == "windows" {
		// Windows exe installer: parameters are passed directly as flags.
		// --quiet MUST be the first argument so the self-extracting exe
		// suppresses the GUI before processing any other flags.
		args := []string{installerPath}
		if quiet {
			args = append(args, "--quiet")
		}
		args = append(args, "--set-app-log-content-access=true")
		if hostGroup != "" {
			args = append(args, fmt.Sprintf("--set-host-group=%s", hostGroup))
		}
		return args
	}

	// Linux shell installer — needs sudo if not already root.
	args := []string{
		installerPath,
		fmt.Sprintf("--set-server=%s", strings.TrimRight(apiURL, "/")),
		"--set-app-log-content-access=true",
	}
	if hostGroup != "" {
		args = append(args, fmt.Sprintf("--set-host-group=%s", hostGroup))
	}

	if needsSudo() {
		return append([]string{"sudo"}, args...)
	}
	return args
}
