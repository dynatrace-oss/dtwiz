package otel

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type pipCommand struct {
	name string
	args []string
}

var otelPythonPackages = []string{
	"opentelemetry-distro",
	"opentelemetry-exporter-otlp",
}

func installPackages(pip *pipCommand, packages []string) error {
	args := append(append([]string{}, pip.args...), append([]string{"install"}, packages...)...)
	logger.Debug("running pip install", "pip", pip.name, "args", args)
	cmd := exec.Command(pip.name, args...)
	full := pip.name + " " + strings.Join(args, " ")
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stdout.Write(out)
		return fmt.Errorf("pip install failed: %w\n    command: %s", err, full)
	}
	return nil
}

func runOtelBootstrap(pythonPath string) error {
	args := []string{"-m", "opentelemetry.instrumentation.bootstrap", "-a", "install"}
	logger.Debug("running opentelemetry-bootstrap", "python", pythonPath, "args", args)
	cmd := exec.Command(pythonPath, args...)
	full := pythonPath + " " + strings.Join(args, " ")
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stdout.Write(out)
		return fmt.Errorf("opentelemetry-bootstrap failed: %w\n    command: %s", err, full)
	}
	return nil
}

// bootstrapRequirementsScript calls bootstrap's internal detection API directly,
// bypassing the CLI entry point, and prints packages that need installing one per line.
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

func normalizePipName(name string) string {
	n := strings.ToLower(name)
	n = strings.ReplaceAll(n, "_", "-")
	n = strings.ReplaceAll(n, ".", "-")
	return n
}

// pipPackageName extracts the bare package name from a pip requirement
// specifier such as "opentelemetry-instrumentation-flask==0.61b0" or
// "requests>=2.0,<3" and normalizes it for comparison with pip-list output.
func pipPackageName(spec string) string {
	if i := strings.IndexAny(spec, "><=!~;["); i != -1 {
		spec = spec[:i]
	}
	return normalizePipName(strings.TrimSpace(spec))
}

func listInstalledPipPackages(pythonBin string) (map[string]bool, error) {
	args := []string{"-m", "pip", "list", "--format=json"}
	logger.Debug("listing installed pip packages", "python", pythonBin)
	cmd := exec.Command(pythonBin, args...)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("pip list failed: %w\n    command: %s %s\n    %s", err, pythonBin, strings.Join(args, " "), stderr)
	}
	var packages []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &packages); err != nil {
		return nil, fmt.Errorf("parsing pip list output: %w\n    pip may have printed warnings that corrupted the output — run: %s -m pip list --format=json", err, pythonBin)
	}
	set := make(map[string]bool, len(packages))
	for _, p := range packages {
		set[normalizePipName(p.Name)] = true
	}
	return set, nil
}

// queryBootstrapRequirements calls bootstrap's internal detection API and returns
// packages that need installing. Returns an error if the API is unavailable.
func queryBootstrapRequirements(pythonBin string, installed map[string]bool) ([]string, error) {
	logger.Debug("querying bootstrap requirements", "python", pythonBin)
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
		if line != "" && !installed[pipPackageName(line)] {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs, nil
}

func ensureFrameworkInstrumentations(pythonBin string, pip *pipCommand) error {
	logger.Debug("verifying framework instrumentations after bootstrap", "python", pythonBin)
	installed, err := listInstalledPipPackages(pythonBin)
	if err != nil {
		return err
	}

	// Quick check: any opentelemetry-instrumentation-* package (beyond the base
	// opentelemetry-instrumentation) means bootstrap did install something.
	for pkg := range installed {
		if strings.HasPrefix(pkg, "opentelemetry-instrumentation-") &&
			pkg != "opentelemetry-instrumentation" {
			logger.Debug("framework instrumentations already present — bootstrap succeeded")
			return nil
		}
	}

	// Bootstrap missed — query its internal API to find what's needed.
	logger.Debug("no framework instrumentation packages detected, falling back to internal API")
	missing, err := queryBootstrapRequirements(pythonBin, installed)
	if err != nil {
		logger.Debug("bootstrap internal API unavailable", "error", err)
		display.StepWarn("Could not detect missing instrumentation packages",
			fmt.Sprintf("run manually: %s -m opentelemetry.instrumentation.bootstrap -a install", pythonBin))
		return nil // non-fatal: services will start but may lack trace spans
	}
	if len(missing) == 0 {
		logger.Debug("no framework instrumentations needed — no instrumented frameworks detected in project")
		return nil
	}

	logger.Debug("installing missing packages via pip", "count", len(missing))
	display.Detail("opentelemetry-bootstrap did not install framework instrumentations; installing %d missing packages directly: %s",
		len(missing), strings.Join(missing, " "))

	if err := installPackages(pip, missing); err != nil {
		return fmt.Errorf("%w\n    install them manually: %s -m pip install %s", err, pythonBin, strings.Join(missing, " "))
	}

	// Final verification — report any remaining gaps.
	logger.Debug("verifying installed packages after pip install")
	updatedInstalled, err := listInstalledPipPackages(pythonBin)
	if err != nil {
		logger.Debug("install succeeded but post-install verification skipped — pip list failed", "error", err)
		return nil
	}
	var stillMissing []string
	for _, pkg := range missing {
		if !updatedInstalled[pipPackageName(pkg)] {
			stillMissing = append(stillMissing, pkg)
		}
	}
	if len(stillMissing) > 0 {
		logger.Debug("post-install verification found remaining gaps", "still_missing", len(stillMissing))
		display.StepWarn(fmt.Sprintf("%d instrumentation packages are still not installed", len(stillMissing)),
			fmt.Sprintf("install manually: %s -m pip install %s", pythonBin, strings.Join(stillMissing, " ")))
	} else {
		logger.Debug("all missing packages successfully installed", "count", len(missing))
	}

	return nil
}

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
		display.Detail("%s", full)
		logger.Debug("running pip install from source", "pip", pip.name, "source", src.file, "args", args)
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
