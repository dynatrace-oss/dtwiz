package installer

import (
	"sync"
	"time"
)

// Feature identifies a capability an install can enable and report on in the
// self-monitoring ist event. The order of positions in the User-Agent is owned by
// the cmd layer; these values are only identities.
type Feature int

const (
	FeatureOtelConfig Feature = iota
	FeatureHostMonitoring
	FeatureRUM
	FeatureSynthetic
	FeatureRDSExtensions
)

// Outcome is the result of trying to enable a Feature during this run.
type Outcome int

const (
	// OutcomeNotTried is the zero value: nothing was recorded for the feature.
	OutcomeNotTried Outcome = iota
	OutcomeSucceeded
	OutcomeFailed
)

// nowFn is the clock used by the install telemetry state. Replaced in tests.
var nowFn = time.Now

// pauseInterval is time spent at a prompt while install work was in progress.
type pauseInterval struct{ from, to time.Time }

// installTelemetry holds the per-run outcomes and the work-time stopwatch.
// Package-level state follows AutoConfirm and OnWatchComplete: only one install runs
// per process. The mutex covers the one concurrent writer, the AWS CloudFormation goroutine.
var installTelemetry struct {
	mu       sync.Mutex
	outcomes map[Feature]Outcome
	start    time.Time
	end      time.Time
	pauses   []pauseInterval
}

// RecordFeature records whether enabling f succeeded. The last call for a feature wins.
func RecordFeature(f Feature, ok bool) {
	installTelemetry.mu.Lock()
	defer installTelemetry.mu.Unlock()
	if installTelemetry.outcomes == nil {
		installTelemetry.outcomes = make(map[Feature]Outcome)
	}
	if ok {
		installTelemetry.outcomes[f] = OutcomeSucceeded
	} else {
		installTelemetry.outcomes[f] = OutcomeFailed
	}
}

// FeatureOutcomes returns a copy of the outcomes recorded since the last reset.
// Features that were never recorded are absent, which means not tried.
func FeatureOutcomes() map[Feature]Outcome {
	installTelemetry.mu.Lock()
	defer installTelemetry.mu.Unlock()
	out := make(map[Feature]Outcome, len(installTelemetry.outcomes))
	for f, o := range installTelemetry.outcomes {
		out[f] = o
	}
	return out
}

// ResetInstallTelemetry clears the recorded outcomes and the stopwatch. Called by the
// cmd layer before each installer runs.
func ResetInstallTelemetry() {
	installTelemetry.mu.Lock()
	defer installTelemetry.mu.Unlock()
	installTelemetry.outcomes = nil
	installTelemetry.start = time.Time{}
	installTelemetry.end = time.Time{}
	installTelemetry.pauses = nil
}

// StartInstallTimer starts the work-time stopwatch if it is not running yet. Install
// confirmations start it automatically; the cmd layer calls this for methods that have
// no confirmation prompt (e.g. a fresh OneAgent install).
func StartInstallTimer() {
	installTelemetry.mu.Lock()
	defer installTelemetry.mu.Unlock()
	if installTelemetry.start.IsZero() {
		installTelemetry.start = nowFn()
	}
}

// MarkInstallDone records the end of the install work as now. See MarkInstallDoneAt.
// Installers that run the post-install watch themselves must call this before the watch.
func MarkInstallDone() { MarkInstallDoneAt(nowFn()) }

// MarkInstallDoneAt records t as the end of the install work. The first call wins, and
// calls before the stopwatch started are ignored. When nothing marks the end, the work
// time ends at the moment InstallWorkTime is read.
func MarkInstallDoneAt(t time.Time) {
	installTelemetry.mu.Lock()
	defer installTelemetry.mu.Unlock()
	if installTelemetry.start.IsZero() || !installTelemetry.end.IsZero() {
		return
	}
	installTelemetry.end = t
}

// InstallWorkTime returns the install work time: the time from the start to the end,
// minus the time spent at prompts shown while the work was in progress. The bool is
// false when the stopwatch never started, i.e. no install work happened.
func InstallWorkTime() (time.Duration, bool) {
	installTelemetry.mu.Lock()
	defer installTelemetry.mu.Unlock()
	if installTelemetry.start.IsZero() {
		return 0, false
	}
	end := installTelemetry.end
	if end.IsZero() {
		end = nowFn()
	}
	elapsed := end.Sub(installTelemetry.start)
	for _, p := range installTelemetry.pauses {
		if !p.from.Before(end) {
			continue
		}
		to := p.to
		if to.After(end) {
			to = end
		}
		elapsed -= to.Sub(p.from)
	}
	if elapsed < 0 {
		elapsed = 0
	}
	return elapsed, true
}

// afterPrompt updates the stopwatch once a prompt that began at promptStart has been
// answered. A starting prompt (an install confirmation) that was accepted starts the
// stopwatch; a prompt shown while the stopwatch is running records the time spent at
// it as a pause, whatever the answer.
func afterPrompt(promptStart time.Time, accepted, starts bool) {
	installTelemetry.mu.Lock()
	defer installTelemetry.mu.Unlock()
	now := nowFn()
	if installTelemetry.start.IsZero() {
		if starts && accepted {
			installTelemetry.start = now
		}
		return
	}
	if now.After(promptStart) {
		installTelemetry.pauses = append(installTelemetry.pauses, pauseInterval{from: promptStart, to: now})
	}
}
