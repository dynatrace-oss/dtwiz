package installer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// InstallOtelCollector installs the Dynatrace OTel Collector and offers
// runtime auto-instrumentation (Python, Java, …) in a single guided flow.
func InstallOtelCollector(envURL, token, ingestToken, platformToken string, dryRun bool) error {
	fmt.Println()
	fmt.Println("  This installer will set up the Dynatrace OpenTelemetry Collector and instrument your chosen application.")
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

	// Show combined plan: collector + instrumentation.
	fmt.Println()
	fmt.Println("  ── Plan ──")
	fmt.Println()
	fmt.Println("  1) Install and run OTel Collector")
	fmt.Println()
	cp.printPlanSteps()

	if pythonPlan != nil {
		fmt.Println()
		fmt.Println("  2) Instrument and run Python application")
		fmt.Println()
		pythonPlan.PrintPlanSteps()
	}

	fmt.Println()
	fmt.Print("  Apply? [Y/n]: ")
	{
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "" && answer != "y" && answer != "yes" {
			return fmt.Errorf("installation aborted")
		}
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
