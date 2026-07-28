package aws

import "testing"

func TestMaskTokenArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "long token is truncated to 10 chars",
			args: []string{"pDtApiToken=dt0s16.abcdefghijklmnop"},
			want: []string{"pDtApiToken=dt0s16.abc***"},
		},
		{
			name: "short token is still masked, not left in the clear",
			args: []string{"pDtIngestToken=short"},
			want: []string{"pDtIngestToken=short***"},
		},
		{
			name: "empty token value is still masked",
			args: []string{"pDtApiToken="},
			want: []string{"pDtApiToken=***"},
		},
		{
			name: "non-token args are untouched",
			args: []string{"--stack-name", "dynatrace-data-acquisition"},
			want: []string{"--stack-name", "dynatrace-data-acquisition"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskTokenArgs(tt.args)
			if len(got) != len(tt.want) {
				t.Fatalf("maskTokenArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("maskTokenArgs(%v)[%d] = %q, want %q", tt.args, i, got[i], tt.want[i])
				}
			}
		})
	}
}
