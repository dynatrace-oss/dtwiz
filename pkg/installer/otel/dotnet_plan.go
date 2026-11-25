package otel

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
)

// dotNetManualSlug is the manual-languages slug whose walkthrough ends step 1.
const dotNetManualSlug = "dotnet"

// showDotNetStep1Outcome prints the services of the selected .NET project with
// their target frameworks and not-supported flags, and returns the manual
// language slug so the flow ends in the manual-steps outcome with the Dynatrace
// .NET walkthrough. It installs, stops and starts nothing.
func showDotNetStep1Outcome(proj detectedProject) string {
	services := dotNetServices(proj.ScannedProject)
	fmt.Println()
	if len(services) == 0 {
		fmt.Printf("  No runnable .NET service found in %s.\n", proj.Path)
		return dotNetManualSlug
	}
	display.ColorMessage.Printf("  .NET services in %s:\n", proj.Path)
	for _, s := range services {
		line := fmt.Sprintf("    %s  (%s)", s.Name, s.FrameworkLabel())
		if reason := s.UnsupportedReason(); reason != "" {
			line += "  [" + reason + "]"
		}
		fmt.Println(line)
	}
	fmt.Println()
	fmt.Println("  Automatic .NET instrumentation is not available yet; manual steps follow.")
	return dotNetManualSlug
}
