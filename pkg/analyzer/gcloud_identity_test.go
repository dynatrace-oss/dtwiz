package analyzer

import (
	"errors"
	"strings"
	"testing"
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

			if got := cleanGCloudConfigValue(tt.out); got != tt.want {
				t.Fatalf("cleanGCloudConfigValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

// fakeGCloudReader answers the two identity reads and records which ones were made.
func fakeGCloudReader(accountOut string, accountErr error, projectOut string, projectErr error, reads *[]string) GCloudReader {
	return func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch joined {
		case "auth list --filter=status:ACTIVE --format=value(account)":
			*reads = append(*reads, "account")
			return accountOut, accountErr
		case "config get-value project":
			*reads = append(*reads, "project")
			return projectOut, projectErr
		}
		return "", errors.New("unexpected gcloud call: " + joined)
	}
}

func TestReadGCloudIdentity(t *testing.T) {
	readFailure := errors.New("exit status 1")
	tests := []struct {
		name        string
		accountOut  string
		accountErr  error
		projectOut  string
		projectErr  error
		wantAccount string
		wantProject string
		wantErr     bool
		wantReads   []string
	}{
		{
			// Logged out: stdout is empty, the filter warning only goes to stderr.
			name:       "logged out with a project configured",
			accountOut: "",
			projectOut: "my-project",
			wantReads:  []string{"account"},
		},
		{
			name:       "account read fails",
			accountErr: readFailure,
			wantErr:    true,
			wantReads:  []string{"account"},
		},
		{
			name:        "account without project",
			accountOut:  "jane@example.com\n",
			projectOut:  "",
			wantAccount: "jane@example.com",
			wantReads:   []string{"account", "project"},
		},
		{
			name:        "account with unset project behind a Cloud Shell notice",
			accountOut:  "jane@example.com\n",
			projectOut:  "Your active configuration is: [cloudshell-1]\n(unset)\n",
			wantAccount: "jane@example.com",
			wantReads:   []string{"account", "project"},
		},
		{
			name:        "project read fails keeps the account",
			accountOut:  "jane@example.com\n",
			projectErr:  readFailure,
			wantAccount: "jane@example.com",
			wantErr:     true,
			wantReads:   []string{"account", "project"},
		},
		{
			name:        "account and project",
			accountOut:  "jane@example.com\n",
			projectOut:  "Your active configuration is: [cloudshell-1]\nmy-project\n",
			wantAccount: "jane@example.com",
			wantProject: "my-project",
			wantReads:   []string{"account", "project"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var reads []string
			read := fakeGCloudReader(tt.accountOut, tt.accountErr, tt.projectOut, tt.projectErr, &reads)

			// Act
			account, project, err := ReadGCloudIdentity(read)

			// Assert
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, readFailure) {
				t.Errorf("err = %v, want it to wrap the read failure", err)
			}
			if account != tt.wantAccount || project != tt.wantProject {
				t.Errorf("account=%q project=%q, want %q/%q", account, project, tt.wantAccount, tt.wantProject)
			}
			if strings.Join(reads, ",") != strings.Join(tt.wantReads, ",") {
				t.Errorf("reads = %v, want %v", reads, tt.wantReads)
			}
		})
	}
}
