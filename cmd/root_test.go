package cmd

import (
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
)

func TestResolveMode(t *testing.T) {
	mode := resolveMode()

	validModes := map[selfmonitoring.Mode]bool{
		selfmonitoring.ModeDebug:  true,
		selfmonitoring.ModeTTY:    true,
		selfmonitoring.ModeNonTTY: true,
	}

	if !validModes[mode] {
		t.Errorf("resolveMode() = %q, want one of: debug, tty, non-tty", mode)
	}
}

// ── watchSignalProps ─────────────────────────────────────────────────────────

func TestWatchSignalProps_NamedKeys(t *testing.T) {
	props := watchSignalProps(map[string]int64{"hst": 12000, "k8s": 5000, "svc": 999})
	if got := props["hosts"]; got != "12000" {
		t.Errorf("hosts = %q, want %q", got, "12000")
	}
	if got := props["kubernetes"]; got != "5000" {
		t.Errorf("kubernetes = %q, want %q", got, "5000")
	}
	if got := props["services"]; got != "999" {
		t.Errorf("services = %q, want %q", got, "999")
	}
	if _, ok := props["cloud"]; ok {
		t.Error("cloud must not be present when not seen")
	}
}

func TestWatchSignalProps_NilReturnsNil(t *testing.T) {
	if got := watchSignalProps(nil); got != nil {
		t.Errorf("watchSignalProps(nil) = %v, want nil", got)
	}
}

// ── watchSignalCSV ───────────────────────────────────────────────────────────

func TestWatchSignalCSV_PositionalEncoding(t *testing.T) {
	// order: cld,exc,hst,k8s,log,rel,req,svc
	// hst=5678ms→5s (pos 2), k8s=9ms→1 (sub-second, pos 3), svc=1234ms→1s (pos 7); rest absent → 0
	got := watchSignalCSV(map[string]int64{"svc": 1234, "hst": 5678, "k8s": 9})
	if got != "0,0,5,1,0,0,0,1" {
		t.Errorf("watchSignalCSV = %q, want %q", got, "0,0,5,1,0,0,0,1")
	}
}

func TestWatchSignalCSV_SubSecondEncodesAsOne(t *testing.T) {
	// sub-second signals must encode as 1, not 0, so 0 exclusively means absent
	got := watchSignalCSV(map[string]int64{"svc": 999, "hst": 1, "cld": 500})
	if got != "1,0,1,0,0,0,0,1" {
		t.Errorf("watchSignalCSV = %q, want %q", got, "1,0,1,0,0,0,0,1")
	}
}

func TestWatchSignalCSV_AllSignals(t *testing.T) {
	got := watchSignalCSV(map[string]int64{
		"cld": 60000, "exc": 45000, "hst": 12000, "k8s": 5000,
		"log": 30000, "rel": 9000, "req": 25000, "svc": 3000,
	})
	if got != "60,45,12,5,30,9,25,3" {
		t.Errorf("watchSignalCSV = %q, want %q", got, "60,45,12,5,30,9,25,3")
	}
}

func TestWatchSignalCSV_EmptyMapReturnsEmpty(t *testing.T) {
	got := watchSignalCSV(nil)
	if got != "" {
		t.Errorf("watchSignalCSV(nil) = %q, want empty string", got)
	}
}

// ── watch event callbacks ───────────────────────────────────────────────────

// captureEvents redirects eventSink into a slice for the duration of the test.
// A slice, not a single value: a watch session now emits one event per newly seen
// signal type plus one on completion.
func captureEvents(t *testing.T) *[]selfmonitoring.EventParams {
	t.Helper()
	var captured []selfmonitoring.EventParams
	original := eventSink
	eventSink = func(p selfmonitoring.EventParams) { captured = append(captured, p) }
	t.Cleanup(func() { eventSink = original })
	return &captured
}

// Both callbacks are invoked from post-install contexts (e.g. install otel), where Cmd
// and Sub must be overridden to "watch" / "" regardless of the triggering command.
func TestBuildWatchEventCallbacks_AlwaysEmitWatchCmd(t *testing.T) {
	captured := captureEvents(t)

	r := installer.WatchSessionResult{FirstDataMs: map[string]int64{"hst": 5000}}
	buildWatchSnapshotEventCallback(installCmd)(r)
	buildWatchEventCallback(installCmd)(r)

	if len(*captured) != 2 {
		t.Fatalf("captured %d events, want 2", len(*captured))
	}
	for _, p := range *captured {
		if p.Cmd != "watch" {
			t.Errorf("step %q: Cmd = %q, want %q", p.StepID, p.Cmd, "watch")
		}
		if p.Sub != "" {
			t.Errorf("step %q: Sub = %q, want empty", p.StepID, p.Sub)
		}
	}
}

func TestBuildWatchSnapshotEventCallback_CarriesSignalData(t *testing.T) {
	captured := captureEvents(t)

	buildWatchSnapshotEventCallback(watchCmd)(installer.WatchSessionResult{
		FirstDataMs: map[string]int64{"hst": 1200, "svc": 8100},
	})

	if len(*captured) != 1 {
		t.Fatalf("captured %d events, want 1", len(*captured))
	}
	p := (*captured)[0]
	if p.StepID != selfmonitoring.StepSnapshot {
		t.Errorf("StepID = %q, want %q", p.StepID, selfmonitoring.StepSnapshot)
	}
	if p.Type != "0,0,1,0,0,0,0,8" {
		t.Errorf("Type = %q, want %q", p.Type, "0,0,1,0,0,0,0,8")
	}
	if p.ExtraProps["hosts"] != "1200" || p.ExtraProps["services"] != "8100" {
		t.Errorf("ExtraProps = %v, want hosts=1200 and services=8100", p.ExtraProps)
	}
}

// The completion event marks that the session ended, nothing more. Timings live in the
// snapshot events, so it must not carry them even when the result still holds them.
func TestBuildWatchEventCallback_CarriesNoSignalData(t *testing.T) {
	captured := captureEvents(t)

	buildWatchEventCallback(watchCmd)(installer.WatchSessionResult{
		FirstDataMs: map[string]int64{"hst": 1200, "svc": 8100},
		ExitReason:  "user_exit",
	})

	if len(*captured) != 1 {
		t.Fatalf("captured %d events, want 1", len(*captured))
	}
	p := (*captured)[0]
	if p.StepID != selfmonitoring.StepCompleted {
		t.Errorf("StepID = %q, want %q", p.StepID, selfmonitoring.StepCompleted)
	}
	if p.Type != "" {
		t.Errorf("Type = %q, want empty", p.Type)
	}
	if p.ExtraProps != nil {
		t.Errorf("ExtraProps = %v, want nil", p.ExtraProps)
	}
}

// The header is bounded by a 64-char capture limit; the body keeps the real value.
func TestWatchSignalCSV_ClampsToThreeDigits(t *testing.T) {
	firstDataMs := map[string]int64{"hst": 1_500_000}

	if got := watchSignalCSV(firstDataMs); got != "0,0,999,0,0,0,0,0" {
		t.Errorf("watchSignalCSV = %q, want %q", got, "0,0,999,0,0,0,0,0")
	}
	if got := watchSignalProps(firstDataMs)["hosts"]; got != "1500000" {
		t.Errorf("props[hosts] = %q, want %q (body must stay unclamped)", got, "1500000")
	}
}

// ── invoked-event ownership ─────────────────────────────────────────────────

// Cobra runs only the closest PersistentPreRun in the parent chain, so exactly one
// place fires the inv event per invocation: root's hook, unless a subcommand overrides
// it and takes over the job. A command that overrides must call fireInvokedEvent
// itself; a command that does not must leave it to root.
//
// This pins which subcommands override. It fails if one gains or loses an override
// without the matching change to its event firing — which is how `setup` ended up
// sending two inv events per run.
func TestPersistentPreRunOwnership(t *testing.T) {
	overrides := map[string]bool{
		"install":   true,
		"update":    true,
		"uninstall": true,
	}

	if rootCmd.PersistentPreRun == nil {
		t.Fatal("rootCmd.PersistentPreRun is nil; nothing fires the inv event")
	}

	for _, cmd := range rootCmd.Commands() {
		name := cmd.Name()
		hasOverride := cmd.PersistentPreRun != nil
		switch {
		case hasOverride && !overrides[name]:
			t.Errorf("%q now overrides PersistentPreRun, which suppresses root's inv event; "+
				"it must call fireInvokedEvent itself, then be added to this test", name)
		case !hasOverride && overrides[name]:
			t.Errorf("%q no longer overrides PersistentPreRun, so root now fires the inv event; "+
				"remove its own fireInvokedEvent call and drop it from this test", name)
		}
	}
}
