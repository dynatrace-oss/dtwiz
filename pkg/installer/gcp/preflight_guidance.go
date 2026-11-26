package gcp

import (
	"errors"
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// Verbs passed to reportPreflight; each maps to `dtwiz <verb> gcp` in the re-run step.
const (
	verbInstall   = "install"
	verbUpdate    = "update"
	verbUninstall = "uninstall"
)

// reportPreflight renders the guidance box for a missing gcloud account or project and
// returns an error that ends the command without an extra "Error: ..." line, classified
// as a config error for self-monitoring. Any other error is returned unchanged.
func reportPreflight(verb string, err error) error {
	switch {
	case errors.Is(err, errNoGCloudAccount):
		printGuidanceBox(noAccountGuidance(verb))
		return alreadyReportedConfigError("gcloud.account")
	case errors.Is(err, errNoGCloudProject):
		var account string
		var npe *noProjectError
		if errors.As(err, &npe) {
			account = npe.account
		}
		printGuidanceBox(noProjectGuidance(verb, account))
		return alreadyReportedConfigError("gcloud.project")
	default:
		return err
	}
}

func alreadyReportedConfigError(field string) error {
	return fmt.Errorf("%w: %w", installer.ErrAlreadyReported, &installer.ConfigError{MissingFields: []string{field}})
}

func printGuidanceBox(lines []string) {
	fmt.Println()
	display.PrintInfoBox(lines...)
	fmt.Println()
}

// guidanceStep formats one numbered step with the command aligned in a second column.
// The description is padded before coloring so ANSI codes don't skew the alignment.
func guidanceStep(n int, desc, command string) string {
	label := fmt.Sprintf("%d. %-22s", n, desc+":")
	return display.ColorMuted.Sprint(label) + " " + display.ColorEmphasis.Sprint(command)
}

// guidanceHeading formats the box's first line in the warning color: the command
// stopped, but nothing is broken.
func guidanceHeading(verb, reason string) string {
	return display.ColorWarning.Sprintf("Google Cloud %s not run: %s.", verb, reason)
}

func rerunCommand(verb string) string {
	return "dtwiz " + verb + " gcp"
}

// dynatraceOnlyHint is appended to the uninstall boxes: without it, a deleted or
// inaccessible project would leave the Dynatrace resources impossible to remove.
func dynatraceOnlyHint(verb string) []string {
	if verb != verbUninstall {
		return nil
	}
	return []string{
		"",
		"Project deleted or no longer accessible? Remove only the Dynatrace side:",
		"   " + display.ColorEmphasis.Sprint(rerunCommand(verb)+" --"+DynatraceOnlyFlag),
	}
}

func noAccountGuidance(verb string) []string {
	lines := []string{
		guidanceHeading(verb, "no gcloud account is logged in"),
		"",
		guidanceStep(1, "Log in", "gcloud auth login"),
		guidanceStep(2, "Re-run", rerunCommand(verb)),
	}
	return append(lines, dynatraceOnlyHint(verb)...)
}

func noProjectGuidance(verb, account string) []string {
	lines := []string{
		guidanceHeading(verb, "no gcloud project is selected"),
	}
	if account != "" {
		lines = append(lines, display.ColorMuted.Sprint("Logged in as:")+" "+account)
	}
	if verb == verbUninstall {
		lines = append(lines,
			"",
			"Without a project, only the Dynatrace resources could be removed;",
			"the GCP service account and its Viewer binding would stay behind.",
		)
	}
	lines = append(lines,
		"",
		guidanceStep(1, "Find your project ID", "gcloud projects list"),
		guidanceStep(2, "Select it", "gcloud config set project <PROJECT_ID>"),
		guidanceStep(3, "Re-run", rerunCommand(verb)),
	)
	return append(lines, dynatraceOnlyHint(verb)...)
}
