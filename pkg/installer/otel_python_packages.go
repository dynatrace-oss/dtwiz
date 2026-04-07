package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// pipCommand holds the resolved pip executable and arguments.
type pipCommand struct {
	name string
	args []string
}

// otelPythonPackages is the list of OpenTelemetry packages to install for
// auto-instrumentation, following the Dynatrace documentation.
var otelPythonPackages = []string{
	"opentelemetry-distro",
	"opentelemetry-exporter-otlp",
}

// installPackages installs the given pip packages using the resolved pip command.
// Output is suppressed unless the command fails.
func installPackages(pip *pipCommand, packages []string) error {
	args := append(append([]string{}, pip.args...), append([]string{"install"}, packages...)...)
	cmd := exec.Command(pip.name, args...)
	full := pip.name + " " + strings.Join(args, " ")
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stdout.Write(out)
		return fmt.Errorf("pip install failed: %w\n    command: %s", err, full)
	}
	return nil
}

// runOtelBootstrap runs `opentelemetry-bootstrap -a install` to automatically
// install instrumentation libraries for all packages found in the environment.
// Output is suppressed unless the command fails.
func runOtelBootstrap(pythonPath string) error {
	args := []string{"-m", "opentelemetry.instrumentation.bootstrap", "-a", "install"}
	cmd := exec.Command(pythonPath, args...)
	full := pythonPath + " " + strings.Join(args, " ")
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stdout.Write(out)
		return fmt.Errorf("opentelemetry-bootstrap failed: %w\n    command: %s", err, full)
	}
	return nil
}

// bootstrapRequirementsScript is a Python snippet that calls bootstrap's
// internal detection API directly (bypassing the broken CLI entry point) and
// prints the packages that need installing, one per line.
const bootstrapRequirementsScript = `
import json
try:
    from opentelemetry.instrumentation.bootstrap_gen import libraries, default_instrumentations
    from opentelemetry.instrumentation.bootstrap import _find_installed_libraries
    for pkg in _find_installed_libraries(default_instrumentations, libraries):
        print(pkg)
except Exception as e:
    import sys
    print("ERROR:" + str(e), file=sys.stderr)
    sys.exit(1)
`

// normalizePipName applies PEP 503 normalization: lowercase, replace
// underscores and dots with hyphens.
func normalizePipName(name string) string {
	n := strings.ToLower(name)
	n = strings.ReplaceAll(n, "_", "-")
	n = strings.ReplaceAll(n, ".", "-")
	return n
}

// listInstalledPipPackages returns the set of normalized package names
// installed in the Python environment.
func listInstalledPipPackages(pythonBin string) (map[string]bool, error) {
	out, err := exec.Command(pythonBin, "-m", "pip", "list", "--format=json").Output()
	if err != nil {
		return nil, fmt.Errorf("pip list failed: %w", err)
	}
	var packages []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &packages); err != nil {
		return nil, fmt.Errorf("parsing pip list output: %w", err)
	}
	set := make(map[string]bool, len(packages))
	for _, p := range packages {
		set[normalizePipName(p.Name)] = true
	}
	return set, nil
}

// queryBootstrapRequirements calls bootstrap's internal detection API via a
// Python snippet and returns the list of packages that need installing.
// Returns an error if the API is unavailable (e.g. API change across versions).
func queryBootstrapRequirements(pythonBin string, installed map[string]bool) ([]string, error) {
	cmd := exec.Command(pythonBin, "-c", bootstrapRequirementsScript)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bootstrap detection API unavailable: %s", stderr)
	}
	var pkgs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !installed[normalizePipName(line)] {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs, nil
}

// ensureFrameworkInstrumentations verifies that opentelemetry-bootstrap
// actually installed framework-specific instrumentation packages. If it
// didn't, uses bootstrap's internal detection API to find the needed packages
// and installs them directly. Prints clear diagnostics so the user knows
// exactly what happened.
func ensureFrameworkInstrumentations(pythonBin string, pip *pipCommand) error {
	installed, err := listInstalledPipPackages(pythonBin)
	if err != nil {
		return err
	}

	// Quick check: any opentelemetry-instrumentation-* package (beyond the base
	// opentelemetry-instrumentation) means bootstrap did install something.
	for pkg := range installed {
		if strings.HasPrefix(pkg, "opentelemetry-instrumentation-") &&
			pkg != "opentelemetry-instrumentation" {
			return nil // bootstrap worked
		}
	}

	// Bootstrap missed — query its internal API to find what's needed.
	missing, err := queryBootstrapRequirements(pythonBin, installed)
	if err != nil {
		fmt.Printf("\n    Warning: could not detect missing instrumentation packages: %v\n", err)
		fmt.Printf("    Run manually to install framework instrumentations:\n")
		fmt.Printf("      %s -m opentelemetry.instrumentation.bootstrap -a install\n", pythonBin)
		return nil // non-fatal: services will start but may lack trace spans
	}
	if len(missing) == 0 {
		return nil
	}

	fmt.Printf("\n    opentelemetry-bootstrap did not install framework instrumentations.\n")
	fmt.Printf("    Detected %d missing packages — installing directly:\n", len(missing))
	for _, pkg := range missing {
		fmt.Printf("      %s\n", pkg)
	}

	if err := installPackages(pip, missing); err != nil {
		fmt.Println()
		fmt.Println("    Some instrumentation packages failed to install.")
		fmt.Printf("    To install them manually, run:\n")
		fmt.Printf("      %s -m pip install %s\n", pythonBin, strings.Join(missing, " "))
		return err
	}

	// Final verification — report any remaining gaps.
	updatedInstalled, err := listInstalledPipPackages(pythonBin)
	if err != nil {
		return nil // install succeeded, just can't verify
	}
	var stillMissing []string
	for _, pkg := range missing {
		if !updatedInstalled[normalizePipName(pkg)] {
			stillMissing = append(stillMissing, pkg)
		}
	}
	if len(stillMissing) > 0 {
		fmt.Println()
		fmt.Printf("    Warning: %d instrumentation packages are still not installed:\n", len(stillMissing))
		for _, pkg := range stillMissing {
			fmt.Printf("      %s\n", pkg)
		}
		fmt.Printf("    To install manually:\n")
		fmt.Printf("      %s -m pip install %s\n", pythonBin, strings.Join(stillMissing, " "))
	}

	return nil
}

// installProjectDeps installs the project's own dependencies using the
// appropriate file: requirements.txt, Pipfile, pyproject.toml, or setup.py.
// Returns the description of what was installed, or "" if nothing found.
func installProjectDeps(pip *pipCommand, projectPath string) (string, error) {
	type depSource struct {
		file    string
		pipArgs []string
		label   string
	}
	sources := []depSource{
		{"requirements.txt", []string{"install", "-r"}, "requirements.txt"},
		{"pyproject.toml", []string{"install", "."}, "pyproject.toml"},
		{"setup.py", []string{"install", "."}, "setup.py"},
	}

	for _, src := range sources {
		srcPath := filepath.Join(projectPath, src.file)
		if _, err := os.Stat(srcPath); err != nil {
			continue
		}
		args := append(append([]string{}, pip.args...), src.pipArgs...)
		if src.file == "requirements.txt" {
			args = append(args, srcPath)
		}
		full := pip.name + " " + strings.Join(args, " ")
		fmt.Printf("\n    %s\n", full)
		cmd := exec.Command(pip.name, args...)
		cmd.Dir = projectPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			os.Stdout.Write(out)
			return "", fmt.Errorf("pip install failed (%s): %w\n    command: %s", src.label, err, full)
		}
		return src.label, nil
	}
	return "", nil
}

// projectDepsDescription returns a human-readable description of how project
// dependencies would be installed, or "" if no supported file is found.
func projectDepsDescription(projectPath string) string {
	if _, err := os.Stat(filepath.Join(projectPath, "requirements.txt")); err == nil {
		return "pip install -r requirements.txt"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "pyproject.toml")); err == nil {
		return "pip install . (pyproject.toml)"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "setup.py")); err == nil {
		return "pip install . (setup.py)"
	}
	return ""
}
