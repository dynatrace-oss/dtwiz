package otel

import (
	"archive/tar"
	"compress/gzip"
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
	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

const (
	demoDirName         = "schnitzel"
	wingetPythonPackage = "Python.Python.3.14"
)

// releaseAssetBaseURL is the base URL for dtwiz release assets.
// Overridden in tests to point at a local httptest.Server.
var releaseAssetBaseURL = "https://github.com/dynatrace-oss/dtwiz/releases/download"

// BundledDemoPath returns the fixed extraction path for the schnitzel demo app:
// $HOME/.dtwiz/examples/schnitzel on macOS/Linux, %USERPROFILE%\.dtwiz\examples\schnitzel on Windows.
func BundledDemoPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dtwiz", "examples", demoDirName)
}

// downloadDemoExamples downloads the version-pinned dtwiz-examples.tar.gz release
// asset and extracts it so that schnitzel ends up at dst.
func downloadDemoExamples(dst string) error {
	ver := version.Version
	url := fmt.Sprintf("%s/v%s/dtwiz-examples.tar.gz", releaseAssetBaseURL, ver)

	resp, err := http.Get(url) //nolint:gosec // URL constructed from hardcoded base + binary version
	if err != nil {
		return fmt.Errorf("downloading demo examples: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading demo examples: HTTP %d from %s", resp.StatusCode, url)
	}

	// Tarball contains examples/schnitzel/...; extract to ~/.dtwiz/ so
	// ~/.dtwiz/examples/schnitzel/ == dst.
	extractTo := filepath.Dir(filepath.Dir(dst))
	if err := os.MkdirAll(extractTo, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", extractTo, err)
	}
	return extractTarGz(resp.Body, extractTo)
}

func extractTarGz(r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("opening gzip stream: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		target := filepath.Join(destDir, filepath.FromSlash(hdr.Name)) //nolint:gosec
		// Prevent path traversal.
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator),
			filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

// pythonInstallPlan returns the command (name + args) needed to install Python 3
// and its prerequisites (pip, venv) on the current platform, or an error if
// installation cannot be automated. Returns nil, nil if all prerequisites are
// already present.
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
	pythonBin, err := DetectPython()
	if err == nil {
		// On Windows and macOS, Python bundles pip — presence of the interpreter is enough.
		// On Linux, pip and venv are separate packages that may be missing even when
		// python3 is installed, so we must probe them explicitly.
		if runtime.GOOS != "linux" {
			logger.Debug("pythonInstallPlan: Python already available, skipping install")
			return nil, nil
		}
		_, pipErr := exec.Command(pythonBin, "-m", "pip", "--version").CombinedOutput()
		if pipErr == nil && probeVenvPip(pythonBin) {
			return nil, nil
		}
	}

	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return nil, fmt.Errorf("Python 3 is required but not found.\nInstall Homebrew first: https://brew.sh, then re-run this command") //nolint:staticcheck // ST1005: keep brand capitalization
		}
		return []string{"brew", "install", "python3"}, nil

	case "linux":
		switch detectLinuxDistro() {
		case "debian", "ubuntu":
			return []string{"sudo", "apt-get", "install", "-y", "python3", "python3-pip", "python3-venv"}, nil
		default:
			// RHEL/Fedora/CentOS/Rocky/Alma
			return []string{"sudo", "dnf", "install", "-y", "python3", "python3-pip", "python3-venv"}, nil
		}

	case "windows":
		return []string{"winget", "install", "--id", wingetPythonPackage}, nil

	default:
		return nil, fmt.Errorf("Python 3 is required but not found; please install it manually") //nolint:staticcheck // ST1005: keep brand capitalization
	}
}

// describeDemoInstallCmd returns a short human-readable label for a demoInstallCmd result,
// e.g. "python3, python3-pip, python3-venv via apt-get".
func describeDemoInstallCmd(cmd []string) string {
	for i, part := range cmd {
		switch part {
		case "brew":
			return "python3 via brew"
		case "apt-get", "dnf":
			var pkgs []string
			for _, p := range cmd[i+1:] {
				if p != "install" && p != "-y" {
					pkgs = append(pkgs, p)
				}
			}
			return strings.Join(pkgs, ", ") + " via " + part
		case "winget":
			return "Python 3 via winget"
		}
	}
	return strings.Join(cmd, " ")
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

// IsDemoRunning returns true when the schnitzel demo services are already running.
func IsDemoRunning() bool {
	demoPath := BundledDemoPath()
	if _, err := os.Stat(demoPath); err != nil {
		return false
	}
	running := len(matchingProcessIDs(demoPath, detectPythonProcesses())) > 0
	if running {
		logger.Debug("IsDemoRunning: demo already running, not showing demo")
	}
	return running
}

// InstallDemo orchestrates the schnitzel demo installation:
// 1. Download schnitzel from dtwiz release asset (if not already present)
// 2. Install Python if missing
// 3. Install OTel Collector + Python auto-instrumentation targeting the bundled schnitzel app
func InstallDemo(envURL, token, platformTok string, dryRun bool) error {
	demoPath := BundledDemoPath()
	_, statErr := os.Stat(demoPath)
	demoExists := statErr == nil

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
		fmt.Printf("  %d) Download schnitzel from dtwiz release asset\n", step)
		step++
	}
	if pythonCmd != nil {
		fmt.Printf("  %d) Install %s\n", step, describeDemoInstallCmd(pythonCmd))
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

	if !demoExists {
		fmt.Println("  Downloading schnitzel demo app...")
		if err := downloadDemoExamples(demoPath); err != nil {
			return err
		}
		fmt.Println("  Demo downloaded.")
	}

	// Step 2: Install missing Python prerequisites if needed
	if pythonCmd != nil {
		fmt.Printf("  Installing %s...\n", describeDemoInstallCmd(pythonCmd))
		var installErr error
		if runtime.GOOS == "windows" {
			installErr = installPythonWindows()
		} else {
			installErr = installer.RunCommand(pythonCmd[0], pythonCmd[1:]...)
		}
		if installErr != nil {
			return fmt.Errorf("Python installation failed: %w", installErr) //nolint:staticcheck // ST1005: keep brand capitalization
		}
		fmt.Println("  Python dependencies installed.")
	}

	installer.AutoConfirm = true
	return InstallOtelCollectorWithProject(envURL, token, platformTok, demoPath, dryRun)
}
