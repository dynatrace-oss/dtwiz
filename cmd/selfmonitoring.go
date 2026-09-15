package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
)

func fireSelfMonitoringEvent(params selfmonitoring.EventParams) {
	if !featureflags.IsEnabled(featureflags.SelfMonitoringPoC) {
		return
	}
	go func() {
		envURL, _, platformTok, err := getDtEnvironment()
		if err != nil {
			logger.Debug(fmt.Sprintf("selfmonitoring: could not resolve credentials: %v", err))
			return
		}
		if err := selfmonitoring.SendEvent(installer.APIURL(envURL), platformTok, params); err != nil {
			logger.Debug(fmt.Sprintf("selfmonitoring: %v", err))
		}
	}()
}

func buildEventParams(cmd *cobra.Command, stepID string) selfmonitoring.EventParams {
	cmdID, subID := deriveCommandIDs(cmd)
	return selfmonitoring.EventParams{
		CmdID:  cmdID,
		SubID:  subID,
		StepID: stepID,
		Mode:   resolveMode(),
	}
}

var normCmdMap = map[string]string{
	"install":   "ins",
	"uninstall": "uni",
	"update":    "upd",
	"analyze":   "ana",
	"recommend": "rec",
	"status":    "sta",
	"watch":     "wch",
	"setup":     "set",
	"version":   "ver",
}

var normSubMap = map[string]string{
	"otel":           "otel",
	"otel-collector": "otlc",
	"otel-python":    "otlp",
	"otel-node":      "otln",
	"otel-java":      "otlj",
	"kubernetes":     "k8s",
	"oneagent":       "oa",
	"gcp":            "gcp",
	"azure":          "az",
	"aws":            "aws",
	"aws-lambda":     "awsl",
	"docker":         "dock",
	"demo":           "demo",
	"self":           "self",
}

func deriveCommandIDs(cmd *cobra.Command) (cmdID, subID string) {
	parent := cmd.Parent()
	if parent == nil || parent.Name() == "dtwiz" {
		return normCmd(cmd.Name()), ""
	}
	return normCmd(parent.Name()), normSub(cmd.Name())
}

func normCmd(name string) string {
	if short, ok := normCmdMap[name]; ok {
		return short
	}
	return name
}

func normSub(name string) string {
	if short, ok := normSubMap[name]; ok {
		return short
	}
	return name
}

func resolveMode() string {
	if debugFlag {
		return "deb"
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return "tty"
	}
	return "ntt"
}
