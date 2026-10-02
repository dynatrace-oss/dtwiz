package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/dynatrace-oss/dtwiz/pkg/recommender"
	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
)

// eventSink is the function that handles a fully-built EventParams.
// Replaced in tests to capture params without firing a real HTTP request.
var eventSink = func(params selfmonitoring.EventParams) {
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

// watchSignalSecondsCap bounds each value in the positional CSV to three digits.
// Sessions extended past the 10-minute prompt can run arbitrarily long, and the
// User-Agent has a 64-char capture limit; the event body carries the unclamped
// milliseconds, so clamping only degrades the fallback channel.
const watchSignalSecondsCap = 999

// watchSignalCSV encodes time-to-first-data in a positional format.
// Format: "0,0,12,5,0,9,0,3" — one value per signal in watchSignalOrder.
// 0 = signal not seen; ≥1 = whole seconds to first data (minimum 1, even if sub-second),
// clamped to watchSignalSecondsCap.
// Returns "" when no signals were seen (t= field is then omitted from the header).
func watchSignalCSV(firstDataMs map[string]int64) string {
	if len(firstDataMs) == 0 {
		return ""
	}
	parts := make([]string, len(watchSignalOrder))
	for i, sig := range watchSignalOrder {
		if ms, ok := firstDataMs[sig]; ok {
			secs := ms / 1000
			switch {
			case secs > watchSignalSecondsCap:
				parts[i] = strconv.Itoa(watchSignalSecondsCap)
			case secs > 0:
				parts[i] = strconv.FormatInt(secs, 10)
			default:
				parts[i] = "1" // present but sub-second — distinguish from absent (0)
			}
		} else {
			parts[i] = "0"
		}
	}
	return strings.Join(parts, ",")
}

// watchEventParams builds the params shared by both watch events. Cmd is pinned to
// "watch" and Sub cleared so post-install watch sessions report as watch rather than
// as the install command that triggered them.
func watchEventParams(cmd *cobra.Command, stepID string) selfmonitoring.EventParams {
	params := buildEventParams(cmd, stepID)
	params.Cmd = "watch"
	params.Sub = ""
	return params
}

// buildWatchSnapshotEventCallback returns the onSnapshot callback used by all
// post-install and standalone watch sessions. It fires once per signal type that
// newly received data, carrying the cumulative timings for every type seen so far.
func buildWatchSnapshotEventCallback(cmd *cobra.Command) func(installer.WatchSessionResult) {
	return func(r installer.WatchSessionResult) {
		params := watchEventParams(cmd, selfmonitoring.StepSnapshot)
		params.Type = watchSignalCSV(r.FirstDataMs)
		params.ExtraProps = watchSignalProps(r.FirstDataMs)
		fireSelfMonitoringEvent(params)
	}
}

// buildWatchEventCallback returns the onComplete callback used by all post-install
// and standalone watch sessions. It fires once when the session ends and carries no
// signal data: the timings live in the snapshot events. Its absence for an execution
// means the process ended before the session could report.
func buildWatchEventCallback(cmd *cobra.Command) func(installer.WatchSessionResult) {
	return func(installer.WatchSessionResult) {
		fireSelfMonitoringEvent(watchEventParams(cmd, selfmonitoring.StepCompleted))
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

func fireSetupAnalyzeEvent(cmd *cobra.Command, cloudProvider, k8sDistro string, err error) {
	p := buildEventParams(cmd, selfmonitoring.StepAnalyze)
	p.CloudProvider = cloudProvider
	p.K8sDistro = k8sDistro
	if err != nil {
		p.Err = "err"
	}
	fireSelfMonitoringEvent(p)
}

func fireSetupRecommendEvent(cmd *cobra.Command, method string) {
	p := buildEventParams(cmd, selfmonitoring.StepRecommendationsSelected)
	p.Opt = method
	fireSelfMonitoringEvent(p)
}

// fireSetupMenuEvent fires one self-monitoring event per presented recommendation.
func fireSetupMenuEvent(cmd *cobra.Command, recs []recommender.Recommendation) {
	for _, r := range recs {
		p := buildEventParams(cmd, selfmonitoring.StepRecommendationsPresented)
		p.Opt = string(r.Method)
		fireSelfMonitoringEvent(p)
	}
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
	applyInstallOutcomes(&p, err)
	fireSelfMonitoringEvent(p)
}

// installFeatures lists the features reported in the install event, in the fixed order
// of their characters in the User-Agent f= field, with the body property of each.
// Positions are add-only: never reorder or reuse them, so a position keeps its meaning
// across versions. A shorter f= from an older version means the trailing features were
// unknown to it, which is not the same as "-" (not tried).
var installFeatures = []struct {
	feature installer.Feature
	prop    string
}{
	{installer.FeatureOtelConfig, "install.otel_config_written"},
	{installer.FeatureHostMonitoring, "install.host_monitoring"},
	{installer.FeatureRUM, "install.rum"},
	{installer.FeatureSynthetic, "install.synthetic_monitoring"},
	{installer.FeatureRDSExtensions, "install.rds_extensions"},
}

// installDurationSecondsCap bounds d= in the User-Agent to four digits. The body carries
// the unclamped milliseconds, so clamping only degrades the fallback channel.
const installDurationSecondsCap = 9999

func installFeatureChar(o installer.Outcome) string {
	switch o {
	case installer.OutcomeSucceeded:
		return "1"
	case installer.OutcomeFailed:
		return "0"
	default:
		return "-"
	}
}

func installFeatureValue(o installer.Outcome) string {
	switch o {
	case installer.OutcomeSucceeded:
		return "succeeded"
	case installer.OutcomeFailed:
		return "failed"
	default:
		return "not_tried"
	}
}

// applyInstallOutcomes adds the install work time and the feature outcomes to an ist
// event: the compact forms for the User-Agent and the readable properties for the body.
// It adds nothing when no install work started: dry-run, a declined confirmation, or a
// failure before the confirmation.
func applyInstallOutcomes(p *selfmonitoring.EventParams, err error) {
	if errors.Is(err, installer.ErrInstallCancelled) {
		return
	}
	elapsed, started := installer.InstallWorkTime()
	if !started {
		return
	}
	features, durationS, body := installOutcomeFields(elapsed, installer.FeatureOutcomes())
	p.Features = features
	p.DurationS = durationS

	if p.ExtraProps == nil {
		p.ExtraProps = body
		return
	}
	for k, v := range body {
		p.ExtraProps[k] = v
	}
}

// installOutcomeFields encodes the install work time and feature outcomes: the f= and
// d= values for the User-Agent, and the readable properties for the event body.
func installOutcomeFields(elapsed time.Duration, outcomes map[installer.Feature]installer.Outcome) (features, durationS string, body map[string]string) {
	var chars strings.Builder
	body = make(map[string]string, len(installFeatures)+1)
	for _, f := range installFeatures {
		chars.WriteString(installFeatureChar(outcomes[f.feature]))
		body[f.prop] = installFeatureValue(outcomes[f.feature])
	}
	body["install.duration_ms"] = strconv.FormatInt(elapsed.Milliseconds(), 10)

	secs := int64(elapsed / time.Second)
	if secs > installDurationSecondsCap {
		secs = installDurationSecondsCap
	}
	return chars.String(), strconv.FormatInt(secs, 10), body
}

// fireInstallEvent fires the StepInstall event for a direct dtwiz install <method>
// invocation, including cancellations (error = user_cancelled). The install work time
// and feature outcomes are included when install work started.
func fireInstallEvent(cmd *cobra.Command, err error) {
	p := buildEventParams(cmd, selfmonitoring.StepInstall)
	if err != nil {
		errType, attrs := selfmonitoring.ClassifyError(err)
		p.Err = string(errType)
		if len(attrs) > 0 {
			p.ExtraProps = make(map[string]string, len(attrs)+len(installFeatures)+1)
			for k, v := range attrs {
				p.ExtraProps[k] = v
			}
		}
	}
	applyInstallOutcomes(&p, err)
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
