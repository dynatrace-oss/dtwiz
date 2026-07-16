package otel

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	demoZipURL          = "https://github.com/dietermayrhofer/schnitzel/archive/refs/heads/master.zip"
	demoDirName         = "schnitzel"
	wingetPythonPackage = "Python.Python.3.14"
)

func checkDemoExists() bool {
	_, err := os.Stat(demoDirName)
	return err == nil
}

// pythonInstallPlan returns the command (name + args) needed to install Python 3
// on the current platform, or an error if installation cannot be automated.
// Returns nil, nil if Python is already present.
func pythonInstallPlan() ([]string, error) {
	// On Windows the process may have a stale PATH (e.g. Python was installed by
	// a previous dtwiz run in the same terminal session). Refresh before checking.
	if runtime.GOOS == "windows" {
		if err := installer.RefreshWindowsPath(); err != nil {
			logger.Debug("pythonInstallPlan: RefreshWindowsPath failed", "error", err)
		} else {
			logger.Debug("pythonInstallPlan: PATH refreshed from registry")
		}
	}
	if _, err := DetectPython(); err == nil {
		logger.Debug("pythonInstallPlan: Python already available, skipping install")
		return nil, nil // already available
	}

	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return nil, fmt.Errorf("Python 3 is required but not found.\nInstall Homebrew first: https://brew.sh, then re-run this command") //nolint:staticcheck // ST1005: keep brand capitalization
		}
		return []string{"brew", "install", "python3"}, nil

	case "linux":
		distro := detectLinuxDistro()
		switch distro {
		case "debian", "ubuntu":
			return []string{"sudo", "apt-get", "install", "-y", "python3"}, nil
		default:
			// RHEL/Fedora/CentOS/Rocky/Alma
			return []string{"sudo", "dnf", "install", "-y", "python3"}, nil
		}

	case "windows":
		return []string{"winget", "install", "--id", wingetPythonPackage}, nil

	default:
		return nil, fmt.Errorf("Python 3 is required but not found; please install it manually") //nolint:staticcheck // ST1005: keep brand capitalization
	}
}

// installPythonWindows installs Python 3 on Windows via winget. The exit code
// is ignored — winget returns non-zero for already-installed packages; DetectPython
// is the true success signal.
func installPythonWindows() error {
	const manualPythonInstructions = "install Python manually from https://www.python.org/downloads/"

	if _, err := exec.LookPath("winget"); err != nil {
		return fmt.Errorf("winget was not found on PATH; install winget or %s", manualPythonInstructions)
	}

	logger.Debug("installPythonWindows: installing", "id", wingetPythonPackage)
	const includeWingetOutput = true
	_, wingetErr := installer.RunCommandWithExitCode([]string{"winget", "install", "--id", wingetPythonPackage,
		"--source", "winget",
		"--accept-package-agreements", "--accept-source-agreements"}, includeWingetOutput)
	if wingetErr != nil {
		logger.Debug("installPythonWindows: winget install failed", "error", wingetErr)
	} else {
		logger.Debug("installPythonWindows: winget install completed")
	}
	if refreshErr := installer.RefreshWindowsPath(); refreshErr != nil {
		logger.Debug("installPythonWindows: RefreshWindowsPath failed", "error", refreshErr)
		fmt.Printf("  Warning: could not refresh PATH: %v\n", refreshErr)
	} else {
		logger.Debug("installPythonWindows: PATH refreshed from registry")
	}
	if _, err := DetectPython(); err == nil {
		return nil
	}
	if wingetErr != nil {
		return fmt.Errorf("could not install Python 3 via winget: %w; %s", wingetErr, manualPythonInstructions) //nolint:staticcheck // ST1005: keep brand capitalization
	}
	return fmt.Errorf("could not install Python 3 via winget; %s", manualPythonInstructions) //nolint:staticcheck // ST1005: keep brand capitalization
}

func detectLinuxDistro() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "rhel"
	}
	content := strings.ToLower(string(data))
	if strings.Contains(content, "ubuntu") {
		return "ubuntu"
	}
	if strings.Contains(content, "debian") {
		return "debian"
	}
	return "rhel"
}

// downloadAndExtractDemo downloads the schnitzel ZIP and extracts it atomically
// (to a temp dir first, then renames) to ./schnitzel/.
func downloadAndExtractDemo() error {
	resp, err := http.Get(demoZipURL)
	if err != nil {
		return fmt.Errorf("downloading demo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading demo: unexpected HTTP %d from %s", resp.StatusCode, demoZipURL)
	}

	tmpFile, err := os.CreateTemp("", "schnitzel-*.zip")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing zip: %w", err)
	}
	tmpFile.Close()

	tmpDir, err := os.MkdirTemp("", "schnitzel-extract-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractZip(tmpFile.Name(), tmpDir); err != nil {
		return fmt.Errorf("extracting zip: %w", err)
	}

	// Find the extracted top-level dir (e.g. schnitzel-master)
	entries, err := os.ReadDir(tmpDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("unexpected zip structure: no top-level directory found")
	}
	extracted := filepath.Join(tmpDir, entries[0].Name())

	if err := os.Rename(extracted, demoDirName); err != nil {
		return fmt.Errorf("moving demo to %s: %w", demoDirName, err)
	}
	return nil
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(destDir, f.Name) //nolint:gosec
		// Prevent zip-slip
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc) //nolint:gosec
		rc.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

// InstallDemo orchestrates the schnitzel demo installation:
// 1. Download & extract schnitzel (if not already present)
// 2. Install Python if missing
// 3. Install OTel Collector + Python auto-instrumentation targeting ./schnitzel
func InstallDemo(envURL, token, platformTok string, dryRun bool) error {
	demoExists := checkDemoExists()
	pythonCmd, err := pythonInstallPlan()
	if err != nil {
		return err
	}

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Demo Installation (schnitzel)")
	fmt.Println()
	fmt.Println("  This will:")

	step := 1
	if !demoExists {
		fmt.Printf("  %d) Download schnitzel demo app to ./%s/\n", step, demoDirName)
		step++
	}
	if pythonCmd != nil {
		fmt.Printf("  %d) Install Python 3 via %s\n", step, pythonCmd[0])
		step++
	}
	fmt.Printf("  %d) Install OTel Collector\n", step)
	step++
	fmt.Printf("  %d) Auto-instrument the schnitzel Python app\n", step)
	fmt.Println()

	if dryRun {
		fmt.Println("  [dry-run] No changes will be made.")
		return nil
	}

	ok, err := installer.ConfirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	// Step 1: Download demo
	if !demoExists {
		fmt.Printf("  Downloading schnitzel demo app...\n")
		if err := downloadAndExtractDemo(); err != nil {
			return err
		}
		fmt.Printf("  Demo extracted to ./%s/\n", demoDirName)
	} else {
		fmt.Printf("  Demo directory ./%s/ already exists, skipping download.\n", demoDirName)
	}

	// Step 2: Install Python if needed
	if pythonCmd != nil {
		fmt.Printf("  Installing Python 3 via %s...\n", pythonCmd[0])
		var installErr error
		if runtime.GOOS == "windows" {
			installErr = installPythonWindows()
		} else {
			installErr = installer.RunCommand(pythonCmd[0], pythonCmd[1:]...)
		}
		if installErr != nil {
			return fmt.Errorf("Python installation failed: %w", installErr) //nolint:staticcheck // ST1005: keep brand capitalization
		}
		fmt.Println("  Python 3 installed.")
	}

	// Step 3+4: OTel Collector + Python instrumentation
	absDemoDir, err := filepath.Abs(demoDirName)
	if err != nil {
		return fmt.Errorf("resolving demo directory path: %w", err)
	}
	installer.AutoConfirm = true
	return InstallOtelCollectorWithProject(envURL, token, platformTok, absDemoDir, dryRun)
}
