package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
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
	"otel-update":    "otlu",
	"azure-update":   "azu",
	"gcp-update":     "gcpu",
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

var watchSignalNames = map[string]string{
	"cld": "cloud",
	"exc": "exceptions",
	"hst": "hosts",
	"k8s": "kubernetes",
	"log": "logs",
	"rel": "relationships",
	"req": "requests",
	"svc": "services",
}

// watchSignalOrder is the fixed positional order used by watchSignalCSV (alphabetical).
var watchSignalOrder = []string{"cld", "exc", "hst", "k8s", "log", "rel", "req", "svc"}

// watchSignalProps builds event body properties from signal first-data timing.
// Each seen signal gets a full-name key (e.g. "hosts") and a value in milliseconds.
// Absent signals produce no key. Returns nil when no signals were seen.
func watchSignalProps(firstDataMs map[string]int64) map[string]string {
	if len(firstDataMs) == 0 {
		return nil
	}
	props := make(map[string]string, len(firstDataMs))
	for sig, ms := range firstDataMs {
		key := watchSignalNames[sig]
		if key == "" {
			key = sig
		}
		props[key] = strconv.FormatInt(ms, 10)
	}
	return props
}

// watchSignalCSV encodes time-to-first-data in a positional format.
// Format: "0,0,12,5,0,9,0,3" — one value per signal in watchSignalOrder.
// 0 = signal not seen; ≥1 = whole seconds to first data (minimum 1, even if sub-second).
// Returns "" when no signals were seen (t= field is then omitted from the header).
func watchSignalCSV(firstDataMs map[string]int64) string {
	if len(firstDataMs) == 0 {
		return ""
	}
	parts := make([]string, len(watchSignalOrder))
	for i, sig := range watchSignalOrder {
		if ms, ok := firstDataMs[sig]; ok {
			if secs := ms / 1000; secs > 0 {
				parts[i] = strconv.FormatInt(secs, 10)
			} else {
				parts[i] = "1" // present but sub-second — distinguish from absent (0)
			}
		} else {
			parts[i] = "0"
		}
	}
	return strings.Join(parts, ",")
}

// buildWatchEventCallback returns the onEvent callback used by all post-install
// and standalone watch sessions to emit a selfmonitoring StepCompleted event
// encoding which signals arrived and how quickly.
func buildWatchEventCallback(cmd *cobra.Command) func(installer.WatchSessionResult) {
	return func(r installer.WatchSessionResult) {
		params := buildEventParams(cmd, selfmonitoring.StepCompleted)
		params.CmdID = ""
		params.Type = watchSignalCSV(r.FirstDataMs)
		params.ExtraProps = watchSignalProps(r.FirstDataMs)
		fireSelfMonitoringEvent(params)
	}
}

func fireInvokedEvent(cmd *cobra.Command) {
	fireSelfMonitoringEvent(buildEventParams(cmd, selfmonitoring.StepInvoked))
}

func completedEventParams(cmd *cobra.Command, err error) selfmonitoring.EventParams {
	p := buildEventParams(cmd, selfmonitoring.StepCompleted)
	if err != nil {
		p.Err = "err"
	}
	return p
}

func fireCompletedEvent(cmd *cobra.Command, err error) {
	fireSelfMonitoringEvent(completedEventParams(cmd, err))
}

func fireSetupAnalyzeEvent(cmd *cobra.Command, err error) {
	p := buildEventParams(cmd, selfmonitoring.StepAnalyze)
	if err != nil {
		p.Err = "err"
	}
	fireSelfMonitoringEvent(p)
}

func fireSetupRecommendEvent(cmd *cobra.Command, subID string) {
	p := buildEventParams(cmd, selfmonitoring.StepRecommend)
	p.SubID = subID
	fireSelfMonitoringEvent(p)
}

func fireSetupInstallEvent(cmd *cobra.Command, subID string, err error) {
	if errors.Is(err, installer.ErrInstallCancelled) || errors.Is(err, otel.ErrUpToDate) {
		return
	}
	p := buildEventParams(cmd, selfmonitoring.StepInstall)
	p.SubID = subID
	if err != nil {
		p.Err = "err"
	}
	fireSelfMonitoringEvent(p)
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
