package analyzer

import "testing"

func TestCleanGCloudConfigValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		out  string
		want string
	}{
		{
			name: "plain value",
			out:  "dev-project\n",
			want: "dev-project",
		},
		{
			name: "cloud shell active configuration notice",
			out:  "Your active configuration is: [cloudshell-12297]\ndev-plg---dieter-mayrhofer\n",
			want: "dev-plg---dieter-mayrhofer",
		},
		{
			name: "unset with active configuration notice",
			out:  "Your active configuration is: [cloudshell-12297]\n(unset)\n",
			want: "(unset)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := CleanGCloudConfigValue(tt.out); got != tt.want {
				t.Fatalf("CleanGCloudConfigValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseGCPJSONArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		out  string
		want int
	}{
		{name: "empty output", out: "", want: 0},
		{name: "empty array", out: "[]", want: 0},
		{name: "array with objects", out: `[{"name":"vm-a"},{"name":"vm-b"}]`, want: 2},
		{name: "array with scalars", out: `["bucket-a","bucket-b","bucket-c"]`, want: 3},
		{name: "invalid json", out: `not-json`, want: 0},
		{name: "non-array json", out: `{"name":"vm-a"}`, want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseGCPJSONArray(tt.out); got != tt.want {
				t.Fatalf("parseGCPJSONArray() = %d, want %d", got, tt.want)
			}
		})
	}
}
