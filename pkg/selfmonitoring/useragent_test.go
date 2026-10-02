package selfmonitoring

import (
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

func TestBuildUserAgent(t *testing.T) {
	tests := []struct {
		name        string
		params      EventParams
		wantKeys    []string // keys that must be present
		notWantKeys []string // keys that must NOT be present (when field is empty)
		maxLen      int
	}{
		{
			name: "minimal_required_fields",
			params: EventParams{
				Cmd:    "install",
				StepID: "inv",
				Mode:   ModeTTY,
			},
			wantKeys:    []string{"dtwiz/", "c=ins", "st=inv"},
			notWantKeys: []string{";s=", ";er=", ";t="},
			maxLen:      64,
		},
		{
			name: "with_subcommand",
			params: EventParams{
				Cmd:    "install",
				Sub:    "otel",
				StepID: "inv",
				Mode:   ModeDebug,
			},
			wantKeys:    []string{"c=ins", "st=inv", "s=otel"},
			notWantKeys: []string{";er=", ";t="},
			maxLen:      64,
		},
		{
			name: "with_error",
			params: EventParams{
				Cmd:    "uninstall",
				Sub:    "kubernetes",
				StepID: "inv",
				Mode:   ModeNonTTY,
				Err:    "timeout",
			},
			wantKeys:    []string{"c=uni", "s=k8s", "er=timeout"},
			notWantKeys: []string{";t="},
			maxLen:      64,
		},
		{
			name: "with_all_fields",
			params: EventParams{
				Cmd:    "update",
				Sub:    "otel",
				StepID: StepSnapshot,
				Mode:   ModeDebug,
				Err:    "net",
				Type:   "retry",
			},
			wantKeys:    []string{"c=upd", "st=snp", "s=otel", "er=net", "t=retry"},
			notWantKeys: []string{";kd=", ";cp=", ";opt="},
			maxLen:      64,
		},
		{
			name: "with_k8s_distro_in_analyze_event",
			params: EventParams{
				Cmd:       "setup",
				StepID:    StepAnalyze,
				Mode:      ModeTTY,
				K8sDistro: "GKE-Autopilot",
			},
			wantKeys:    []string{"c=set", "st=ana", "kd=gka"},
			notWantKeys: []string{"GKE-Autopilot"}, // full name must not appear in header
			maxLen:      64,
		},
		{
			name: "k8s_distro_absent_from_install_event",
			params: EventParams{
				Cmd:    "install",
				Sub:    "kubernetes",
				StepID: StepInstall,
				Mode:   ModeTTY,
			},
			wantKeys:    []string{"s=k8s"},
			notWantKeys: []string{";kd="},
			maxLen:      64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ua := buildUserAgent(tt.params)

			// Check length
			if len(ua) > tt.maxLen {
				t.Errorf("User-Agent too long: %d > %d: %s", len(ua), tt.maxLen, ua)
			}

			// Check all required keys present
			for _, key := range tt.wantKeys {
				if !strings.Contains(ua, key) {
					t.Errorf("missing key %q in User-Agent: %s", key, ua)
				}
			}

			// Ensure fields are omitted when empty
			for _, key := range tt.notWantKeys {
				if strings.Contains(ua, key) {
					t.Errorf("key %q should be omitted, but found in: %s", key, ua)
				}
			}
		})
	}
}

func TestBuildTabID(t *testing.T) {
	tests := []struct {
		name     string
		params   EventParams
		modeWant string
		maxLen   int
	}{
		{
			name: "debug_mode",
			params: EventParams{
				Mode: ModeDebug,
			},
			modeWant: "m=deb",
			maxLen:   16,
		},
		{
			name: "tty_mode",
			params: EventParams{
				Mode: ModeTTY,
			},
			modeWant: "m=tty",
			maxLen:   16,
		},
		{
			name: "ntt_mode",
			params: EventParams{
				Mode: ModeNonTTY,
			},
			modeWant: "m=ntt",
			maxLen:   16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tabID := buildTabID(tt.params)

			// Check length
			if len(tabID) > tt.maxLen {
				t.Errorf("Tab-Id too long: %d > %d: %s", len(tabID), tt.maxLen, tabID)
			}

			// Check format: <3hex>;m=<mode>;o=<os>
			if len(tabID) < 6 {
				t.Errorf("Tab-Id malformed (too short): %s", tabID)
			}
			if tabID[3] != ';' {
				t.Errorf("Tab-Id format error: expected ';' at position 3, got %q in %s", tabID[3], tabID)
			}

			// Check mode is present
			if !strings.Contains(tabID, tt.modeWant) {
				t.Errorf("missing %q in Tab-Id: %s", tt.modeWant, tabID)
			}

			// Check OS is present (3 chars: mac, lin, or win)
			if !strings.Contains(tabID, "o=") {
				t.Errorf("missing OS code in Tab-Id: %s", tabID)
			}

			// Parse and validate OS code
			parts := strings.Split(tabID, ";")
			if len(parts) != 3 {
				t.Errorf("Tab-Id should have 3 parts separated by ';', got %d: %s", len(parts), tabID)
			}
		})
	}
}

// The User-Agent is truncated by HAProxy at 64 chars, so the worst-case combination of
// version, command, subcommand, and error must still fit. Release versions are short
// ("1.8.1"), but goreleaser snapshots and preview builds append suffixes, so the budget
// below is deliberately generous — without abbreviating the error this test fails.
func TestBuildUserAgentWithinCaptureLimit(t *testing.T) {
	const (
		limit            = 64
		maxVersionLength = 24
	)

	origVersion := version.Version
	defer func() { version.Version = origVersion }()
	version.Version = strings.Repeat("v", maxVersionLength)

	longestCmd := longestKey(cmdShortMap)
	longestSub := longestKey(subShortMap)
	longestDistro := longestKey(distroShortMap)
	// Worst-case cloud provider: all three detected, comma-separated abbreviated form.
	// analyzeSystem never returns an error, so er= never appears in analyze events.
	longestCloud := "aws,az,gcp"

	for errType := range errShortMap {
		// Install/uninstall events: have subcommand + error, never cloud provider or k8s distro.
		ua := buildUserAgent(EventParams{
			Cmd:    longestCmd,
			Sub:    longestSub,
			StepID: StepFailed,
			Err:    errType,
		})
		if len(ua) > limit {
			t.Errorf("install User-Agent %q is %d chars, exceeds %d-char limit", ua, len(ua), limit)
		}
	}

	// Analyze events: have kd= + cp= but never er= (analyzeSystem never fails).
	ua := buildUserAgent(EventParams{
		Cmd:           longestCmd,
		StepID:        StepAnalyze,
		K8sDistro:     longestDistro,
		CloudProvider: longestCloud,
	})
	if len(ua) > limit {
		t.Errorf("analyze User-Agent %q is %d chars, exceeds %d-char limit", ua, len(ua), limit)
	}
}

func TestBuildUserAgentOpt(t *testing.T) {
	tests := []struct {
		name        string
		params      EventParams
		wantKeys    []string
		notWantKeys []string
	}{
		{
			name: "opt_set_appears_as_opt_not_s",
			params: EventParams{
				Cmd:    "setup",
				StepID: StepRecommendationsPresented,
				Mode:   ModeTTY,
				Opt:    "otel",
			},
			wantKeys:    []string{"c=set", "st=rpr", "opt=otel"},
			notWantKeys: []string{";s=otel"},
		},
		{
			name: "step_selected",
			params: EventParams{
				Cmd:    "setup",
				StepID: StepRecommendationsSelected,
				Mode:   ModeTTY,
				Opt:    "kubernetes",
			},
			wantKeys:    []string{"st=rsl", "opt=k8s"},
			notWantKeys: []string{},
		},
		{
			name: "unmapped_opt_passes_through",
			params: EventParams{
				Cmd:    "setup",
				StepID: StepRecommendationsPresented,
				Mode:   ModeTTY,
				Opt:    "aws",
			},
			wantKeys:    []string{"st=rpr", "opt=aws"},
			notWantKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ua := buildUserAgent(tt.params)
			if len(ua) > 64 {
				t.Errorf("User-Agent too long: %d > 64: %s", len(ua), ua)
			}
			for _, key := range tt.wantKeys {
				if !strings.Contains(ua, key) {
					t.Errorf("missing key %q in User-Agent: %s", key, ua)
				}
			}
			for _, key := range tt.notWantKeys {
				if strings.Contains(ua, key) {
					t.Errorf("key %q should be absent in User-Agent: %s", key, ua)
				}
			}
		})
	}
}

// longestKey returns the key whose abbreviated value is longest, breaking ties by key length.
func longestKey(m map[string]string) string {
	var best string
	for k, v := range m {
		switch {
		case len(v) > len(m[best]):
			best = k
		case len(v) == len(m[best]) && len(k) > len(best):
			best = k
		}
	}
	return best
}

// The watch snapshot event is the longest User-Agent dtwiz produces: it carries a
// t= field with one clamped 3-digit value per signal type. This pins the budget left
// over for the version string, so anything that lengthens the header (another field,
// a wider clamp, more signal types) fails here instead of truncating the tail of t=
// at the HAProxy capture limit.
//
// NOTE: 11 chars is exactly what goreleaser's "{{ incpatch .Version }}-next" snapshot
// template produces today ("1.10.1-next"). There is no slack: a two-digit patch number
// ("1.10.10-next", 12 chars) overflows. That cliff predates the snapshot event, because
// the completion event it replaced encoded the same 8 three-digit values.
func TestWatchSnapshotUserAgentVersionBudget(t *testing.T) {
	const (
		limit         = 64
		wantVersionCS = 11
	)

	origVersion := version.Version
	defer func() { version.Version = origVersion }()
	version.Version = ""

	// Worst case: all 8 signal types seen, every value clamped to 3 digits.
	// Mirrors cmd.watchSignalCSV output at cmd.watchSignalSecondsCap.
	worstCaseType := strings.TrimSuffix(strings.Repeat("999,", 8), ",")

	overhead := len(buildUserAgent(EventParams{
		Cmd:    "watch",
		StepID: StepSnapshot,
		Type:   worstCaseType,
	}))

	if budget := limit - overhead; budget < wantVersionCS {
		t.Errorf("version budget is %d chars, want at least %d; worst-case header overhead grew to %d of the %d-char limit",
			budget, wantVersionCS, overhead, limit)
	}
}
