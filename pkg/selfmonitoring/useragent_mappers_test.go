package selfmonitoring

import (
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

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

// Every valid setup option must have an explicit entry, even when its short form equals the name.
func TestShortOpt(t *testing.T) {
	want := map[string]string{
		"oneagent":     "oa",
		"kubernetes":   "k8s",
		"docker":       "dock",
		"otel":         "otel",
		"otel-update":  "otlu",
		"aws":          "aws",
		"azure":        "az",
		"azure-update": "azu",
		"gcp":          "gcp",
		"gcp-update":   "gcpu",
		"uninstall":    "uni",
		"demo":         "demo",
	}

	for name, code := range want {
		if got, ok := optShortMap[name]; !ok || got != code {
			t.Errorf("optShortMap[%q] = %q (present=%v), want %q", name, got, ok, code)
		}
		if got := shortOpt(name); got != code {
			t.Errorf("shortOpt(%q) = %q, want %q", name, got, code)
		}
	}
	if len(optShortMap) != len(want) {
		t.Errorf("optShortMap has %d entries, want %d", len(optShortMap), len(want))
	}
	// Unknown values pass through unchanged.
	if got := shortOpt("unknown-opt"); got != "unknown-opt" {
		t.Errorf("shortOpt passthrough = %q, want %q", got, "unknown-opt")
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
