package installer

import (
	"errors"
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		secret string
		want   string
	}{
		{name: "empty secret leaves input unchanged", input: "abc", secret: "", want: "abc"},
		{name: "single occurrence", input: "token=secret", secret: "secret", want: "token=***"},
		{name: "multiple occurrences", input: "secret and secret", secret: "secret", want: "*** and ***"},
		{name: "no occurrence", input: "visible", secret: "secret", want: "visible"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := MaskSecret(tt.input, tt.secret); got != tt.want {
				t.Fatalf("MaskSecret() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuthHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		token string
		want  string
	}{
		{"dt0c01.abc123.secret", "Api-Token dt0c01.abc123.secret"},
		{"dt0s16.abc123.secret", "Bearer dt0s16.abc123.secret"},
		{"some-oauth-token", "Bearer some-oauth-token"},
		{"", "Bearer "},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.token, func(t *testing.T) {
			t.Parallel()

			got := AuthHeader(tt.token)
			if got != tt.want {
				t.Errorf("AuthHeader(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestAPIURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		// apps → classic conversion
		{"https://abc123.apps.dynatrace.com", "https://abc123.live.dynatrace.com"},
		{"https://abc123.apps.dynatracelabs.com", "https://abc123.dynatracelabs.com"},
		{"https://abc123.dev.apps.dynatracelabs.com", "https://abc123.dev.dynatracelabs.com"},
		// already-classic URLs returned unchanged
		{"https://abc123.live.dynatrace.com", "https://abc123.live.dynatrace.com"},
		{"https://abc123.dynatracelabs.com", "https://abc123.dynatracelabs.com"},
		{"https://abc123.dev.dynatracelabs.com", "https://abc123.dev.dynatracelabs.com"},
		// trailing slashes stripped
		{"https://abc123.apps.dynatrace.com/", "https://abc123.live.dynatrace.com"},
		{"https://abc123.live.dynatrace.com/", "https://abc123.live.dynatrace.com"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := APIURL(tt.input)
			if got != tt.want {
				t.Errorf("APIURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAppsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"https://abc123.live.dynatrace.com", "https://abc123.apps.dynatrace.com"},
		{"https://abc123.dynatracelabs.com", "https://abc123.apps.dynatracelabs.com"},
		{"https://abc123.dev.dynatracelabs.com", "https://abc123.dev.apps.dynatracelabs.com"},
		{"https://abc123.apps.dynatrace.com", "https://abc123.apps.dynatrace.com"},
		{"https://abc123.apps.dynatracelabs.com", "https://abc123.apps.dynatracelabs.com"},
		{"https://abc123.live.dynatrace.com/", "https://abc123.apps.dynatrace.com"},
		{"https://custom.example.com", "https://custom.example.com"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := AppsURL(tt.input)
			if got != tt.want {
				t.Errorf("AppsURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractTenantID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"https://abc12345.live.dynatrace.com", "abc12345"},
		{"https://abc12345.apps.dynatrace.com", "abc12345"},
		{"https://fxz0998d.dev.dynatracelabs.com", "fxz0998d"},
		{"https://fxz0998d.dev.apps.dynatracelabs.com", "fxz0998d"},
		{"abc12345.live.dynatrace.com", "abc12345"},
		// Managed URL with /e/<id>
		{"https://my-managed.example.com/e/abc12345", "abc12345"},
		{"https://my-managed.example.com/e/abc12345/", "abc12345"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := ExtractTenantID(tt.input)
			if got != tt.want {
				t.Errorf("ExtractTenantID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIngestTimeFormat_IsRFC3339(t *testing.T) {
	t.Parallel()

	// Go's reference time is Mon Jan 2 15:04:05 MST 2006 → RFC3339: "2006-01-02T15:04:05Z"
	const wantRFC3339 = "2006-01-02T15:04:05Z"
	if IngestTimeFormat != wantRFC3339 {
		t.Errorf("IngestTimeFormat = %q, want RFC3339 reference value %q", IngestTimeFormat, wantRFC3339)
	}
}

func TestRunCommandWithExitCode_EmptyCommand(t *testing.T) {
	t.Parallel()

	code, err := RunCommandWithExitCode(nil, true)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "empty command") {
		t.Fatalf("error = %v, want empty command", err)
	}
}

func TestIsNotFoundErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg  string
		want bool
	}{
		{msg: "resource not found", want: true},
		{msg: "The resource does not exist", want: true},
		{msg: "NOT_FOUND", want: true},
		{msg: "permission denied", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			t.Parallel()

			if got := IsNotFoundErr(errors.New(tt.msg)); got != tt.want {
				t.Fatalf("IsNotFoundErr(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

// ── ShouldProceed ─────────────────────────────────────────────────────────────

func TestShouldProceed_DryRunReturnsFalseNoError(t *testing.T) {
	proceed, err := ShouldProceed(true, "Installation")
	if proceed {
		t.Error("dry-run must return proceed=false")
	}
	if err != nil {
		t.Errorf("dry-run must return nil error, got %v", err)
	}
}

func TestShouldProceed_AutoConfirmProceeds(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = true
	defer func() { AutoConfirm = old }()

	var proceed bool
	var err error
	withStdin(t, "", func() {
		proceed, err = ShouldProceed(false, "Installation")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !proceed {
		t.Error("AutoConfirm=true must return proceed=true")
	}
}

func TestShouldProceed_UserAccepts(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()

	var proceed bool
	var err error
	withStdin(t, "y\n", func() {
		proceed, err = ShouldProceed(false, "Update")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !proceed {
		t.Error("user confirming with 'y' must return proceed=true")
	}
}

func TestShouldProceed_UserDeclines(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()

	var proceed bool
	var err error
	withStdin(t, "n\n", func() {
		proceed, err = ShouldProceed(false, "Uninstall")
	})
	if proceed {
		t.Error("user declining must return proceed=false")
	}
	if !errors.Is(err, ErrInstallCancelled) {
		t.Errorf("user declining must return ErrInstallCancelled, got %v", err)
	}
}

func TestMergePathEntries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		desc    string
		current string
		newPath string
		want    string
	}{
		{
			desc:    "new entry appended",
			current: `C:\Windows;C:\Windows\system32`,
			newPath: `C:\Program Files\Helm`,
			want:    `C:\Windows;C:\Windows\system32;C:\Program Files\Helm`,
		},
		{
			desc:    "duplicate not added (case-insensitive)",
			current: `C:\Windows;C:\Program Files\Helm`,
			newPath: `C:\program files\helm`,
			want:    `C:\Windows;C:\Program Files\Helm`,
		},
		{
			desc:    "only new entries appended from mixed list",
			current: `C:\Windows`,
			newPath: `C:\Windows;C:\Program Files\Helm`,
			want:    `C:\Windows;C:\Program Files\Helm`,
		},
		{
			desc:    "empty new path returns current unchanged",
			current: `C:\Windows`,
			newPath: "",
			want:    `C:\Windows`,
		},
		{
			desc:    "empty current path returns new entries without leading separator",
			current: "",
			newPath: `C:\Program Files\Helm;C:\Program Files\Python`,
			want:    `C:\Program Files\Helm;C:\Program Files\Python`,
		},
		{
			desc:    "whitespace around entries is trimmed to avoid duplicates",
			current: `C:\Tools`,
			newPath: `C:\Tools `,
			want:    `C:\Tools`,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.desc, func(t *testing.T) {
			t.Parallel()

			got := mergePathEntries(c.current, c.newPath)
			if got != c.want {
				t.Errorf("mergePathEntries(%q, %q) = %q, want %q", c.current, c.newPath, got, c.want)
			}
		})
	}
}
