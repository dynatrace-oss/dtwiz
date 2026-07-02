package analyzer

import "testing"

func TestCleanGCloudConfigValue(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanGCloudConfigValue(tt.out); got != tt.want {
				t.Fatalf("CleanGCloudConfigValue() = %q, want %q", got, tt.want)
			}
		})
	}
}
