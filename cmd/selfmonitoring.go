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

// eventSink is the function that handles a fully-built EventParams.
// Replaced in tests to capture params without firing a real HTTP request.
var eventSink = func(params selfmonitoring.EventParams) {
	if !featureflags.IsEnabled(featureflags.SelfMonitoringPoC) {
		return
	}
	selfmonitoring.TrackSend()
	go func() {
		defer selfmonitoring.SendDone()
		// Only a missing tenant URL is fatal: there is nowhere to send. A missing or
		// rejected token is still worth attempting, because the request reaches the
		// tenant's HAProxy and its User-Agent capture records the attempt even when
		// the API rejects it — which is exactly how auth failures become visible.
		envURL := environmentHint()
		if envURL == "" {
			logger.Debug("selfmonitoring: no environment URL configured, dropping event")
			return
		}
		if err := selfmonitoring.SendEvent(installer.APIURL(envURL), platformToken(), params); err != nil {
			logger.Debug(fmt.Sprintf("selfmonitoring: %v", err))
		}
	}()
}

func fireSelfMonitoringEvent(params selfmonitoring.EventParams) {
	eventSink(params)
}

func fireSelfMonitoringEventWithError(params selfmonitoring.EventParams, err error) {
	errType, attrs := selfmonitoring.ClassifyError(err)
	params.Err = string(errType)
	if len(attrs) > 0 {
		if params.ExtraProps == nil {
			params.ExtraProps = make(map[string]string, len(attrs))
		}
		for k, v := range attrs {
			params.ExtraProps[k] = v
		}
	}
	fireSelfMonitoringEvent(params)
}

func buildEventParams(cmd *cobra.Command, stepID string) selfmonitoring.EventParams {
	cmdName, subName := deriveCommandNames(cmd)
	return selfmonitoring.EventParams{
		Cmd:    cmdName,
		Sub:    subName,
		StepID: stepID,
		Mode:   resolveMode(),
	}
}

func deriveCommandNames(cmd *cobra.Command) (cmdName, subName string) {
	parent := cmd.Parent()
	if parent == nil || parent.Name() == "dtwiz" {
		return cmd.Name(), ""
	}
	return parent.Name(), cmd.Name()
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
		params.Cmd = ""
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

func fireSetupRecommendEvent(cmd *cobra.Command, sub string) {
	p := buildEventParams(cmd, selfmonitoring.StepRecommend)
	p.Sub = sub
	fireSelfMonitoringEvent(p)
}

func fireSetupInstallEvent(cmd *cobra.Command, sub string, err error) {
	if errors.Is(err, installer.ErrInstallCancelled) || errors.Is(err, otel.ErrUpToDate) {
		return
	}
	p := buildEventParams(cmd, selfmonitoring.StepInstall)
	p.Sub = sub
	if err != nil {
		errType, attrs := selfmonitoring.ClassifyError(err)
		p.Err = string(errType)
		if len(attrs) > 0 {
			p.ExtraProps = make(map[string]string, len(attrs))
			for k, v := range attrs {
				p.ExtraProps[k] = v
			}
		}
	}
	fireSelfMonitoringEvent(p)
}

func resolveMode() selfmonitoring.Mode {
	if debugFlag {
		return selfmonitoring.ModeDebug
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return selfmonitoring.ModeTTY
	}
	return selfmonitoring.ModeNonTTY
}
