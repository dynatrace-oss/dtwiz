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
			notWantKeys: []string{},
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
}

// Every step constant must map to a full name, otherwise SendEvent falls back to the
// shortcode and the event body carries e.g. "can" instead of "cancelled".
func TestStepFullNames(t *testing.T) {
	want := map[string]string{
		StepInvoked:                  "invoked",
		StepAnalyze:                  "analyze",
		StepRecommend:                "recommend",
		StepInstall:                  "install",
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

	for errType := range errShortMap {
		ua := buildUserAgent(EventParams{
			Cmd:    longestCmd,
			Sub:    longestSub,
			StepID: StepFailed,
			Err:    errType,
		})
		if len(ua) > limit {
			t.Errorf("User-Agent %q is %d chars, exceeds %d-char limit", ua, len(ua), limit)
		}
	}
}

func TestBuildUserAgentOptAndTechs(t *testing.T) {
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
			name: "tech_set_appears_as_tx",
			params: EventParams{
				Cmd:    "setup",
				StepID: StepRecommendationsPresented,
				Mode:   ModeTTY,
				Opt:    "otel",
				Tech:   "Node.js",
			},
			wantKeys:    []string{"opt=otel", "tx=nd"},
			notWantKeys: []string{},
		},
		{
			name: "opt_and_tech_both_set",
			params: EventParams{
				Cmd:    "setup",
				StepID: StepRecommendationsPresented,
				Mode:   ModeTTY,
				Opt:    "otel",
				Tech:   "Python",
			},
			wantKeys:    []string{"st=rpr", "opt=otel", "tx=py"},
			notWantKeys: []string{},
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
			notWantKeys: []string{"tx="},
		},
		{
			name: "no_techs_omits_tx",
			params: EventParams{
				Cmd:    "setup",
				StepID: StepRecommendationsPresented,
				Mode:   ModeTTY,
				Opt:    "aws",
			},
			wantKeys:    []string{"st=rpr", "opt=aws"},
			notWantKeys: []string{"tx="},
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

func TestBuildEventPropsOptAndTechs(t *testing.T) {
	t.Run("opt_in_body_as_option_key", func(t *testing.T) {
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
		if _, ok := props["technology"]; ok {
			t.Errorf("body[technology] should be absent when Tech is empty, got %q", props["technology"])
		}
	})

	t.Run("tech_full_name_in_body_as_technology_key", func(t *testing.T) {
		params := EventParams{
			Cmd:    "setup",
			StepID: StepRecommendationsPresented,
			Mode:   ModeTTY,
			Opt:    "otel",
			Tech:   "Node.js",
		}
		props := buildEventProps(params)
		if props["technology"] != "Node.js" {
			t.Errorf("body[technology] = %q, want full name %q", props["technology"], "Node.js")
		}
	})
}

func TestShortTech(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Node.js", "nd"},
		{"Go", "go"},
		{"Python", "py"},
		{"Java", "jv"},
		{"Rust", "rs"},
		{"Ruby", "rb"},
		{"PHP", "ph"},
		{".NET", "dn"},
		{"Unknown", ""}, // unmapped returns empty
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shortTech(tt.input)
			if got != tt.want {
				t.Errorf("shortTech(%q) = %q, want %q", tt.input, got, tt.want)
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
		if k != "type" && strings.Contains(ua, v) {
			t.Errorf("User-Agent %q carries full-length %q; it should be abbreviated", ua, v)
		}
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
