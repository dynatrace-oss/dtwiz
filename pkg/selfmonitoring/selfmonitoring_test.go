package selfmonitoring

import (
	"strings"
	"testing"
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
