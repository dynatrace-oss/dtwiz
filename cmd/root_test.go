package cmd

import (
	"testing"

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
