package installer

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/fatih/color"
)

// InstallOtelCollector installs the Dynatrace OTel Collector and offers
// runtime auto-instrumentation (Python, Java, …) in a single guided flow.
func InstallOtelCollector(envURL, token, ingestToken, platformToken string, dryRun bool) error {
	cyan := color.New(color.FgMagenta)

	fmt.Println()
	cyan.Println("  Dynatrace OpenTelemetry Installation")
	fmt.Println()

	cp, err := prepareCollectorPlan(envURL, token, ingestToken)
	if err != nil {
		return err
	}

	if dryRun {
		cp.printDryRun(ingestToken)
		return nil
	}

	// Detect runtimes and let the user pick a project upfront, before
	// showing the combined plan and installing anything.
	var pythonPlan *PythonInstrumentationPlan
	if _, err := exec.LookPath("python3"); err == nil {
		pythonPlan = DetectPythonPlan(cp.apiURL, token)
	}
	fmt.Println()

	// Show combined plan: collector + instrumentation.
	sep := strings.Repeat("─", 60)

	if pythonPlan != nil {
		cyan.Println("  This will install the OTel Collector and auto-instrument your application.")
	}
	fmt.Println()

	cyan.Println("  1) OTel Collector")
	fmt.Printf("     Directory: %s\n", cp.installDir)
	fmt.Printf("     Binary:    %s\n", cp.binaryPath)
	if len(cp.runningPIDs) > 0 {
		for _, rc := range cp.runningPIDs {
			if rc.path != "" {
				fmt.Printf("     Running:  Existing OTel Collector PID %d at %s (will be stopped)\n", rc.pid, rc.path)
			} else {
				fmt.Printf("     Running:  Existing OTel Collector PID %d (will be stopped)\n", rc.pid)
			}
		}
	}

	cp.printConfigPreview(cyan, sep)

	if pythonPlan != nil {
		fmt.Println()
		cyan.Println("  2) Python auto-instrumentation")
		pythonPlan.PrintPlanSteps()
	}

	fmt.Println()
	ok, err := confirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return nil
	}
	fmt.Println()

	if err := cp.execute(envURL, platformToken, pythonPlan != nil); err != nil {
		return err
	}

	// Execute the Python instrumentation plan if one was chosen.
	if pythonPlan != nil {
		pythonPlan.EnvURL = envURL
		pythonPlan.PlatformToken = platformToken
		pythonPlan.EnvVars = generateOtelPythonEnvVars(cp.apiURL, cp.collectorToken, "my-service")

		fmt.Printf("\n  ── Python auto-instrumentation ──\n\n")
		pythonPlan.Execute()
	}

	return nil
}
