package installer

import "testing"

func TestAuthHeader(t *testing.T) {
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
		got := AuthHeader(tt.token)
		if got != tt.want {
			t.Errorf("AuthHeader(%q) = %q, want %q", tt.token, got, tt.want)
		}
	}
}

func TestClassicAPIURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://abc123.apps.dynatrace.com", "https://abc123.live.dynatrace.com"},
		{"https://abc123.apps.dynatracelabs.com", "https://abc123.dynatracelabs.com"},
		{"https://abc123.live.dynatrace.com", "https://abc123.live.dynatrace.com"},
		{"https://abc123.dev.dynatracelabs.com", "https://abc123.dev.dynatracelabs.com"},
	}
	for _, tt := range tests {
		got := ClassicAPIURL(tt.input)
		if got != tt.want {
			t.Errorf("ClassicAPIURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAPIURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://abc123.apps.dynatrace.com", "https://abc123.live.dynatrace.com"},
		{"https://abc123.apps.dynatracelabs.com", "https://abc123.dynatracelabs.com"},
		{"https://abc123.live.dynatrace.com/", "https://abc123.live.dynatrace.com"},
		{"https://abc123.dev.dynatracelabs.com/", "https://abc123.dev.dynatracelabs.com"},
	}
	for _, tt := range tests {
		got := APIURL(tt.input)
		if got != tt.want {
			t.Errorf("APIURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAppsURL(t *testing.T) {
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
		got := AppsURL(tt.input)
		if got != tt.want {
			t.Errorf("AppsURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractTenantID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://abc12345.live.dynatrace.com", "abc12345"},
		{"https://abc12345.apps.dynatrace.com", "abc12345"},
		{"https://fxz0998d.dev.dynatracelabs.com", "fxz0998d"},
		{"https://fxz0998d.dev.apps.dynatracelabs.com", "fxz0998d"},
		{"abc12345.live.dynatrace.com", "abc12345"},
	}
	for _, tt := range tests {
		got := ExtractTenantID(tt.input)
		if got != tt.want {
			t.Errorf("ExtractTenantID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
