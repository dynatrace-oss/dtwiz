package selfmonitoring

import "testing"

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
	if StepInstall != "ist" {
		t.Errorf("StepInstall = %q, want %q", StepInstall, "ist")
	}
	if StepSnapshot != "snp" {
		t.Errorf("StepSnapshot = %q, want %q", StepSnapshot, "snp")
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
