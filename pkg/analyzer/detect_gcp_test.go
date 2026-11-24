package analyzer

import (
	"errors"
	"testing"
	"time"
)

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

// fakeGcloudReads answers the two gcloud reads detectGCPWith makes and records them.
type fakeGcloudReads struct {
	accountOut string
	accountErr error
	projectOut string
	projectErr error
	reads      []string
}

func (f *fakeGcloudReads) run(_ time.Duration, _ string, args ...string) (string, error) {
	switch {
	case len(args) >= 2 && args[0] == "auth" && args[1] == "list":
		f.reads = append(f.reads, "account")
		return f.accountOut, f.accountErr
	case len(args) >= 3 && args[0] == "config" && args[2] == "project":
		f.reads = append(f.reads, "project")
		return f.projectOut, f.projectErr
	}
	return "", errors.New("unexpected gcloud call")
}

func TestDetectGCPWith(t *testing.T) {
	tests := []struct {
		name              string
		fake              fakeGcloudReads
		wantAuthenticated bool
		wantAvailable     bool
		wantAccount       string
		wantProject       string
		wantProjectRead   bool
	}{
		{
			name: "gcloud missing",
			fake: fakeGcloudReads{accountErr: errors.New("executable file not found")},
		},
		{
			// When logged out, gcloud writes nothing to stdout and only a filter
			// warning to stderr; the stdout-only runner yields "".
			name: "logged out with a project configured",
			fake: fakeGcloudReads{accountOut: "", projectOut: "my-project"},
		},
		{
			name:              "account without project",
			fake:              fakeGcloudReads{accountOut: "jane@example.com\n", projectOut: ""},
			wantAuthenticated: true,
			wantAccount:       "jane@example.com",
			wantProjectRead:   true,
		},
		{
			name:              "account with unset project",
			fake:              fakeGcloudReads{accountOut: "jane@example.com\n", projectOut: "(unset)"},
			wantAuthenticated: true,
			wantAccount:       "jane@example.com",
			wantProjectRead:   true,
		},
		{
			name:              "account and project",
			fake:              fakeGcloudReads{accountOut: "jane@example.com\n", projectOut: "Your active configuration is: [cloudshell-1]\nmy-project\n"},
			wantAuthenticated: true,
			wantAvailable:     true,
			wantAccount:       "jane@example.com",
			wantProject:       "my-project",
			wantProjectRead:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fake := tt.fake
			probed := false
			probe := func() ([]GCPService, bool) {
				probed = true
				return nil, false
			}

			// Act
			info := detectGCPWith(fake.run, probe)

			// Assert
			if info.Authenticated != tt.wantAuthenticated || info.Available != tt.wantAvailable {
				t.Errorf("authenticated=%v available=%v, want %v/%v", info.Authenticated, info.Available, tt.wantAuthenticated, tt.wantAvailable)
			}
			if info.Account != tt.wantAccount || info.ProjectID != tt.wantProject {
				t.Errorf("account=%q project=%q, want %q/%q", info.Account, info.ProjectID, tt.wantAccount, tt.wantProject)
			}
			projectRead := len(fake.reads) > 1
			if projectRead != tt.wantProjectRead {
				t.Errorf("project read = %v, want %v (reads: %v)", projectRead, tt.wantProjectRead, fake.reads)
			}
			if probed != tt.wantAvailable {
				t.Errorf("services probed = %v, want only when available", probed)
			}
		})
	}
}
