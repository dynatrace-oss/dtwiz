package cmd

import "testing"

func TestNormSubUpdateVariants(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"otel-update", "otlu"},
		{"azure-update", "azu"},
		{"gcp-update", "gcpu"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normSub(tt.input)
			if got != tt.want {
				t.Errorf("normSub(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
