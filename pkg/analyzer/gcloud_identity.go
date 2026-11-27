package analyzer

import (
	"fmt"
	"strings"
)

// GCloudReader runs `gcloud <args...>` and returns its stdout only. gcloud writes
// notices and warnings to stderr (e.g. "WARNING: The following filter keys were not
// present ..."), which must never be parsed as an account or project value.
type GCloudReader func(args ...string) (string, error)

// ReadGCloudIdentity resolves the active gcloud account, then the active project. It is
// the single source of truth for "is gcloud ready" in both system detection and the GCP
// installer preflight. Exported so pkg/installer/gcp can use it without pkg/analyzer
// importing pkg/installer, which would create an import cycle.
//
// The account is the one `gcloud auth list` reports as ACTIVE, which requires stored
// credentials. The `core/account` setting (`gcloud config get-value account`) is not
// used: it can name an account that was never logged in or whose credentials were
// removed. Expired credentials are not detected; that needs a network call.
//
// An empty account or project means "not set". The project is not read without an
// account. A failing read returns the values resolved so far together with the error.
func ReadGCloudIdentity(read GCloudReader) (account, project string, err error) {
	out, err := read("auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
	if err != nil {
		return "", "", fmt.Errorf("reading active gcloud account: %w", err)
	}
	account = firstLine(out)
	if account == "" {
		return "", "", nil
	}

	out, err = read("config", "get-value", "project")
	if err != nil {
		return account, "", fmt.Errorf("reading active gcloud project: %w", err)
	}
	project = cleanGCloudConfigValue(out)
	if strings.Contains(project, "(unset)") {
		project = ""
	}
	return account, project, nil
}

// firstLine returns the first non-empty trimmed line of out.
func firstLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// cleanGCloudConfigValue extracts the value from `gcloud config get-value ...` output.
// In some environments (e.g. Cloud Shell with multiple named configurations) gcloud
// prefixes the value with a "Your active configuration is: [...]" notice line; this
// returns the last non-empty, non-notice line instead of the raw multi-line output.
func cleanGCloudConfigValue(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "Your active configuration is:") {
			continue
		}
		return line
	}
	return ""
}
