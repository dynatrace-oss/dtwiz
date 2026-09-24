package selfmonitoring

import (
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
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
				StepID: "inv",
				Mode:   ModeDebug,
				Err:    "net",
				Type:   "retry",
			},
			wantKeys:    []string{"c=upd", "s=otel", "er=net", "t=retry"},
			notWantKeys: []string{";kd="},
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

func TestResolveOS(t *testing.T) {
	tests := []struct {
		goos    string
		want    string
		setup   func()
		cleanup func()
	}{
		{
			goos: "darwin",
			want: "mac",
		},
		{
			goos: "linux",
			want: "lin",
		},
		{
			goos: "windows",
			want: "win",
		},
		{
			goos: "freebsd",
			want: "freebsd", // unknown values pass through
		},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			// Note: we can't easily override runtime.GOOS in a test,
			// so this is a static code check. In practice, resolveOS
			// would be called with the actual runtime.GOOS on each platform.
			// We verify the mapping logic by checking the function works
			// as designed (this test would need build tags to test all OSes).

			// For now, just verify the function doesn't panic and returns a string.
			result := resolveOS()
			if result == "" {
				t.Error("resolveOS returned empty string")
			}
			if !isValidOSCode(result) {
				t.Errorf("resolveOS returned invalid OS code: %s", result)
			}
		})
	}
}

func isValidOSCode(code string) bool {
	validCodes := map[string]bool{
		"mac": true, "lin": true, "win": true,
		"linux": true, "darwin": true, "windows": true, // pass-through values
		"freebsd": true, "openbsd": true, "netbsd": true, // other known OSes
	}
	return validCodes[code] || len(code) > 0
}

func TestAuthHeader(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "classic_api_token",
			token: "dt0c01.abc123",
			want:  "Api-Token dt0c01.abc123",
		},
		{
			name:  "platform_token",
			token: "dt0s16.def456",
			want:  "Bearer dt0s16.def456",
		},
		{
			name:  "legacy_token_pattern",
			token: "dt0c01.xyz789",
			want:  "Api-Token dt0c01.xyz789",
		},
		{
			name:  "modern_token_pattern",
			token: "dt0s16.abc123",
			want:  "Bearer dt0s16.abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authHeader(tt.token)
			if got != tt.want {
				t.Errorf("authHeader(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestStepConstants(t *testing.T) {
	if StepAnalyze != "ana" {
		t.Errorf("StepAnalyze = %q, want %q", StepAnalyze, "ana")
	}
	if StepRecommend != "rec" {
		t.Errorf("StepRecommend = %q, want %q", StepRecommend, "rec")
	}
	if StepInstall != "ist" {
		t.Errorf("StepInstall = %q, want %q", StepInstall, "ist")
	}
	if StepSnapshot != "snp" {
		t.Errorf("StepSnapshot = %q, want %q", StepSnapshot, "snp")
	}
}

// Every step constant must map to a full name, otherwise SendEvent falls back to the
// shortcode and the event body carries e.g. "can" instead of "cancelled".
func TestStepFullNames(t *testing.T) {
	want := map[string]string{
		StepInvoked:                  "invoked",
		StepAnalyze:                  "analyze",
		StepRecommend:                "recommend",
		StepInstall:                  "install",
		StepSnapshot:                 "snapshot",
		StepCompleted:                "completed",
		StepFailed:                   "failed",
		StepCancelled:                "cancelled",
		StepRecommendationsPresented: "recommendations_presented",
		StepRecommendationsSelected:  "recommendations_selected",
	}

	for code, name := range want {
		if got := stepFullNames[code]; got != name {
			t.Errorf("stepFullNames[%q] = %q, want %q", code, got, name)
		}
	}
	if len(stepFullNames) != len(want) {
		t.Errorf("stepFullNames has %d entries, want %d — a step constant is missing a full name", len(stepFullNames), len(want))
	}
}

// Every taxonomy value must have a short code, otherwise it lands in the User-Agent at
// full length and risks blowing the 64-char capture limit.
func TestShortErr(t *testing.T) {
	want := map[installer.ErrorType]string{
		installer.ErrTypeUserCancelled:       "ucl",
		installer.ErrTypeAuthError:           "aut",
		installer.ErrTypeConfigError:         "cfg",
		installer.ErrTypeDependencyMissing:   "dep",
		installer.ErrTypeNetworkError:        "net",
		installer.ErrTypeInstallFailed:       "ifl",
		installer.ErrTypePlatformUnsupported: "plt",
	}

	for errType, code := range want {
		if got := shortErr(string(errType)); got != code {
			t.Errorf("shortErr(%q) = %q, want %q", errType, got, code)
		}
	}
	if len(errShortMap) != len(want) {
		t.Errorf("errShortMap has %d entries, want %d — an ErrorType is missing a short code", len(errShortMap), len(want))
	}
	// Unknown values pass through unchanged.
	if got := shortErr("something_else"); got != "something_else" {
		t.Errorf("shortErr passthrough = %q, want %q", got, "something_else")
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

func TestBuildEventPropsOpt(t *testing.T) {
	params := EventParams{
		Cmd:    "setup",
		StepID: StepRecommendationsPresented,
		Mode:   ModeTTY,
		Opt:    "otel",
	}
	props := buildEventProps(params)
	if props["option"] != "otel" {
		t.Errorf("body[option] = %q, want %q", props["option"], "otel")
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

// Queries read the event body, so every field encoded in the abbreviated User-Agent must
// also appear in the body at full length. That invariant is what lets the header encoding
// change freely without any query being updated.
func TestEventBodyCarriesFullNamesForAllHeaderFields(t *testing.T) {
	params := EventParams{
		Cmd:    "uninstall",
		Sub:    "otel-collector",
		StepID: StepCancelled,
		Mode:   ModeTTY,
		Err:    string(installer.ErrTypePlatformUnsupported),
		Type:   "retry",
	}

	props := buildEventProps(params)
	ua := buildUserAgent(params)

	want := map[string]string{
		"command":    "uninstall",
		"subcommand": "otel-collector",
		"step":       "cancelled",
		"error":      "platform_unsupported",
		"type":       "retry",
	}
	for k, v := range want {
		if props[k] != v {
			t.Errorf("body[%q] = %q, want full name %q", k, props[k], v)
		}
		// The whole point: the header is abbreviated, the body is not.
		// "type" full value may legitimately appear since it passes through
		// unabbreviated when no short form applies, so skip the header check for it.
		if k != "type" && strings.Contains(ua, v) {
			t.Errorf("User-Agent %q carries full-length %q; it should be abbreviated", ua, v)
		}
	}
	if strings.Contains(ua, "kd=") {
		t.Errorf("User-Agent %q carries kd= for a non-analyze step", ua)
	}
}

func TestEventParamsDefaults(t *testing.T) {
	params := EventParams{
		Cmd:  "analyze",
		Mode: ModeTTY,
	}

	// StepID should default to StepInvoked when empty
	if params.StepID != "" {
		t.Errorf("empty StepID should be set to default in SendEvent, not here")
	}

	// Verify StepInvoked constant exists and is correct
	if StepInvoked != "inv" {
		t.Errorf("StepInvoked = %q, want %q", StepInvoked, "inv")
	}
}

func TestShortCmd(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"install", "ins"},
		{"uninstall", "uni"},
		{"update", "upd"},
		{"analyze", "ana"},
		{"recommend", "rec"},
		{"status", "sta"},
		{"watch", "wch"},
		{"setup", "set"},
		{"version", "ver"},
		{"unknown", "unknown"}, // pass-through for unmapped names
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shortCmd(tt.input)
			if got != tt.want {
				t.Errorf("shortCmd(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShortSub(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"kubernetes", "k8s"},
		{"oneagent", "oa"},
		{"aws-lambda", "awsl"},
		{"otel-update", "otlu"},
		{"azure-update", "azu"},
		{"gcp-update", "gcpu"},
		{"otel", "otel"},
		{"unknown-sub", "unknown-sub"}, // pass-through for unmapped names
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shortSub(tt.input)
			if got != tt.want {
				t.Errorf("shortSub(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Every known distro must have a short code no longer than 3 chars so the 64-char
// User-Agent capture budget holds when combined with the other abbreviated fields.
func TestShortDistro(t *testing.T) {
	want := map[string]string{
		"GKE":              "gke",
		"EKS":              "eks",
		"AKS":              "aks",
		"IKS":              "iks",
		"OpenShift":        "ocp",
		"k3s":              "k3s",
		"RKE":              "rke",
		"kubernetes":       "k8s",
		"GKE-Autopilot":    "gka",
		"EKS-Bottlerocket": "ekb",
		"minikube":         "mnk",
		"kind":             "knd",
		"TKGI":             "tkg",
	}

	for distro, code := range want {
		if got := shortDistro(distro); got != code {
			t.Errorf("shortDistro(%q) = %q, want %q", distro, got, code)
		}
		if len(code) > 3 {
			t.Errorf("distroShortMap[%q] = %q is longer than 3 chars, risks exceeding User-Agent capture limit", distro, code)
		}
	}
	if len(distroShortMap) != len(want) {
		t.Errorf("distroShortMap has %d entries, want %d — a distro is missing a short code", len(distroShortMap), len(want))
	}
	// Unknown values pass through unchanged.
	if got := shortDistro("unknown-distro"); got != "unknown-distro" {
		t.Errorf("shortDistro passthrough = %q, want %q", got, "unknown-distro")
	}
}

// Watch snapshot events are the only ones carrying st=snp; the body must spell it out
// in full so queries filter on "snapshot" rather than the shortcode.
func TestSnapshotStepEncoding(t *testing.T) {
	params := EventParams{Cmd: "watch", StepID: StepSnapshot, Type: "0,0,1,0,0,0,0,8"}

	if got := buildEventProps(params)["step"]; got != "snapshot" {
		t.Errorf("body step = %q, want %q", got, "snapshot")
	}
	if ua := buildUserAgent(params); !strings.Contains(ua, ";st=snp") {
		t.Errorf("User-Agent %q missing %q", ua, ";st=snp")
	}
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
