package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/projectreport"
)

var (
	analyzeProjectsDir     string
	analyzeProjectsRuntime string
)

// detectProjectsFn is overridable in tests.
var detectProjectsFn = otel.DetectProjects

var analyzeProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List application projects that can be instrumented with OpenTelemetry",
	Long: `List the application projects below a directory that "dtwiz install otel" can instrument.

Read-only and non-interactive: needs no credentials and never prompts. Scans the
working directory, or the directory given by --dir, and nothing else. Each project
can be installed with:

  dtwiz install otel --project <path> --runtime <runtime>`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Input errors are reported on their own, without the usage text.
		cmd.Root().SilenceUsage = true
		err := runAnalyzeProjects(cmd.OutOrStdout())
		fireCompletedEvent(cmd, err)
		return err
	},
}

func runAnalyzeProjects(w io.Writer) error {
	runtimes, err := otel.ParseRuntimeFilter(analyzeProjectsRuntime)
	if err != nil {
		return err
	}
	inventory, err := detectProjectsFn(otel.ProjectQuery{Dir: analyzeProjectsDir, Runtimes: runtimes})
	if err != nil {
		return err
	}
	if analyzeJSON {
		return projectreport.RenderJSON(w, inventory)
	}
	return projectreport.RenderText(w, inventory)
}

// runtimeFlagUsage lists the currently valid runtime identifiers, so the help
// text only shows feature-flagged runtimes while their flag is on.
func runtimeFlagUsage(prefix string) string {
	return prefix + ": " + otel.FormatRuntimeIDs(otel.ActiveRuntimeIDs())
}

const analyzeProjectsRuntimeUsage = "comma-separated runtimes to list"

func init() {
	analyzeProjectsCmd.Flags().StringVar(&analyzeProjectsDir, "dir", "", "directory to scan (default: working directory)")
	analyzeProjectsCmd.Flags().StringVar(&analyzeProjectsRuntime, "runtime", "", runtimeFlagUsage(analyzeProjectsRuntimeUsage))
	analyzeCmd.AddCommand(analyzeProjectsCmd)

	// PersistentPreRun doesn't run for --help, so resolve feature flags here to
	// list only the runtimes that are currently valid. Rendering is delegated to
	// the root help func so its behavior (help event) is kept.
	analyzeCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		featureflags.ApplyCLIOverrides(cmd.Flags())
		analyzeProjectsCmd.Flags().Lookup("runtime").Usage = runtimeFlagUsage(analyzeProjectsRuntimeUsage)
		cmd.Root().HelpFunc()(cmd, args)
	})
}
