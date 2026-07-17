package otel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

var venvNames = []string{".venv", "venv", "env", ".env"}

func DetectPython() (string, error) {
	for _, name := range []string{"python3", "python"} {
		for _, path := range allOnPath(name) {
			if isWindowsStorePythonStub(path) {
				logger.Debug("skipping Windows Store Python stub", "path", path)
				continue
			}
			logger.Debug("checking python interpreter on PATH", "candidate", name, "path", path)
			out, err := exec.Command(path, "--version").CombinedOutput()
			if err != nil {
				logger.Debug("python interpreter version probe failed", "candidate", name, "path", path, "error", err)
				continue
			}
			version := strings.TrimSpace(string(out))
			logger.Debug("python interpreter version probe succeeded", "candidate", name, "path", path, "version", version)
			if strings.HasPrefix(version, "Python 3") {
				logger.Debug("selected python interpreter", "candidate", name, "path", path)
				return path, nil
			}
		}
	}
	logger.Debug("no usable python 3 interpreter found on PATH")
	return "", fmt.Errorf("Python 3 interpreter not found: install Python 3 and ensure either `python3` or `python` is in PATH") //nolint:staticcheck // ST1005: keep brand capitalization
}

// allOnPath returns every executable named `name` found across all PATH
// directories, in order of appearance. On Windows the ".exe" suffix is
// appended automatically. Using os.Stat instead of exec.LookPath allows
// callers to inspect all matches (e.g. to skip Windows Store stubs). Empty
// and relative PATH entries are ignored to avoid implicit current-directory
// lookups.
func allOnPath(name string) []string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	var found []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" || !filepath.IsAbs(dir) {
			logger.Debug("skipping unsafe PATH entry during executable scan", "name", name, "dir", dir)
			continue
		}
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	logger.Debug("allOnPath scan complete", "name", name, "matches", found)
	return found
}

// isWindowsStorePythonStub reports whether the given path is the Windows Store
// Python stub (located under the WindowsApps directory). The stub exists on
// PATH by default but only opens the Microsoft Store when invoked — it is not
// a real interpreter.
func isWindowsStorePythonStub(path string) bool {
	return strings.Contains(strings.ToLower(filepath.ToSlash(path)), "windowsapps")
}

func validatePythonPrerequisites() (string, error) {
	pythonBin, err := DetectPython()
	if err != nil {
		return "", err
	}
	logger.Debug("validating python prerequisites", "python", pythonBin)
	if out, err := exec.Command(pythonBin, "-m", "pip", "--version").CombinedOutput(); err != nil {
		logger.Debug("python pip check failed", "python", pythonBin, "output", strings.TrimSpace(string(out)), "error", err)
		return "", fmt.Errorf("pip is not available for the detected Python 3 interpreter (%s): %w\n    %s", pythonBin, err, strings.TrimSpace(string(out)))
	}
	logger.Debug("python pip check succeeded", "python", pythonBin)
	if out, err := exec.Command(pythonBin, "-m", "venv", "--help").CombinedOutput(); err != nil {
		logger.Debug("python venv check failed", "python", pythonBin, "output", strings.TrimSpace(string(out)), "error", err)
		return "", fmt.Errorf("venv module is not available for the detected Python 3 interpreter (%s) — on Debian/Ubuntu run: apt install python3-venv: %w\n    %s", pythonBin, err, strings.TrimSpace(string(out)))
	}
	logger.Debug("python venv check succeeded", "python", pythonBin)
	return pythonBin, nil
}

func resolveVenvBinary(projectPath, name string) string {
	for _, venvName := range venvNames {
		binPath := filepath.Join(projectPath, venvName, "bin", name)
		if _, err := os.Stat(binPath); err == nil {
			return binPath
		}
		binPath = filepath.Join(projectPath, venvName, "Scripts", name+".exe")
		if _, err := os.Stat(binPath); err == nil {
			return binPath
		}
	}
	return name
}

func detectProjectVenvDir(projectPath string) string {
	for _, venvName := range venvNames {
		venvDir := filepath.Join(projectPath, venvName)
		info, err := os.Stat(venvDir)
		if err == nil && info.IsDir() {
			logger.Debug("found project virtualenv directory", "project", projectPath, "venv_dir", venvDir)
			return venvDir
		}
	}
	logger.Debug("no project virtualenv directory found", "project", projectPath)
	return ""
}

// detectProjectPip invokes pip via `python -m pip` using the venv's own Python binary
// to avoid shebang breakage when the venv was created with a Python that no longer exists.
func detectProjectPip(projectPath string) *pipCommand {
	for _, venvName := range venvNames {
		for _, pyName := range []string{"python", "python3"} {
			pyPath := filepath.Join(projectPath, venvName, "bin", pyName)
			if _, err := os.Stat(pyPath); err == nil {
				logger.Debug("resolved project pip command", "project", projectPath, "python", pyPath, "venv", venvName)
				return &pipCommand{name: pyPath, args: []string{"-m", "pip"}}
			}
		}
		for _, pyName := range []string{"python.exe", "python3.exe"} {
			pyPath := filepath.Join(projectPath, venvName, "Scripts", pyName)
			if _, err := os.Stat(pyPath); err == nil {
				logger.Debug("resolved project pip command", "project", projectPath, "python", pyPath, "venv", venvName)
				return &pipCommand{name: pyPath, args: []string{"-m", "pip"}}
			}
		}
	}
	logger.Debug("could not resolve project pip command", "project", projectPath)
	return nil
}

// isVenvHealthy returns true if the venv's Python binary is actually executable.
// A venv can exist but be broken if created with a Python version since removed.
func isVenvHealthy(projectPath string) bool {
	pip := detectProjectPip(projectPath)
	if pip == nil {
		logger.Debug("project virtualenv is not healthy", "project", projectPath, "reason", "no_python_binary_found")
		return false
	}
	out, err := exec.Command(pip.name, "--version").CombinedOutput()
	if err != nil {
		logger.Debug("project virtualenv health check failed", "project", projectPath, "python", pip.name, "output", strings.TrimSpace(string(out)), "error", err)
		return false
	}
	logger.Debug("project virtualenv health check succeeded", "project", projectPath, "python", pip.name, "output", strings.TrimSpace(string(out)))
	return true
}

func confirmRecreateVirtualenv(venvDir string) (bool, error) {
	prompt := fmt.Sprintf(
		"  Existing virtualenv at %s is not usable.\n  A working virtualenv is required to install Python dependencies, add instrumentation packages, and start OTLP ingest reliably.\n  Remove it and recreate it?",
		venvDir,
	)
	return installer.ConfirmProceed(prompt)
}

func removeStaleVirtualenv(venvDir string) (bool, error) {
	confirmed, err := confirmRecreateVirtualenv(venvDir)
	if err != nil {
		return false, err
	}
	if !confirmed {
		return false, nil
	}
	if err := os.RemoveAll(venvDir); err != nil {
		return false, fmt.Errorf("remove stale virtualenv %s: %w", venvDir, err)
	}
	return true, nil
}
