package aws

import "testing"

func TestMaskTokenArgs(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		tokens []string
		want   []string
	}{
		{
			name:   "token value is fully masked, not partially revealed",
			args:   []string{"pDtApiToken=dt0s16.abcdefghijklmnop"},
			tokens: []string{"dt0s16.abcdefghijklmnop"},
			want:   []string{"pDtApiToken=***"},
		},
		{
			name:   "short token value is still fully masked",
			args:   []string{"pDtIngestToken=short"},
			tokens: []string{"short"},
			want:   []string{"pDtIngestToken=***"},
		},
		{
			name:   "multiple distinct tokens are each masked",
			args:   []string{"pDtApiToken=tokenA pDtIngestToken=tokenB"},
			tokens: []string{"tokenA", "tokenB"},
			want:   []string{"pDtApiToken=*** pDtIngestToken=***"},
		},
		{
			name:   "args without a matching token are untouched",
			args:   []string{"--stack-name", "dynatrace-data-acquisition"},
			tokens: []string{"tokenA"},
			want:   []string{"--stack-name", "dynatrace-data-acquisition"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskTokenArgs(tt.args, tt.tokens...)
			if len(got) != len(tt.want) {
				t.Fatalf("maskTokenArgs(%v, %v) = %v, want %v", tt.args, tt.tokens, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("maskTokenArgs(%v, %v)[%d] = %q, want %q", tt.args, tt.tokens, i, got[i], tt.want[i])
				}
			}
		})
	}
}
